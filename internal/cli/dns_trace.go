package cli

import (
	"context"
	"fmt"
	"strings"
	"time"

	"github.com/miekg/dns"
	"github.com/spf13/cobra"

	"github.com/xunull/jdan/internal/dnslookup"
	"github.com/xunull/jdan/internal/dnstrace"
)

// newTraceCommand 构造 `jdan dns trace <domain>` 子命令。
//
// trace 输出形态与 dns lookup / reverse 不同（hop 分段叙事，非表格），所以
// 不复用 runDNSQuery helper。共享的 flag 解析模式接受一定重复——这是
// "三处重复才抽象"的合理停顿。
func newTraceCommand(deps dnsCmdDeps) *cobra.Command {
	cmd := &cobra.Command{
		Use:   "trace <domain>",
		Short: "从根 DNS 服务器开始迭代解析，展示每一跳委派路径",
		Long: `从根 DNS 服务器开始迭代解析，展示每一跳委派路径（dig +trace 的 jdan 同款）。

trace 主路径直接 UDP/TCP 到权威 NS（不走 recursive resolver）：
  1. 从 13 台根 DNS server 起步
  2. 解析 referral，沿 NS 委派一路追到权威 zone
  3. 在权威 NS 上查询目标 record type

--doh 在 trace 中**仅作 glueless NS bootstrap** 用途——当 referral 给的 NS 没有
in-bailiwick glue 时用 DoH 解析 NS hostname。trace 主跳路径仍直接查权威 NS。
未传 --doh 时 glueless 走系统 resolver（可能被本地劫持，但 90%+ 主流域名
in-bailiwick glue 现成不触发此路径）。

--server 与 dns lookup 中不同：在 trace 中是"起步 server IP"，覆盖 13 个根
（极少用，便于自建权威服务器测试）。--server 与 --doh 可共存。

退出码：默认拿到 final answer 即 0；trace 中断（cycle / max hops / NS 全失败）
exit 1。--strict 切换为"任一 hop 报错即 exit 1"。

NOT 支持：DNSSEC 验证、CNAME 链自动追踪、parallel NS、path latency 总结，
这些留给未来 jdan dns sec 或独立 PR。`,
		Args: cobra.ExactArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			return runDNSTrace(cmd, args, deps)
		},
	}

	cmd.Flags().StringP("type", "t", "A", "查询的 record type（默认 A）")
	cmd.Flags().String("doh", "", "DoH endpoint：别名 ("+strings.Join(dnslookup.ProviderAliases(), "/")+")、主机名或完整 https:// URL。仅用于 glueless NS bootstrap，trace 主路径仍直接查权威 NS")
	cmd.Flags().StringP("server", "s", "", "起步 root server IP（空时用 13 个根，按 a-m 顺序 fallback）")
	cmd.Flags().BoolP("json", "j", false, "以 JSON 格式输出")
	cmd.Flags().Bool("short", false, "仅输出最终答案（dig +short 风格）")
	cmd.Flags().BoolP("verbose", "v", false, "每跳含 NS referrals 与 glue 详情")
	cmd.Flags().Bool("strict", false, "任一 hop 报错即 exit 1（默认拿到 final answer 即 0）")
	cmd.Flags().Duration("timeout", 30*time.Second, "整 trace 总超时")
	cmd.Flags().Duration("hop-timeout", 3*time.Second, "单跳查询超时")

	return cmd
}

func runDNSTrace(cmd *cobra.Command, args []string, deps dnsCmdDeps) error {
	domain := args[0]
	typeStr, _ := cmd.Flags().GetString("type")
	doh, _ := cmd.Flags().GetString("doh")
	server, _ := cmd.Flags().GetString("server")
	jsonOut, _ := cmd.Flags().GetBool("json")
	short, _ := cmd.Flags().GetBool("short")
	verbose, _ := cmd.Flags().GetBool("verbose")
	strict, _ := cmd.Flags().GetBool("strict")
	timeout, _ := cmd.Flags().GetDuration("timeout")
	hopTimeout, _ := cmd.Flags().GetDuration("hop-timeout")

	if jsonOut && short {
		return fmt.Errorf("--json 与 --short 不能同时使用")
	}
	if jsonOut && verbose {
		return fmt.Errorf("--json 与 --verbose 不能同时使用")
	}
	if short && verbose {
		return fmt.Errorf("--short 与 --verbose 不能同时使用")
	}
	if timeout <= 0 {
		return fmt.Errorf("--timeout 必须大于 0")
	}
	if hopTimeout <= 0 {
		return fmt.Errorf("--hop-timeout 必须大于 0")
	}
	if hopTimeout > timeout {
		return fmt.Errorf("--hop-timeout (%s) 不应大于 --timeout (%s)", hopTimeout, timeout)
	}

	qtype, err := parseTraceType(typeStr)
	if err != nil {
		return err
	}

	// 构造 bootstrap resolver：--doh 传了走 DoH，否则用 OS resolver。
	// trace 主路径不走 bootstrap——bootstrap 仅在 glueless NS 时被调。
	bootstrap, err := buildTraceBootstrap(doh, hopTimeout)
	if err != nil {
		return err
	}

	doTrace := deps.trace
	if doTrace == nil {
		tracer := dnstrace.NewTracer(dnstrace.Options{
			Bootstrap:    bootstrap,
			HopTimeout:   hopTimeout,
			TotalTimeout: timeout,
			StartServer:  server,
		})
		doTrace = func(ctx context.Context, domain string, qtype uint16) (*dnstrace.Result, error) {
			return tracer.Trace(ctx, domain, qtype)
		}
	}

	res, err := doTrace(context.Background(), domain, qtype)
	if err != nil {
		return err
	}

	switch {
	case jsonOut:
		out, jerr := dnstrace.FormatJSON(res)
		if jerr != nil {
			return jerr
		}
		fmt.Fprintln(deps.out, out)
	case short:
		fmt.Fprint(deps.out, dnstrace.FormatShort(res))
	case verbose:
		fmt.Fprint(deps.out, dnstrace.FormatVerbose(res))
	default:
		fmt.Fprint(deps.out, dnstrace.FormatText(res))
	}

	// exit code：默认宽容（拿到 final answer 即 0）；--strict 严格（任一 hop error 即 1）
	failed := !res.Succeeded()
	if strict {
		failed = res.HasAnyError()
	}
	if failed {
		deps.exit(1)
	}
	return nil
}

// parseTraceType 把 --type 字符串解析为 dns type code。默认 A。
func parseTraceType(s string) (uint16, error) {
	s = strings.ToUpper(strings.TrimSpace(s))
	if s == "" {
		s = "A"
	}
	qt, ok := dns.StringToType[s]
	if !ok {
		return 0, fmt.Errorf("不支持的 record type: %q", s)
	}
	return qt, nil
}

// buildTraceBootstrap 根据 --doh 选择 bootstrap Resolver 实现。
// --doh 非空 → dohResolver（绕过本地 DNS 劫持）
// --doh 空 → osLookupResolver（包装 net.DefaultResolver）
func buildTraceBootstrap(doh string, timeout time.Duration) (dnslookup.Resolver, error) {
	if doh == "" {
		return dnstrace.NewOSLookupResolver(timeout), nil
	}
	target, err := dnslookup.ResolveDoHTarget(doh)
	if err != nil {
		return nil, err
	}
	return dnslookup.NewDoHResolver(target, timeout), nil
}
