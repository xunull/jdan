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
	"github.com/xunull/jdan/internal/sechdrx"
)

type httpGradeDeps struct {
	out    io.Writer
	client *http.Client // 注入用；nil 时按 flag 构造
}

func newHTTPGradeCommand(deps httpGradeDeps) *cobra.Command {
	if deps.out == nil {
		deps.out = os.Stdout
	}
	cmd := &cobra.Command{
		Use:   "grade <url>",
		Short: "给站点的安全响应头打分（A+~F）",
		Long: `拉一个 URL，看安全相关的响应头有没有、配得好不好，给字母等级 A+~F +
分项 pass/warn/fail + 修复建议。风格同 securityheaders.com，0 新依赖（复用
http headers 的抓取）。

评级看核心 6 项：HSTS / CSP / X-Content-Type-Options / X-Frame-Options /
Referrer-Policy / Permissions-Policy；并对信息泄露头（Server 带版本号、
X-Powered-By 等）反向扣分。跨源隔离 COOP/COEP/CORP 默认只提示，--strict 才计入。

例：
  jdan http grade github.com               # 无 scheme 自动补 https://
  jdan http grade https://example.com --json
  jdan http grade example.com --strict     # 把跨源隔离纳入评级
  jdan http grade example.com --fail-under B   # 低于 B 则退出非 0（CI 卡门）

退出码：默认恒 0（它是评估报告）；只有设了 --fail-under 且实际等级更低时才非 0。
有意不做：不主动扫漏洞/不发探测 payload、只读一次正常响应头；不代改服务器配置。`,
		Args:          cobra.ExactArgs(1),
		SilenceUsage:  true,
		SilenceErrors: true,
		RunE: func(cmd *cobra.Command, args []string) error {
			insecure, _ := cmd.Flags().GetBool("insecure")
			asJSON, _ := cmd.Flags().GetBool("json")
			timeout, _ := cmd.Flags().GetDuration("timeout")
			maxRedirects, _ := cmd.Flags().GetInt("max-redirects")
			strict, _ := cmd.Flags().GetBool("strict")
			failUnder, _ := cmd.Flags().GetString("fail-under")

			if maxRedirects < 0 {
				return fmt.Errorf("--max-redirects 不能为负")
			}
			if failUnder != "" && sechdrx.Rank(failUnder) == 0 {
				return fmt.Errorf("--fail-under 非法等级 %q（应为 A+/A/B/C/D/F）", failUnder)
			}

			client := deps.client
			if client == nil {
				client = &http.Client{Timeout: timeout}
				if insecure {
					client.Transport = &http.Transport{TLSClientConfig: &tls.Config{InsecureSkipVerify: true}}
				}
			}

			hops, ferr := httphdr.Fetch(client, httphdr.EnsureScheme(args[0]), "GET", nil, maxRedirects)
			if ferr != nil {
				return ferr
			}
			if len(hops) == 0 {
				return fmt.Errorf("没有拿到响应")
			}
			final := hops[len(hops)-1]
			isHTTPS := strings.HasPrefix(strings.ToLower(final.URL), "https://")

			report := sechdrx.Grade(final.Header, isHTTPS, sechdrx.Options{Strict: strict})
			report.URL = final.URL

			if asJSON {
				s, err := report.FormatJSON()
				if err != nil {
					return err
				}
				fmt.Fprintln(deps.out, s)
			} else {
				fmt.Fprint(deps.out, report.Render())
			}

			if failUnder != "" && sechdrx.Rank(report.Grade) < sechdrx.Rank(failUnder) {
				return fmt.Errorf("等级 %s 低于阈值 %s", report.Grade, strings.ToUpper(failUnder))
			}
			return nil
		},
	}
	cmd.Flags().BoolP("insecure", "k", false, "跳过 TLS 证书验证")
	cmd.Flags().Bool("json", false, "JSON 输出")
	cmd.Flags().Duration("timeout", 10*time.Second, "整体超时")
	cmd.Flags().Int("max-redirects", 10, "最多跟几跳重定向（评级看最终页）")
	cmd.Flags().Bool("strict", false, "把 COOP/COEP/CORP 跨源隔离纳入评级")
	cmd.Flags().String("fail-under", "", "低于此等级则退出非 0（如 B），默认不卡门")
	return cmd
}

func init() {
	httpCmd.AddCommand(newHTTPGradeCommand(httpGradeDeps{}))
}
