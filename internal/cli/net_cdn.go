package cli

import (
	"context"
	"crypto/tls"
	"fmt"
	"io"
	"net"
	"net/http"
	"net/netip"
	"net/url"
	"os"
	"strings"
	"time"

	"github.com/spf13/cobra"

	"github.com/xunull/jdan/internal/cdnx"
	"github.com/xunull/jdan/internal/httphdr"
)

// cdnCmdDeps 的三个采集函数是注入点：nil 时用真实网络实现，测试里换成 mock 喂数据。
type cdnCmdDeps struct {
	out          io.Writer
	fetchHeaders func(ctx context.Context, rawURL string, insecure bool, maxRedirects int, timeout time.Duration) (finalURL string, headers map[string]string, err error)
	lookupNS     func(ctx context.Context, host string) ([]string, error)
	lookupIPs    func(ctx context.Context, host string) ([]netip.Addr, error)
}

func newCDNCommand(deps cdnCmdDeps) *cobra.Command {
	if deps.out == nil {
		deps.out = os.Stdout
	}
	if deps.fetchHeaders == nil {
		deps.fetchHeaders = realFetchHeaders
	}
	if deps.lookupNS == nil {
		deps.lookupNS = realLookupNS
	}
	if deps.lookupIPs == nil {
		deps.lookupIPs = realLookupIPs
	}

	cmd := &cobra.Command{
		Use:   "cdn <url>",
		Short: "识别站点前面挂的 CDN/WAF（Cloudflare/CloudFront/Akamai/Fastly）",
		Long: `判断一个网址前面是不是挂了 CDN/WAF，挂的是哪家。0 新依赖（纯 stdlib）。

三路互相独立的信号，任一命中即报，多路一致定性"确定"：
  · HTTP 响应头指纹（如 Cloudflare 的 CF-RAY、CloudFront 的 x-amz-cf-id）
  · DNS NS 记录（域名是否托管在该 CDN 的 DNS，如 *.ns.cloudflare.com）
  · 解析 IP 是否落在该 CDN 公布的网段（目前仅 Cloudflare 内嵌完整段）

Cloudflare 支持最深：还会从 CF-RAY 解出边缘机房（IATA 机场码）。

例：
  jdan net cdn example.com              # 无 scheme 自动补 https://
  jdan net cdn https://x.com --json     # 机读
  jdan net cdn x.com --headers-only     # 只看响应头，不做 DNS/IP 解析（快）

退出码：检测到 = 0，没检测到 = 非 0（文本模式，可进 CI）；--json 恒 0，看 .detected。`,
		Args:          cobra.ExactArgs(1),
		SilenceUsage:  true,
		SilenceErrors: true,
		RunE: func(cmd *cobra.Command, args []string) error {
			headersOnly, _ := cmd.Flags().GetBool("headers-only")
			insecure, _ := cmd.Flags().GetBool("insecure")
			asJSON, _ := cmd.Flags().GetBool("json")
			maxRedirects, _ := cmd.Flags().GetInt("max-redirects")
			timeout, _ := cmd.Flags().GetDuration("timeout")

			if maxRedirects < 0 {
				return fmt.Errorf("--max-redirects 不能为负")
			}

			rawURL := httphdr.EnsureScheme(args[0])
			ctx, cancel := context.WithTimeout(cmd.Context(), timeout+5*time.Second)
			defer cancel()

			finalURL, headers, err := deps.fetchHeaders(ctx, rawURL, insecure, maxRedirects, timeout)
			if err != nil {
				return fmt.Errorf("拉取 %s 失败：%w", rawURL, err)
			}
			host := hostFromURL(finalURL)

			var ns []string
			var ips []netip.Addr
			if !headersOnly && host != "" {
				ns, _ = deps.lookupNS(ctx, host)  // DNS 失败不致命，降级为只用头判
				ips, _ = deps.lookupIPs(ctx, host) // 同上
			}

			res := cdnx.Detect(headers, ns, ips, cdnx.DefaultProviders())
			res.FinalURL = finalURL
			res.Host = host
			res.NS = ns
			res.IPs = addrsToStrings(ips)

			if asJSON {
				s, jerr := cdnx.FormatJSON(res)
				if jerr != nil {
					return jerr
				}
				fmt.Fprintln(deps.out, s)
				return nil // JSON 模式恒 0，脚本靠 .detected 判
			}

			fmt.Fprint(deps.out, cdnx.Render(res))
			if !res.Detected() {
				return fmt.Errorf("未检测到 CDN")
			}
			return nil
		},
	}
	cmd.Flags().Bool("headers-only", false, "只看响应头，跳过 DNS NS / IP 段解析")
	cmd.Flags().BoolP("insecure", "k", false, "跳过 TLS 证书验证")
	cmd.Flags().Bool("json", false, "JSON 输出")
	cmd.Flags().Int("max-redirects", 10, "最多跟几跳重定向（0 = 不跟）")
	cmd.Flags().Duration("timeout", 10*time.Second, "单步超时")
	return cmd
}

