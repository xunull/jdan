package cli

import (
	"fmt"
	"io"
	"net/http"
	"os"
	"strings"

	"github.com/spf13/cobra"

	"github.com/xunull/jdan/internal/cspx"
)

type cspCmdDeps struct {
	out    io.Writer
	in     io.Reader
	client *http.Client // 注入便于测试
}

func newCSPCommand(deps cspCmdDeps) *cobra.Command {
	if deps.out == nil {
		deps.out = os.Stdout
	}
	if deps.in == nil {
		deps.in = os.Stdin
	}
	cmd := &cobra.Command{
		Use:   "csp [url|policy]",
		Short: "解析 Content-Security-Policy 并做安全体检",
		Long: `解析 CSP 头成可读表格，并揪出常见弱点（unsafe-inline / unsafe-eval / 通配 * /
缺 default-src / 缺 object-src 'none' 等）。0 依赖。

输入三选一：
  jdan csp https://example.com              # 抓 URL，取 Content-Security-Policy 头
  jdan csp "default-src 'self'; script-src 'self' 'unsafe-inline'"   # 直接给头值
  echo "default-src 'self'" | jdan csp      # stdin

含空格或分号 → 当头值解析；否则当 URL 抓。`,
		Args:          cobra.MaximumNArgs(1),
		SilenceUsage:  true,
		SilenceErrors: true,
		RunE: func(cmd *cobra.Command, args []string) error {
			asJSON, _ := cmd.Flags().GetBool("json")

			value, srcURL, err := resolveCSPInput(deps, args)
			if err != nil {
				return err
			}
			value = strings.TrimSpace(value)
			if value == "" {
				return fmt.Errorf("没有 CSP 内容可解析")
			}

			policy := cspx.Parse(value)
			issues := cspx.Audit(policy)

			if asJSON {
				return writeIndentJSON(deps.out, map[string]any{
					"url":        srcURL,
					"directives": policy.Directives,
					"issues":     issues,
				})
			}
			emitCSPText(deps.out, srcURL, policy, issues)
			return nil
		},
	}
	cmd.Flags().Bool("json", false, "结构化输出")
	return cmd
}

// resolveCSPInput：含空格/分号 → 头值；否则当 URL 抓 Content-Security-Policy；无参 → stdin。
func resolveCSPInput(deps cspCmdDeps, args []string) (value, srcURL string, err error) {
	if len(args) == 0 {
		b, e := io.ReadAll(deps.in)
		return string(b), "", e
	}
	arg := args[0]
	if strings.ContainsAny(arg, " ;") {
		return arg, "", nil
	}
	// 当 URL 抓；优先 enforcing，缺了再试 Report-Only
	vals, final, e := fetchResponseHeader(deps.client, arg, "Content-Security-Policy")
	if e != nil {
		return "", "", e
	}
	if len(vals) == 0 {
		if ro, _, _ := fetchResponseHeaderQuiet(deps.client, final, "Content-Security-Policy-Report-Only"); len(ro) > 0 {
			return strings.Join(ro, ", ") + "  (Report-Only)", final, nil
		}
		return "", final, fmt.Errorf("该 URL 没有 Content-Security-Policy 头")
	}
	return strings.Join(vals, ", "), final, nil
}

// fetchResponseHeaderQuiet 同 fetchResponseHeader 但忽略错误（用于 fallback 探测）。
func fetchResponseHeaderQuiet(client *http.Client, raw, name string) ([]string, string, error) {
	return fetchResponseHeader(client, raw, name)
}

func emitCSPText(out io.Writer, srcURL string, p cspx.Policy, issues []cspx.Issue) {
	if srcURL != "" {
		fmt.Fprintf(out, "来源: %s\n\n", srcURL)
	}
	fmt.Fprintln(out, "CSP 指令:")
	if len(p.Directives) == 0 {
		fmt.Fprintln(out, "  （空）")
	}
	for _, d := range p.Directives {
		fmt.Fprintf(out, "  %-22s %s\n", d.Name, strings.Join(d.Sources, " "))
	}

	fmt.Fprintln(out, "\n体检:")
	if len(issues) == 0 {
		fmt.Fprintln(out, "  ✓ 未发现常见弱点")
		return
	}
	for _, is := range issues {
		mark := "⚠"
		if is.Level == "info" {
			mark = "·"
		}
		fmt.Fprintf(out, "  %s %s: %s\n", mark, is.Directive, is.Msg)
	}
}

func init() {
	rootCmd.AddCommand(newCSPCommand(cspCmdDeps{}))
}
