package cli

import (
	"fmt"
	"io"
	"net/http"
	"os"
	"strings"

	"github.com/spf13/cobra"

	"github.com/xunull/jdan/internal/cookiex"
)

type cookieCmdDeps struct {
	out    io.Writer
	in     io.Reader
	client *http.Client
}

func newCookieCommand(deps cookieCmdDeps) *cobra.Command {
	if deps.out == nil {
		deps.out = os.Stdout
	}
	if deps.in == nil {
		deps.in = os.Stdin
	}
	cmd := &cobra.Command{
		Use:   "cookie [url|set-cookie]",
		Short: "解析 Set-Cookie / Cookie 并做安全体检",
		Long: `解析 Set-Cookie 头（含属性）成可读表格，并揪出安全问题（缺 Secure / HttpOnly、
SameSite=None 无 Secure、__Host-/__Secure- 前缀规则、过宽 Domain）。解析用 stdlib
http.ParseSetCookie，0 依赖。

输入三选一：
  jdan cookie https://example.com                        # 抓 URL，取全部 Set-Cookie
  jdan cookie "sid=abc; Path=/; Secure; HttpOnly; SameSite=Lax"   # 直接给一条 Set-Cookie
  echo "sid=abc; Secure" | jdan cookie                   # stdin

含 = → 当头值解析；否则当 URL 抓。--request 把输入当请求 Cookie 头（只列 name=value 对）。`,
		Args:          cobra.MaximumNArgs(1),
		SilenceUsage:  true,
		SilenceErrors: true,
		RunE: func(cmd *cobra.Command, args []string) error {
			asJSON, _ := cmd.Flags().GetBool("json")
			asRequest, _ := cmd.Flags().GetBool("request")

			lines, srcURL, err := resolveCookieInput(deps, args)
			if err != nil {
				return err
			}
			if len(lines) == 0 {
				return fmt.Errorf("没有 cookie 内容可解析")
			}

			if asRequest {
				return emitCookieRequest(deps.out, lines[0], asJSON)
			}
			return emitSetCookies(deps.out, srcURL, lines, asJSON)
		},
	}
	cmd.Flags().Bool("json", false, "结构化输出")
	cmd.Flags().Bool("request", false, "把输入当请求 Cookie 头解析（只列 name=value 对，不审计）")
	return cmd
}

// resolveCookieInput：含 = → 头值（单条）；否则当 URL 抓全部 Set-Cookie；无参 → stdin。
func resolveCookieInput(deps cookieCmdDeps, args []string) (lines []string, srcURL string, err error) {
	if len(args) == 0 {
		b, e := io.ReadAll(deps.in)
		if e != nil {
			return nil, "", e
		}
		s := strings.TrimSpace(string(b))
		if s == "" {
			return nil, "", nil
		}
		return []string{s}, "", nil
	}
	arg := args[0]
	if strings.Contains(arg, "=") {
		return []string{arg}, "", nil
	}
	vals, final, e := fetchResponseHeader(deps.client, arg, "Set-Cookie")
	if e != nil {
		return nil, "", e
	}
	if len(vals) == 0 {
		return nil, final, fmt.Errorf("该 URL 没有 Set-Cookie 头")
	}
	return vals, final, nil
}

func emitSetCookies(out io.Writer, srcURL string, lines []string, asJSON bool) error {
	type entry struct {
		Name     string          `json:"name"`
		Value    string          `json:"value"`
		Path     string          `json:"path,omitempty"`
		Domain   string          `json:"domain,omitempty"`
		Secure   bool            `json:"secure"`
		HttpOnly bool            `json:"http_only"`
		SameSite string          `json:"same_site"`
		MaxAge   int             `json:"max_age,omitempty"`
		Issues   []cookiex.Issue `json:"issues"`
		ParseErr string          `json:"parse_error,omitempty"`
	}
	var entries []entry
	for _, line := range lines {
		c, err := cookiex.ParseSetCookie(line)
		if err != nil || c == nil {
			entries = append(entries, entry{ParseErr: fmt.Sprintf("解析失败：%v", err)})
			continue
		}
		entries = append(entries, entry{
			Name: c.Name, Value: c.Value, Path: c.Path, Domain: c.Domain,
			Secure: c.Secure, HttpOnly: c.HttpOnly,
			SameSite: cookiex.SameSiteName(c.SameSite), MaxAge: c.MaxAge,
			Issues: cookiex.Audit(c),
		})
	}

	if asJSON {
		return writeIndentJSON(out, map[string]any{"url": srcURL, "cookies": entries})
	}

	if srcURL != "" {
		fmt.Fprintf(out, "来源: %s\n", srcURL)
	}
	for i, e := range entries {
		if i > 0 {
			fmt.Fprintln(out)
		}
		if e.ParseErr != "" {
			fmt.Fprintf(out, "⚠ %s\n", e.ParseErr)
			continue
		}
		fmt.Fprintf(out, "%s = %s\n", e.Name, e.Value)
		fmt.Fprintf(out, "  Path=%s Domain=%s Secure=%v HttpOnly=%v SameSite=%s",
			orDash(e.Path), orDash(e.Domain), e.Secure, e.HttpOnly, e.SameSite)
		if e.MaxAge != 0 {
			fmt.Fprintf(out, " Max-Age=%d", e.MaxAge)
		}
		fmt.Fprintln(out)
		if len(e.Issues) == 0 {
			fmt.Fprintln(out, "  ✓ 未发现问题")
		}
		for _, is := range e.Issues {
			mark := "⚠"
			if is.Level == "info" {
				mark = "·"
			}
			fmt.Fprintf(out, "  %s %s\n", mark, is.Msg)
		}
	}
	return nil
}

func emitCookieRequest(out io.Writer, line string, asJSON bool) error {
	cookies, err := cookiex.ParseCookie(line)
	if err != nil {
		return fmt.Errorf("解析请求 Cookie 失败：%w", err)
	}
	type pair struct {
		Name  string `json:"name"`
		Value string `json:"value"`
	}
	pairs := make([]pair, 0, len(cookies))
	for _, c := range cookies {
		pairs = append(pairs, pair{c.Name, c.Value})
	}
	if asJSON {
		return writeIndentJSON(out, map[string]any{"cookies": pairs})
	}
	for _, p := range pairs {
		fmt.Fprintf(out, "%-24s %s\n", p.Name, p.Value)
	}
	return nil
}

func init() {
	rootCmd.AddCommand(newCookieCommand(cookieCmdDeps{}))
}
