package cli

import (
	"crypto/tls"
	"fmt"
	"io"
	"net/http"
	"os"
	"strings"
	"time"

	"github.com/spf13/cobra"

	"github.com/xunull/jdan/internal/httphdr"
)

type httpHeadersDeps struct {
	out    io.Writer
	client *http.Client // 注入用；nil 时按 flag 构造
}

func newHTTPHeadersCommand(deps httpHeadersDeps) *cobra.Command {
	if deps.out == nil {
		deps.out = os.Stdout
	}
	cmd := &cobra.Command{
		Use:   "headers <url>",
		Short: "看响应头 + 完整重定向链",
		Long: `拉一个 URL，打印状态行 + 响应头 + 完整重定向链（逐跳显示）。
比 curl -I 好读，0 新依赖（纯 stdlib）。

手动跟重定向（不靠 client 自动跟），逐跳展示每一跳的 status/Location/响应头。
默认 GET 但只读响应头、不下载 body。

例：
  jdan http headers github.com            # 无 scheme 自动补 https://
  jdan http headers https://example.com --max-redirects 0   # 不跟转
  jdan http headers <url> -a              # 每一跳都打全部头
  jdan http headers <url> -H "Authorization: Bearer x"
  jdan http headers <url> --json`,
		Args:          cobra.ExactArgs(1),
		SilenceUsage:  true,
		SilenceErrors: true,
		RunE: func(cmd *cobra.Command, args []string) error {
			method, _ := cmd.Flags().GetString("method")
			maxRedirects, _ := cmd.Flags().GetInt("max-redirects")
			showAll, _ := cmd.Flags().GetBool("all")
			rawHeaders, _ := cmd.Flags().GetStringArray("header")
			insecure, _ := cmd.Flags().GetBool("insecure")
			asJSON, _ := cmd.Flags().GetBool("json")
			timeout, _ := cmd.Flags().GetDuration("timeout")

			if maxRedirects < 0 {
				return fmt.Errorf("--max-redirects 不能为负")
			}
			reqHeader, err := parseRequestHeaders(rawHeaders)
			if err != nil {
				return err
			}

			client := deps.client
			if client == nil {
				client = &http.Client{Timeout: timeout}
				if insecure {
					client.Transport = &http.Transport{TLSClientConfig: &tls.Config{InsecureSkipVerify: true}}
				}
			}

			hops, ferr := httphdr.Fetch(client, httphdr.EnsureScheme(args[0]), method, reqHeader, maxRedirects)

			if asJSON {
				s, jerr := httphdr.FormatJSON(hops)
				if jerr != nil {
					return jerr
				}
				fmt.Fprintln(deps.out, s)
			} else if len(hops) > 0 {
				fmt.Fprint(deps.out, httphdr.FormatText(hops, showAll))
			}
			return ferr
		},
	}
	cmd.Flags().StringP("method", "X", "GET", "请求方法（GET/HEAD/POST…）")
	cmd.Flags().Int("max-redirects", 10, "最多跟几跳重定向（0 = 不跟）")
	cmd.Flags().BoolP("all", "a", false, "每一跳都打全部响应头（默认重定向跳只显 Location）")
	cmd.Flags().StringArrayP("header", "H", nil, `加请求头（可重复），如 -H "User-Agent: x"`)
	cmd.Flags().BoolP("insecure", "k", false, "跳过 TLS 证书验证")
	cmd.Flags().Bool("json", false, "JSON 数组输出")
	cmd.Flags().Duration("timeout", 10*time.Second, "整体超时")
	return cmd
}

// parseRequestHeaders 把 "Key: Value" 形式解析成 http.Header。
func parseRequestHeaders(raw []string) (http.Header, error) {
	h := http.Header{}
	for _, s := range raw {
		k, v, ok := strings.Cut(s, ":")
		if !ok || strings.TrimSpace(k) == "" {
			return nil, fmt.Errorf("非法请求头 %q（应为 \"Key: Value\"）", s)
		}
		h.Add(strings.TrimSpace(k), strings.TrimSpace(v))
	}
	return h, nil
}

func init() {
	httpCmd.AddCommand(newHTTPHeadersCommand(httpHeadersDeps{}))
}
