package cli

import (
	"context"
	"encoding/json"
	"fmt"
	"io"
	"math"
	"os"
	"strings"
	"time"

	"github.com/spf13/cobra"

	"github.com/xunull/jdan/internal/whois"
)

type whoisCmdDeps struct {
	out io.Writer
	// 测试 hook：替换实际的 WHOIS 查询函数（mock server 用）
	lookup           func(ctx context.Context, target string, timeout time.Duration) (*whois.Result, error)
	lookupWithServer func(ctx context.Context, target, server string, timeout time.Duration) (*whois.Result, error)
	// now 用于"距今多久"渲染的可测试性
	now func() time.Time
}

func newWhoisCommand(deps whoisCmdDeps) *cobra.Command {
	if deps.out == nil {
		deps.out = os.Stdout
	}
	if deps.lookup == nil {
		deps.lookup = whois.Lookup
	}
	if deps.lookupWithServer == nil {
		deps.lookupWithServer = whois.LookupWithServer
	}
	if deps.now == nil {
		deps.now = time.Now
	}
	cmd := &cobra.Command{
		Use:   "whois <target>",
		Short: "查询域名/IP 的 WHOIS 信息（RFC 3912）",
		Long: `WHOIS 查询：自动检测 domain vs IP，自动路由到正确的服务器，
跟随 IANA root / ARIN 的 referral 到最终响应。默认输出解析后的字段表；
parser 失败（schema 不识别）时自动回退到原文，--raw 永远拿原文。

例：
  jdan whois example.com           # parsed 表（默认）
  jdan whois example.com --raw     # 原始 WHOIS 文本
  jdan whois example.com --full    # parsed 表 + 后附原文
  jdan whois example.com --json    # 结构化 JSON（含 parsed + raw）
  jdan whois 8.8.8.8               # IPv4 → ARIN → 自动跟到 RIPE/APNIC（如有 referral）
  jdan whois ex.com --server custom.whois.com  # 覆盖默认 server`,
		Args:          cobra.ExactArgs(1),
		SilenceUsage:  true,
		SilenceErrors: true,
		RunE: func(cmd *cobra.Command, args []string) error {
			return runWhois(cmd, args, deps)
		},
	}
	cmd.Flags().Bool("raw", false, "只输出原始 WHOIS 响应（不解析）")
	cmd.Flags().Bool("full", false, "parsed 表 + 后附原文（debug 用）")
	cmd.Flags().Bool("json", false, "结构化 JSON 输出（含 parsed 字段 + 完整原文）")
	cmd.Flags().String("server", "", "覆盖默认 server（绕过 TLD 映射 + IANA fallback）")
	cmd.Flags().Duration("timeout", whois.DefaultTimeout, "单次查询超时")
	cmd.MarkFlagsMutuallyExclusive("raw", "full", "json")
	return cmd
}

func runWhois(cmd *cobra.Command, args []string, deps whoisCmdDeps) error {
	target := strings.TrimSpace(args[0])
	asJSON, _ := cmd.Flags().GetBool("json")
	raw, _ := cmd.Flags().GetBool("raw")
	full, _ := cmd.Flags().GetBool("full")
	server, _ := cmd.Flags().GetString("server")
	timeout, _ := cmd.Flags().GetDuration("timeout")

	// 外层 context 给 3x 余量：referral 跟随会发起多次 Query
	ctx, cancel := context.WithTimeout(context.Background(), time.Duration(float64(timeout)*float64(maxLookupHopsForCtx)))
	defer cancel()

	var (
		res *whois.Result
		err error
	)
	if server != "" {
		res, err = deps.lookupWithServer(ctx, target, server, timeout)
	} else {
		res, err = deps.lookup(ctx, target, timeout)
	}
	if err != nil {
		return err
	}

	switch {
	case asJSON:
		enc := json.NewEncoder(deps.out)
		enc.SetIndent("", "  ")
		return enc.Encode(res)
	case raw:
		renderWhoisRaw(deps.out, res)
	case full:
		renderWhoisParsed(deps.out, res, deps.now)
		fmt.Fprintln(deps.out, "\n--- Raw WHOIS response ---")
		fmt.Fprint(deps.out, ensureTrailingNL(res.RawText))
	default:
		// parser 一无所获时自动回退 raw（保证用户至少能看到点东西）
		if res.Parsed == nil || res.Parsed.IsEmpty() {
			renderWhoisRaw(deps.out, res)
		} else {
			renderWhoisParsed(deps.out, res, deps.now)
		}
	}
	return nil
}

// maxLookupHopsForCtx 是 outer context timeout 的倍数（覆盖最大跳数）。
const maxLookupHopsForCtx = 3

