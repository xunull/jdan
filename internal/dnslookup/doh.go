package dnslookup

import (
	"bytes"
	"context"
	"fmt"
	"io"
	"net"
	"net/http"
	"net/url"
	"sort"
	"strings"
	"time"

	"github.com/miekg/dns"
)

// DoHTarget 描述一个 DoH endpoint。
//
// BootstrapIPs 非空时，HTTP 客户端会绕过 OS resolver 直接 dial 这些 IP；
// TLS SNI 仍是 URL 的 host，证书验证不破。这是 curl --resolve / dig +https 的同款机制。
// 用途：在本地 DNS 被劫持的环境下仍能可靠访问已知 DoH 提供商。
type DoHTarget struct {
	URL          string
	BootstrapIPs []string
}

// providerAliases 内置 6 个常用 DoH 提供商。
//
// 这一份数据同时承担两个职责：
//   - 别名 → URL 映射（用户输入 "google" 得到完整 endpoint）
//   - host → bootstrap IP 映射（绕过本地 DNS）
//
// IP 多年未变（8.8.8.8 / 1.1.1.1 / 9.9.9.9 都列入了 RFC 或提供商首页），
// 若有变化只需 patch 一行。
var providerAliases = map[string]DoHTarget{
	"google":     {URL: "https://dns.google/dns-query", BootstrapIPs: []string{"8.8.8.8", "8.8.4.4"}},
	"cloudflare": {URL: "https://cloudflare-dns.com/dns-query", BootstrapIPs: []string{"1.1.1.1", "1.0.0.1"}},
	"quad9":      {URL: "https://dns.quad9.net/dns-query", BootstrapIPs: []string{"9.9.9.9", "149.112.112.112"}},
	"opendns":    {URL: "https://doh.opendns.com/dns-query", BootstrapIPs: []string{"208.67.222.222", "208.67.220.220"}},
	"ali":        {URL: "https://dns.alidns.com/dns-query", BootstrapIPs: []string{"223.5.5.5", "223.6.6.6"}},
	"360":        {URL: "https://doh.360.cn/dns-query", BootstrapIPs: []string{"101.226.4.6", "218.30.118.6"}},
}

// ProviderAliases 返回内置别名列表（字母序），供 --help 文案使用。
func ProviderAliases() []string {
	out := make([]string, 0, len(providerAliases))
	for k := range providerAliases {
		out = append(out, k)
	}
	sort.Strings(out)
	return out
}

// IsDoHURL 判断字符串是否是 DoH URL（必须 https:// 前缀）。
// 调用方据此决定走 dohResolver 还是 miekgResolver。
func IsDoHURL(s string) bool {
	return strings.HasPrefix(strings.ToLower(s), "https://")
}

// ResolveDoHTarget 把用户输入解析为 DoHTarget。
//
// 输入优先级：
//  1. 内置别名（大小写不敏感）→ 返回完整 URL + bootstrap IPs
//  2. https:// 开头的完整 URL → 原样使用，无 bootstrap
//  3. 主机名（无 scheme）→ 补全为 https://<host>/dns-query，无 bootstrap
//
// 错误：空串、http:// 等非 https scheme、含空白字符的主机名、URL 解析失败。
func ResolveDoHTarget(s string) (DoHTarget, error) {
	s = strings.TrimSpace(s)
	if s == "" {
		return DoHTarget{}, fmt.Errorf("--doh 不能为空")
	}

	if p, ok := providerAliases[strings.ToLower(s)]; ok {
		return DoHTarget{
			URL:          p.URL,
			BootstrapIPs: append([]string{}, p.BootstrapIPs...),
		}, nil
	}

	if strings.Contains(s, "://") {
		u, err := url.Parse(s)
		if err != nil {
			return DoHTarget{}, fmt.Errorf("--doh URL 无效: %v", err)
		}
		if !strings.EqualFold(u.Scheme, "https") {
			return DoHTarget{}, fmt.Errorf("--doh 仅支持 https:// scheme，传入 %q", u.Scheme)
		}
		if u.Host == "" {
			return DoHTarget{}, fmt.Errorf("--doh URL 缺少 host")
		}
		return DoHTarget{URL: s}, nil
	}

	if strings.ContainsAny(s, " \t\n\r/?#") {
		return DoHTarget{}, fmt.Errorf("--doh 主机名含非法字符: %q（如需带 path 请用完整 https:// URL）", s)
	}
	return DoHTarget{URL: "https://" + s + "/dns-query"}, nil
}

type dohResolver struct {
	client *http.Client
	target DoHTarget
}

// NewDoHResolver 构造走 DoH 协议的 Resolver。
//
// timeout 同时作用于 dial / TLS 握手 / 整个 HTTP 请求。
// 默认验证 TLS 证书（无 --insecure-tls；DoH 的初衷就是加密+认证）。
func NewDoHResolver(target DoHTarget, timeout time.Duration) Resolver {
	transport := &http.Transport{
		TLSHandshakeTimeout:   timeout,
		ResponseHeaderTimeout: timeout,
		DialContext:           newDoHDialContext(target),
		// Go 文档：设置了自定义 DialContext 时 HTTP/2 默认关闭。
		// 必须显式 ForceAttemptHTTP2，否则某些 DoH 服务器（Quad9）会 HTTP 505。
		ForceAttemptHTTP2: true,
	}
	return &dohResolver{
		client: &http.Client{
			Transport: transport,
			Timeout:   timeout,
		},
		target: target,
	}
}

