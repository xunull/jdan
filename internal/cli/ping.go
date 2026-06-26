package cli

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net"
	"os"
	"runtime"
	"strings"
	"time"

	"github.com/spf13/cobra"

	"github.com/xunull/jdan/internal/dnslookup"
	"github.com/xunull/jdan/internal/pingx"
)

type pingCmdDeps struct {
	out      io.Writer
	errOut   io.Writer
	runner   pingx.Runner
	resolver dnslookup.Resolver // 注入用；nil 时按 --dns 构造
	goos     string
}

type pingJSON struct {
	Host        string   `json:"host"`
	ResolvedIP  string   `json:"resolved_ip,omitempty"`
	DNSServer   string   `json:"dns_server,omitempty"`
	IPVersion   int      `json:"ip_version"`
	Transmitted *int     `json:"transmitted,omitempty"`
	Received    *int     `json:"received,omitempty"`
	LossPct     *float64 `json:"loss_pct,omitempty"`
	RTTMin      *float64 `json:"rtt_min_ms,omitempty"`
	RTTAvg      *float64 `json:"rtt_avg_ms,omitempty"`
	RTTMax      *float64 `json:"rtt_max_ms,omitempty"`
	ExitCode    int      `json:"exit_code"`
}

func newPingCommand(deps pingCmdDeps) *cobra.Command {
	if deps.out == nil {
		deps.out = os.Stdout
	}
	if deps.errOut == nil {
		deps.errOut = os.Stderr
	}
	if deps.runner == nil {
		deps.runner = pingx.ExecRunner
	}
	if deps.goos == "" {
		deps.goos = runtime.GOOS
	}

	cmd := &cobra.Command{
		Use:   "ping [flags] <host> [-- ping-args...]",
		Short: "ping，可用 --dns 指定解析域名用的 DNS server",
		Long: `ping 一个主机。给 --dns 时先用指定 DNS 把域名解析成 IP 再 ping 这个 IP
（确保用你指定的 DNS 解析，不被系统 resolver / DNS 劫持影响）；不给则退化成
系统 ping 默认行为。底层调用系统 ping（仅 macOS + Linux）。

例：
  jdan ping example.com                       # 普通 ping（系统解析）
  jdan ping --dns 8.8.8.8 example.com         # 用 8.8.8.8 解析再 ping IP
  jdan ping --doh google example.com          # 用 DoH（别名，自带 bootstrap IP）
  jdan ping --doh https://dns.google/dns-query example.com   # DoH 完整 URL
  jdan ping --dns 8.8.8.8 -c 3 example.com    # 发 3 个包
  jdan ping --dns 8.8.8.8 example.com -- -i 0.2 -s 64   # -- 后透传给系统 ping
  jdan ping --doh cloudflare -c 3 example.com --json`,
		Args:          cobra.ArbitraryArgs,
		SilenceUsage:  true,
		SilenceErrors: true,
		RunE: func(cmd *cobra.Command, args []string) error {
			return runPing(cmd, deps, args)
		},
	}
	cmd.Flags().String("dns", "", "解析域名用的 DNS server（8.8.8.8 / 8.8.8.8:5353 / DoH URL）")
	cmd.Flags().String("doh", "", "用 DoH 解析：别名("+strings.Join(dnslookup.ProviderAliases(), "/")+")、主机名或完整 https:// URL；与 --dns 互斥")
	cmd.Flags().IntP("count", "c", 0, "发送的包数（-c，Linux/macOS 通用）")
	cmd.Flags().BoolP("ipv4", "4", false, "解析 A / ping IPv4（默认）")
	cmd.Flags().BoolP("ipv6", "6", false, "解析 AAAA / ping IPv6")
	cmd.Flags().Bool("json", false, "JSON 输出（解析事实 + 尽力解析的汇总）")
	cmd.Flags().Duration("dns-timeout", 5*time.Second, "DNS 解析超时")
	return cmd
}

