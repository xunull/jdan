package cli

import (
	"context"
	"fmt"
	"io"
	"os"
	"strings"
	"time"

	"github.com/spf13/cobra"

	"github.com/xunull/jdan/internal/dnslookup"
)

// dnsCmdDeps 把外部依赖暴露为依赖注入点，便于在测试里替换 lookup / detectServer / exit。
type dnsCmdDeps struct {
	out           io.Writer
	lookup        func(ctx context.Context, opts dnslookup.Options) (*dnslookup.Result, error)
	detectServer  func() string
	exit          func(code int)
}

func newDNSCommand(deps dnsCmdDeps) *cobra.Command {
	if deps.out == nil {
		deps.out = os.Stdout
	}
	if deps.detectServer == nil {
		deps.detectServer = dnslookup.DetectSystemServer
	}
	if deps.exit == nil {
		deps.exit = os.Exit
	}

	dnsCmd := &cobra.Command{
		Use:   "dns",
		Short: "DNS 相关子命令",
	}

	lookupCmd := &cobra.Command{
		Use:   "lookup <domain>",
		Short: "并发查询域名的多个 DNS 记录类型",
		Long: `并发查询域名的多个 DNS 记录类型。

默认查询 6 个最常用 type：A、AAAA、MX、TXT、CNAME、NS。可通过 --type 指定单个或
多个类型（逗号分隔），--type all 查询 9 个 type（默认 6 个 + SOA + CAA + SRV）。

默认从 /etc/resolv.conf 读取系统 DNS server，文件不存在或解析失败时 fallback 8.8.8.8。
顶部一行会打印 'domain — via X.X.X.X:53' 说明实际查询源。

加密查询：使用 --doh 通过 DNS-over-HTTPS (RFC 8484) 查询。接受三种输入：
  • 别名：google / cloudflare / quad9 / opendns / ali / 360
  • 主机名：dns.google（自动补 /dns-query path）
  • 完整 URL：https://dns.example.com/path
别名形式用内置的提供商 IP 直连，绕过本地 DNS 劫持。--doh 与 --server 互斥。

退出码：默认宽容（任一 type 有结果即 0；全部失败才 1）。--strict 切换严格模式：
任一 type 失败即 1。`,
		Args: cobra.ExactArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			return runDNSLookup(cmd, args, deps)
		},
	}

	lookupCmd.Flags().StringP("type", "t", "", "查询的 record type，逗号分隔；'all' 表示 9 个；空表示默认 6 个")
	lookupCmd.Flags().StringP("server", "s", "", "DNS server (e.g. 8.8.8.8 或 8.8.8.8:53)；空表示读 /etc/resolv.conf")
	lookupCmd.Flags().String("doh", "", "DoH endpoint：别名 ("+strings.Join(dnslookup.ProviderAliases(), "/")+")、主机名或完整 https:// URL；与 --server 互斥")
	lookupCmd.Flags().BoolP("json", "j", false, "以 JSON 格式输出")
	lookupCmd.Flags().Bool("short", false, "仅输出值，dig +short 风格")
	lookupCmd.Flags().BoolP("verbose", "v", false, "输出 query time 等元数据，rcode 单独列")
	lookupCmd.Flags().Bool("strict", false, "任一 type 失败即 exit 1")
	lookupCmd.Flags().Duration("timeout", 5*time.Second, "整体查询超时")

	dnsCmd.AddCommand(lookupCmd)
	return dnsCmd
}

// runDNSLookup 是 `jdan dns lookup` 的 RunE。它只负责解析 lookup 专属的
// `--type` flag，剩下的共享 flag + 查询 + 输出全部委托给 runDNSQuery。
func runDNSLookup(cmd *cobra.Command, args []string, deps dnsCmdDeps) error {
	typeStr, _ := cmd.Flags().GetString("type")
	types, err := dnslookup.ParseTypes(typeStr)
	if err != nil {
		return err
	}
	// lookup 不需要 DisplayName 覆盖——formatter 会回退到 Domain。
	return runDNSQuery(cmd, deps, args[0], types, "")
}

// runDNSQuery 是 dns 名空间下所有子命令的共享流水线：读取共享 flag、
// 检查互斥、解析 DoH/server、构造 resolver、执行 lookup、按 flag 选 formatter、
// 决定 exit code。caller 提供已解析好的 domain + types + 可选 displayName。
//
// 调用方约定：
//   - domain：实际发出的 DNS 查询域名（lookup 直接传入参；reverse 传 PTR 域名）
//   - types：要查询的 record type 列表（lookup 从 --type 解析；reverse 传 [TypePTR]）
//   - displayName：空串表示 formatter 顶部用 domain；非空表示覆盖（reverse 用原始 IP）
func runDNSQuery(cmd *cobra.Command, deps dnsCmdDeps, domain string, types []uint16, displayName string) error {
	server, _ := cmd.Flags().GetString("server")
	doh, _ := cmd.Flags().GetString("doh")
	jsonOut, _ := cmd.Flags().GetBool("json")
	short, _ := cmd.Flags().GetBool("short")
	verbose, _ := cmd.Flags().GetBool("verbose")
	strict, _ := cmd.Flags().GetBool("strict")
	timeout, _ := cmd.Flags().GetDuration("timeout")

	if jsonOut && short {
		return fmt.Errorf("--json 与 --short 不能同时使用")
	}
	if jsonOut && verbose {
		return fmt.Errorf("--json 与 --verbose 不能同时使用")
	}
	if short && verbose {
		return fmt.Errorf("--short 与 --verbose 不能同时使用")
	}
	if doh != "" && server != "" {
		return fmt.Errorf("--doh 与 --server 不能同时使用")
	}
	if timeout <= 0 {
		return fmt.Errorf("--timeout 必须大于 0")
	}

	var dohTarget dnslookup.DoHTarget
	useDoH := false
	if doh != "" {
		var err error
		dohTarget, err = dnslookup.ResolveDoHTarget(doh)
		if err != nil {
			return err
		}
		server = dohTarget.URL // 顶部 "via <server>" 显示完整 URL
		useDoH = true
	} else if server == "" {
		server = deps.detectServer()
	}

	doLookup := deps.lookup
	if doLookup == nil {
		var resolver dnslookup.Resolver
		if useDoH {
			resolver = dnslookup.NewDoHResolver(dohTarget, timeout)
		} else {
			resolver = dnslookup.NewResolver(timeout)
		}
		doLookup = func(ctx context.Context, opts dnslookup.Options) (*dnslookup.Result, error) {
			return dnslookup.Lookup(ctx, resolver, opts)
		}
	}

	opts := dnslookup.Options{
		Domain:      domain,
		DisplayName: displayName,
		Types:       types,
		Server:      server,
		Timeout:     timeout,
	}

	res, err := doLookup(context.Background(), opts)
	if err != nil {
		return err
	}

	switch {
	case jsonOut:
		out, jerr := dnslookup.FormatJSON(res)
		if jerr != nil {
			return jerr
		}
		fmt.Fprintln(deps.out, out)
	case short:
		fmt.Fprint(deps.out, dnslookup.FormatShort(res))
	case verbose:
		fmt.Fprint(deps.out, dnslookup.FormatVerbose(res))
	default:
		fmt.Fprint(deps.out, dnslookup.FormatText(res))
	}

	failed := res.AllFailed()
	if strict {
		failed = res.AnyFailed()
	}
	if failed {
		deps.exit(1)
	}
	return nil
}

func init() {
	rootCmd.AddCommand(newDNSCommand(dnsCmdDeps{}))
}