// newDoHResolverWithClient 是测试入口，允许注入已配置好的 http.Client
// （典型用法：httptest.Server.Client() 已带 self-signed 证书的信任）。
func newDoHResolverWithClient(target DoHTarget, client *http.Client) *dohResolver {
	return &dohResolver{client: client, target: target}
}

// resolveDialAddr 决定 dial 时实际使用的地址列表。
//
//   - 无 bootstrap 或 targetHost 解析失败 → [addr] 原样
//   - addr host ≠ targetHost（如 SNI 二级 host 不同）→ [addr] 原样
//   - 命中：返回 [bootstrap1:port, bootstrap2:port, ...]
//
// 永远返回至少一个候选地址。
func resolveDialAddr(target DoHTarget, addr, targetHost string) []string {
	if len(target.BootstrapIPs) == 0 || targetHost == "" {
		return []string{addr}
	}
	host, port, err := net.SplitHostPort(addr)
	if err != nil || host != targetHost {
		return []string{addr}
	}
	out := make([]string, 0, len(target.BootstrapIPs))
	for _, ip := range target.BootstrapIPs {
		out = append(out, net.JoinHostPort(ip, port))
	}
	return out
}

// newDoHDialContext 返回一个用于 http.Transport.DialContext 的函数。
//
// 命中 bootstrap 时，依次尝试每个 IP；全部失败才返回最后一个错误。
func newDoHDialContext(target DoHTarget) func(ctx context.Context, network, addr string) (net.Conn, error) {
	targetHost := dohURLHost(target.URL)
	var dialer net.Dialer
	return func(ctx context.Context, network, addr string) (net.Conn, error) {
		addrs := resolveDialAddr(target, addr, targetHost)
		var lastErr error
		for _, a := range addrs {
			conn, err := dialer.DialContext(ctx, network, a)
			if err == nil {
				return conn, nil
			}
			lastErr = err
		}
		return nil, lastErr
	}
}

// dohURLHost 抽取 URL 的 host（不含 port）。解析失败返回空串。
func dohURLHost(urlStr string) string {
	u, err := url.Parse(urlStr)
	if err != nil || u == nil {
		return ""
	}
	return u.Hostname()
}

// Query 实现 Resolver 接口。
//
// 第 4 个参数 server 在 DoH 路径下被忽略——endpoint 已通过 NewDoHResolver 的
// target 固定。Lookup 层透过这个参数的是 opts.Server 字符串，dohResolver 用不上。
func (r *dohResolver) Query(ctx context.Context, domain string, qtype uint16, _ string) (*dns.Msg, error) {
	msg := new(dns.Msg)
	msg.SetQuestion(dns.Fqdn(domain), qtype)
	msg.RecursionDesired = true

	body, err := msg.Pack()
	if err != nil {
		return nil, fmt.Errorf("PACK_ERROR: %v", err)
	}

	req, err := http.NewRequestWithContext(ctx, http.MethodPost, r.target.URL, bytes.NewReader(body))
	if err != nil {
		return nil, fmt.Errorf("REQUEST_ERROR: %v", err)
	}
	req.Header.Set("Content-Type", "application/dns-message")
	req.Header.Set("Accept", "application/dns-message")
	// 部分 DoH 服务器（如 Quad9）对空 User-Agent 返回 HTTP 505，加一个明确标识。
	req.Header.Set("User-Agent", "jdan-dns/1.0")

	resp, err := r.client.Do(req)
	if err != nil {
		return nil, friendlyDoHErr(err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		return nil, fmt.Errorf("HTTP_%d", resp.StatusCode)
	}

	respBytes, err := io.ReadAll(resp.Body)
	if err != nil {
		return nil, friendlyDoHErr(err)
	}

	out := new(dns.Msg)
	if err := out.Unpack(respBytes); err != nil {
		return nil, fmt.Errorf("UNPACK_ERROR: %v", err)
	}
	return out, nil
}

// friendlyDoHErr 把 net/http 的长错误翻译为短标识，与 friendlyErr 的风格一致，
// 便于在 text 表格里展示。lookup.go 的 friendlyErr 对这些短标识走 default 透传。
func friendlyDoHErr(err error) error {
	msg := err.Error()
	switch {
	case strings.Contains(msg, "tls:"),
		strings.Contains(msg, "x509:"),
		strings.Contains(msg, "certificate"):
		return fmt.Errorf("TLS_ERROR")
	case strings.Contains(msg, "Client.Timeout exceeded"),
		strings.Contains(msg, "context deadline exceeded"),
		strings.Contains(msg, "context canceled"),
		strings.Contains(msg, "i/o timeout"),
		strings.Contains(msg, "TLS handshake timeout"):
		return fmt.Errorf("TIMEOUT")
	case strings.Contains(msg, "connection refused"):
		return fmt.Errorf("CONNECTION_REFUSED")
	case strings.Contains(msg, "no such host"):
		return fmt.Errorf("NO_SUCH_HOST")
	default:
		return err
	}
}