// realFetchHeaders 复用 httphdr.Fetch：手动跟重定向、逐跳带响应头、不下 body。
// 取最后一跳的 URL + 响应头（键统一小写）喂给检测器。
func realFetchHeaders(_ context.Context, rawURL string, insecure bool, maxRedirects int, timeout time.Duration) (string, map[string]string, error) {
	client := &http.Client{Timeout: timeout * 2}
	if insecure {
		client.Transport = &http.Transport{TLSClientConfig: &tls.Config{InsecureSkipVerify: true}}
	}
	reqHeader := http.Header{}
	reqHeader.Set("User-Agent", "jdan-net-cdn/1.0")

	hops, err := httphdr.Fetch(client, rawURL, "GET", reqHeader, maxRedirects)
	if len(hops) == 0 {
		if err != nil {
			return "", nil, err
		}
		return "", nil, fmt.Errorf("无响应")
	}
	last := hops[len(hops)-1]
	return last.URL, lowerHeaders(last.Header), nil
}

func realLookupNS(ctx context.Context, host string) ([]string, error) {
	// NS 记录在区顶（registrable domain），子域多半查不到——从全名往上逐级试，
	// 命中第一个有 NS 的层级（即委派点）。不依赖 PSL，2 标签处停。
	labels := strings.Split(strings.TrimSuffix(host, "."), ".")
	for i := 0; i+1 < len(labels); i++ {
		cand := strings.Join(labels[i:], ".")
		nss, err := net.DefaultResolver.LookupNS(ctx, cand)
		if err == nil && len(nss) > 0 {
			out := make([]string, len(nss))
			for j, n := range nss {
				out[j] = strings.TrimSuffix(n.Host, ".")
			}
			return out, nil
		}
	}
	return nil, fmt.Errorf("无 NS 记录")
}

func realLookupIPs(ctx context.Context, host string) ([]netip.Addr, error) {
	ips, err := net.DefaultResolver.LookupNetIP(ctx, "ip", host)
	if err != nil {
		return nil, err
	}
	out := make([]netip.Addr, 0, len(ips))
	for _, ip := range ips {
		out = append(out, ip.Unmap()) // 去掉 IPv4-in-IPv6 包装，统一成裸 v4/v6
	}
	return out, nil
}

func hostFromURL(raw string) string {
	u, err := url.Parse(raw)
	if err != nil {
		return ""
	}
	return u.Hostname()
}

func lowerHeaders(h http.Header) map[string]string {
	m := make(map[string]string, len(h))
	for k, vs := range h {
		m[strings.ToLower(k)] = strings.Join(vs, ", ")
	}
	return m
}

func addrsToStrings(addrs []netip.Addr) []string {
	if len(addrs) == 0 {
		return nil
	}
	out := make([]string, len(addrs))
	for i, a := range addrs {
		out[i] = a.String()
	}
	return out
}

func init() {
	netCmd.AddCommand(newCDNCommand(cdnCmdDeps{}))
}