func renderWhoisRaw(w io.Writer, r *whois.Result) {
	fmt.Fprintf(w, "%% Target: %s (%s)\n", r.Target, r.Kind)
	fmt.Fprintf(w, "%% Server: %s\n", r.Server)
	if len(r.Hops) > 0 {
		fmt.Fprint(w, "% Chain:  ")
		for _, h := range r.Hops {
			fmt.Fprintf(w, "%s -> ", h.Server)
		}
		fmt.Fprintln(w, r.Server)
	}
	fmt.Fprintln(w)
	fmt.Fprint(w, ensureTrailingNL(r.RawText))
}

func renderWhoisParsed(w io.Writer, r *whois.Result, nowFn func() time.Time) {
	fmt.Fprintf(w, "Target:    %s (%s)\n", r.Target, r.Kind)
	fmt.Fprintf(w, "Server:    %s\n", r.Server)
	if len(r.Hops) > 0 {
		fmt.Fprint(w, "Chain:     ")
		for _, h := range r.Hops {
			fmt.Fprintf(w, "%s -> ", h.Server)
		}
		fmt.Fprintln(w, r.Server)
	}
	fmt.Fprintln(w)
	if r.Parsed == nil {
		fmt.Fprintln(w, "(no parsed fields available)")
		return
	}
	switch r.Kind {
	case whois.KindDomain:
		renderDomainTable(w, r.Parsed, nowFn())
	case whois.KindIPv4, whois.KindIPv6:
		renderIPTable(w, r.Parsed, nowFn())
	}
}

func renderDomainTable(w io.Writer, p *whois.Parsed, now time.Time) {
	row := func(label, val string) {
		if val != "" {
			fmt.Fprintf(w, "  %-15s %s\n", label+":", val)
		}
	}
	dateRow := func(label string, t time.Time) {
		if !t.IsZero() {
			fmt.Fprintf(w, "  %-15s %s  (%s)\n", label+":", t.Format("2006-01-02 15:04 MST"), humanizeAgo(t, now))
		}
	}
	row("Domain", p.DomainName)
	row("Registrar", p.Registrar)
	dateRow("Created", p.CreationDate)
	dateRow("Updated", p.UpdatedDate)
	dateRow("Expires", p.ExpiryDate)
	row("Registry ID", p.RegistryDomainID)
	row("DNSSEC", p.DNSSEC)
	row("Country", p.RegistrantCountry)
	multilineRow(w, "Status", p.Status)
	multilineRow(w, "Nameservers", p.Nameservers)
}

func renderIPTable(w io.Writer, p *whois.Parsed, now time.Time) {
	row := func(label, val string) {
		if val != "" {
			fmt.Fprintf(w, "  %-15s %s\n", label+":", val)
		}
	}
	row("Range", p.NetRange)
	row("Net name", p.NetName)
	row("Org", p.OrgName)
	row("Country", p.Country)
	row("Abuse email", p.AbuseEmail)
	if !p.RegDate.IsZero() {
		fmt.Fprintf(w, "  %-15s %s  (%s)\n", "Registered:", p.RegDate.Format("2006-01-02"), humanizeAgo(p.RegDate, now))
	}
}

func multilineRow(w io.Writer, label string, items []string) {
	if len(items) == 0 {
		return
	}
	fmt.Fprintf(w, "  %-15s %s\n", label+":", items[0])
	for _, it := range items[1:] {
		fmt.Fprintf(w, "  %-15s %s\n", "", it)
	}
}

// humanizeAgo 把时间渲染成 "5 days ago" / "in 2 months" / "just now" 等可读字符串。
func humanizeAgo(t, now time.Time) string {
	delta := now.Sub(t)
	future := delta < 0
	if future {
		delta = -delta
	}
	d := math.Abs(delta.Seconds())
	var unit string
	var n float64
	switch {
	case d < 60:
		unit, n = "second", d
	case d < 3600:
		unit, n = "minute", d/60
	case d < 86400:
		unit, n = "hour", d/3600
	case d < 86400*30:
		unit, n = "day", d/86400
	case d < 86400*365:
		unit, n = "month", d/(86400*30)
	default:
		unit, n = "year", d/(86400*365)
	}
	plural := ""
	if math.Round(n) != 1 {
		plural = "s"
	}
	if future {
		return fmt.Sprintf("in %.0f %s%s", n, unit, plural)
	}
	return fmt.Sprintf("%.0f %s%s ago", n, unit, plural)
}

func ensureTrailingNL(s string) string {
	if !strings.HasSuffix(s, "\n") {
		return s + "\n"
	}
	return s
}

func init() {
	rootCmd.AddCommand(newWhoisCommand(whoisCmdDeps{}))
}
