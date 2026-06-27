package cli

import (
	"fmt"
	"io"
	"net/http"
	"os"
	"sort"
	"strings"
	"time"

	"github.com/spf13/cobra"

	"github.com/xunull/jdan/internal/httphdr"
	"github.com/xunull/jdan/internal/metascan"
)

// 默认伪装成常见浏览器 UA：不少站对非浏览器 UA 返回阉割页，浏览器 UA 更贴近真实分享卡片。
const defaultMetaUA = "Mozilla/5.0 (Macintosh; Intel Mac OS X 10_15_7) AppleWebKit/537.36 " +
	"(KHTML, like Gecko) Chrome/120.0.0.0 Safari/537.36"

const maxMetaBody = 512 << 10 // 512 KiB：meta 都在 <head>，不下整个大页面

type metaCmdDeps struct {
	out    io.Writer
	errOut io.Writer
	in     io.Reader
	client *http.Client // 注入便于测试；零值用带超时的默认 client
}

func newMetaCommand(deps metaCmdDeps) *cobra.Command {
	if deps.out == nil {
		deps.out = os.Stdout
	}
	if deps.errOut == nil {
		deps.errOut = os.Stderr
	}
	if deps.in == nil {
		deps.in = os.Stdin
	}
	cmd := &cobra.Command{
		Use:   "meta [url|file]",
		Short: "抓网页 meta / Open Graph / Twitter Card（分享卡片体检）",
		Long: `抓取网页的 <meta> / Open Graph / Twitter Card / canonical / favicon，回答「这链接
分享到微信/Twitter/Slack 时长啥样」，顺手做分享/SEO 体检。复用 x/net/html，0 新依赖。

例：
  jdan meta https://example.com/article
  jdan meta example.com --json
  cat page.html | jdan meta          # 离线解析本地 HTML
  jdan meta page.html                # 解析本地文件

只读静态 HTML、不跑 JS：靠 JS 注入标签的 SPA 抓不到（会如实反映）。`,
		Args:          cobra.MaximumNArgs(1),
		SilenceUsage:  true,
		SilenceErrors: true,
		RunE: func(cmd *cobra.Command, args []string) error {
			asJSON, _ := cmd.Flags().GetBool("json")
			ua, _ := cmd.Flags().GetString("ua")
			timeout, _ := cmd.Flags().GetDuration("timeout")

			var (
				body    io.Reader
				srcURL  string
				cleanup func()
			)
			switch {
			case len(args) == 0:
				body = deps.in
			case fileExists(args[0]):
				f, err := os.Open(args[0])
				if err != nil {
					return fmt.Errorf("无法打开 %q：%w", args[0], err)
				}
				cleanup = func() { f.Close() }
				body = f
			default:
				rc, final, err := fetchMeta(deps.client, args[0], ua, timeout)
				if err != nil {
					return err
				}
				cleanup = func() { rc.Close() }
				body = io.LimitReader(rc, maxMetaBody)
				srcURL = final
			}
			if cleanup != nil {
				defer cleanup()
			}

			meta, err := metascan.ParseMeta(body)
			if err != nil {
				return fmt.Errorf("解析 HTML 失败：%w", err)
			}
			issues := metascan.Audit(meta)

			if asJSON {
				return writeIndentJSON(deps.out, map[string]any{
					"url":    srcURL,
					"meta":   meta,
					"issues": issues,
				})
			}
			emitMetaText(deps.out, srcURL, meta, issues)
			return nil
		},
	}
	cmd.Flags().Bool("json", false, "结构化输出")
	cmd.Flags().String("ua", defaultMetaUA, "User-Agent（默认浏览器 UA；模拟某平台爬虫可改）")
	cmd.Flags().Duration("timeout", 10*time.Second, "抓取超时")
	return cmd
}

func fetchMeta(client *http.Client, raw, ua string, timeout time.Duration) (io.ReadCloser, string, error) {
	if client == nil {
		client = &http.Client{Timeout: timeout}
	}
	url := httphdr.EnsureScheme(raw)
	req, err := http.NewRequest(http.MethodGet, url, nil)
	if err != nil {
		return nil, "", fmt.Errorf("非法 URL %q：%w", raw, err)
	}
	req.Header.Set("User-Agent", ua)
	resp, err := client.Do(req)
	if err != nil {
		return nil, "", fmt.Errorf("抓取失败：%w", err)
	}
	ct := strings.ToLower(resp.Header.Get("Content-Type"))
	if ct != "" && !strings.Contains(ct, "html") {
		resp.Body.Close()
		return nil, "", fmt.Errorf("非 HTML 内容（Content-Type: %s）", resp.Header.Get("Content-Type"))
	}
	final := url
	if resp.Request != nil && resp.Request.URL != nil {
		final = resp.Request.URL.String() // 跟随重定向后的最终 URL
	}
	return resp.Body, final, nil
}

func fileExists(p string) bool {
	info, err := os.Stat(p)
	return err == nil && !info.IsDir()
}

func emitMetaText(out io.Writer, srcURL string, m metascan.Meta, issues []metascan.Issue) {
	line := func(label, val string) {
		if val != "" {
			fmt.Fprintf(out, "%-10s %s\n", label+":", val)
		}
	}
	line("标题", m.Title)
	line("描述", m.Description)
	line("canonical", m.Canonical)
	if srcURL != "" && srcURL != m.Canonical {
		line("最终 URL", srcURL)
	}
	if m.Charset != "" || m.Lang != "" {
		fmt.Fprintf(out, "charset:   %s   lang: %s\n", orDash(m.Charset), orDash(m.Lang))
	}
	line("robots", m.Robots)

	if len(m.OG) > 0 {
		fmt.Fprintln(out, "\n[Open Graph]")
		for _, k := range sortedKeys(m.OG) {
			fmt.Fprintf(out, "  og:%-12s %s\n", k, m.OG[k])
		}
	}
	if len(m.Twitter) > 0 {
		fmt.Fprintln(out, "\n[Twitter Card]")
		for _, k := range sortedKeys(m.Twitter) {
			fmt.Fprintf(out, "  twitter:%-10s %s\n", k, m.Twitter[k])
		}
	}
	if len(m.Icons) > 0 {
		fmt.Fprintln(out, "\n[favicon]")
		for _, ic := range m.Icons {
			fmt.Fprintf(out, "  %s\n", ic)
		}
	}

	fmt.Fprintln(out, "\n体检:")
	if len(issues) == 0 {
		fmt.Fprintln(out, "  ✓ 关键标签齐全")
		return
	}
	for _, is := range issues {
		mark := "⚠"
		if is.Level == "info" {
			mark = "·"
		}
		fmt.Fprintf(out, "  %s %s\n", mark, is.Msg)
	}
}

func orDash(s string) string {
	if s == "" {
		return "—"
	}
	return s
}

func sortedKeys(m map[string]string) []string {
	keys := make([]string, 0, len(m))
	for k := range m {
		keys = append(keys, k)
	}
	sort.Strings(keys)
	return keys
}

func init() {
	rootCmd.AddCommand(newMetaCommand(metaCmdDeps{}))
}
