package cli

import (
	"context"
	"encoding/json"
	"fmt"
	"io"
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
	cmd := &cobra.Command{
		Use:   "whois <target>",
		Short: "查询域名/IP 的 WHOIS 信息（RFC 3912）",
		Long: `WHOIS 查询：自动检测 domain vs IP，自动路由到正确的服务器，
跟随 IANA root / ARIN 的 referral 到最终响应。

例：
  jdan whois example.com           # 域名 → Verisign
  jdan whois 8.8.8.8               # IPv4 → ARIN → 自动跟到 RIPE/APNIC（如有 referral）
  jdan whois 2001:db8::1           # IPv6
  jdan whois example.com --json    # 结构化输出
  jdan whois ex.com --server custom.whois.com  # 覆盖默认 server`,
		Args:          cobra.ExactArgs(1),
		SilenceUsage:  true,
		SilenceErrors: true,
		RunE: func(cmd *cobra.Command, args []string) error {
			return runWhois(cmd, args, deps)
		},
	}
	cmd.Flags().Bool("json", false, "结构化 JSON 输出")
	cmd.Flags().String("server", "", "覆盖默认 server（绕过 TLD 映射 + IANA fallback）")
	cmd.Flags().Duration("timeout", whois.DefaultTimeout, "单次查询超时")
	return cmd
}

func runWhois(cmd *cobra.Command, args []string, deps whoisCmdDeps) error {
	target := strings.TrimSpace(args[0])
	asJSON, _ := cmd.Flags().GetBool("json")
	server, _ := cmd.Flags().GetString("server")
	timeout, _ := cmd.Flags().GetDuration("timeout")

	// 外层 context 给 1.5x 余量：referral 跟随会发起多次 Query
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

	if asJSON {
		enc := json.NewEncoder(deps.out)
		enc.SetIndent("", "  ")
		return enc.Encode(res)
	}
	renderWhoisRaw(deps.out, res)
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
	body := r.RawText
	// 防御：保证 raw 末尾换行规整
	if !strings.HasSuffix(body, "\n") {
		body += "\n"
	}
	fmt.Fprint(w, body)
}

func init() {
	rootCmd.AddCommand(newWhoisCommand(whoisCmdDeps{}))
}