func runPing(cmd *cobra.Command, deps pingCmdDeps, args []string) error {
	dnsServer, _ := cmd.Flags().GetString("dns")
	doh, _ := cmd.Flags().GetString("doh")
	count, _ := cmd.Flags().GetInt("count")
	v4, _ := cmd.Flags().GetBool("ipv4")
	v6, _ := cmd.Flags().GetBool("ipv6")
	asJSON, _ := cmd.Flags().GetBool("json")
	dnsTimeout, _ := cmd.Flags().GetDuration("dns-timeout")

	if v4 && v6 {
		return fmt.Errorf("-4 和 -6 不能同时用")
	}
	if dnsServer != "" && doh != "" {
		return fmt.Errorf("--dns 与 --doh 不能同时使用")
	}

	// 分离 host 与 -- 之后的透传参数
	host, extra, err := splitPingArgs(cmd, args)
	if err != nil {
		return err
	}

	ctx := context.Background()
	hostIsIP := net.ParseIP(host) != nil

	target := host
	resolvedIP := ""
	displayServer := ""
	if (dnsServer != "" || doh != "") && !hostIsIP {
		resolver, queryServer, disp, err := buildPingResolver(deps, dnsServer, doh, dnsTimeout)
		if err != nil {
			return err
		}
		ip, err := pingx.Resolve(ctx, resolver, host, queryServer, v6)
		if err != nil {
			return err
		}
		resolvedIP = ip
		target = ip
		displayServer = disp
		if !asJSON {
			fmt.Fprintf(deps.out, "%s → %s (via %s)\n", host, ip, disp)
		}
	}

	// target 是 IPv6 字面量则强制走 v6
	if ip := net.ParseIP(target); ip != nil && ip.To4() == nil {
		v6 = true
	}
	ipVersion := 4
	if v6 {
		ipVersion = 6
	}

	// JSON 模式必须有限次数才能拿到汇总（否则 ping 不退出），默认补 4
	if asJSON && count <= 0 {
		count = 4
	}

	bin, pargs := pingx.BuildCommand(target, pingx.Options{Count: count, V6: v6, Extra: extra}, deps.goos)

	if asJSON {
		var buf bytes.Buffer
		code, err := deps.runner(ctx, bin, pargs, &buf, &buf)
		if err != nil {
			return err
		}
		rec := buildPingJSON(host, resolvedIP, displayServer, ipVersion, code, pingx.ParseSummary(buf.String()))
		data, err := json.MarshalIndent(rec, "", "  ")
		if err != nil {
			return err
		}
		fmt.Fprintln(deps.out, string(data))
		if code != 0 {
			return fmt.Errorf("ping 退出码 %d", code)
		}
		return nil
	}

	code, err := deps.runner(ctx, bin, pargs, deps.out, deps.errOut)
	if err != nil {
		return err
	}
	if code != 0 {
		return fmt.Errorf("ping 退出码 %d", code)
	}
	return nil
}

// splitPingArgs 用 cobra 的 ArgsLenAtDash 把 host 与 -- 后透传参数分开。
func splitPingArgs(cmd *cobra.Command, args []string) (host string, extra []string, err error) {
	dash := cmd.ArgsLenAtDash()
	pre := args
	if dash >= 0 {
		pre = args[:dash]
		extra = args[dash:]
	}
	if len(pre) != 1 {
		return "", nil, fmt.Errorf("需要且只需要一个 host 参数（透传给 ping 的参数放在 -- 之后）")
	}
	return pre[0], extra, nil
}

func buildPingJSON(host, resolvedIP, dnsServer string, ipVersion, code int, sum pingx.Summary) pingJSON {
	// dnsServer / resolvedIP 仅在发生了解析时非空，omitempty 自动处理 host 是 IP 的情况
	rec := pingJSON{Host: host, ResolvedIP: resolvedIP, DNSServer: dnsServer, IPVersion: ipVersion, ExitCode: code}
	if sum.HasStats {
		rec.Transmitted = &sum.Transmitted
		rec.Received = &sum.Received
		rec.LossPct = &sum.LossPct
	}
	if sum.HasRTT {
		rec.RTTMin = &sum.RTTMin
		rec.RTTAvg = &sum.RTTAvg
		rec.RTTMax = &sum.RTTMax
	}
	return rec
}

// buildPingResolver 根据 --dns / --doh 构造解析器。
// 返回 (resolver, queryServer, displayServer, err)：
//   - queryServer 传给 Resolver.Query（DoH 时为空，DoH resolver 自带 target）
//   - displayServer 用于 "host → ip (via X)" 头和 --json 的 dns_server 字段
func buildPingResolver(deps pingCmdDeps, dnsServer, doh string, timeout time.Duration) (dnslookup.Resolver, string, string, error) {
	disp := dnsServer
	if doh != "" {
		disp = doh
	}
	if deps.resolver != nil {
		return deps.resolver, dnsServer, disp, nil
	}
	// --doh：别名（自带 bootstrap IP）/ 主机名 / 完整 https URL
	if doh != "" {
		tgt, err := dnslookup.ResolveDoHTarget(doh)
		if err != nil {
			return nil, "", "", err
		}
		return dnslookup.NewDoHResolver(tgt, timeout), "", tgt.URL, nil
	}
	// --dns 是完整 DoH URL（无 bootstrap）
	if dnslookup.IsDoHURL(dnsServer) {
		tgt, err := dnslookup.ResolveDoHTarget(dnsServer)
		if err != nil {
			return nil, "", "", err
		}
		return dnslookup.NewDoHResolver(tgt, timeout), "", dnsServer, nil
	}
	// 普通 UDP/TCP DNS
	return dnslookup.NewResolver(timeout), dnsServer, dnsServer, nil
}

func init() {
	rootCmd.AddCommand(newPingCommand(pingCmdDeps{}))
}
