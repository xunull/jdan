package cdnx

import "net/netip"

// cloudflareCIDRs 是 Cloudflare 官方公布的边缘网段（内嵌、0 依赖）。
// 来源：https://www.cloudflare.com/ips-v4 + https://www.cloudflare.com/ips-v6
// 取数日期：2026-06。段很稳定，若要刷新，替换本表即可——cdnx_test.go 里
// TestCloudflareRanges_AllParse 做往返保证：每条都能 ParsePrefix 通过。
var cloudflareCIDRs = []string{
	// IPv4
	"173.245.48.0/20",
	"103.21.244.0/22",
	"103.22.200.0/22",
	"103.31.4.0/22",
	"141.101.64.0/18",
	"108.162.192.0/18",
	"190.93.240.0/20",
	"188.114.96.0/20",
	"197.234.240.0/22",
	"198.41.128.0/17",
	"162.158.0.0/15",
	"104.16.0.0/13",
	"104.24.0.0/14",
	"172.64.0.0/13",
	"131.0.72.0/22",
	// IPv6
	"2400:cb00::/32",
	"2606:4700::/32",
	"2803:f800::/32",
	"2405:b500::/32",
	"2405:8100::/32",
	"2a06:98c0::/29",
	"2c0f:f248::/32",
}

// cloudflareRanges 把内嵌 CIDR 字面量解析成 netip.Prefix。无法解析的条目被跳过
// （测试保证不会发生），所以这里不 panic、也不在 init 期崩。
func cloudflareRanges() []netip.Prefix {
	out := make([]netip.Prefix, 0, len(cloudflareCIDRs))
	for _, c := range cloudflareCIDRs {
		if p, err := netip.ParsePrefix(c); err == nil {
			out = append(out, p)
		}
	}
	return out
}

// DefaultProviders 返回内置识别表。Cloudflare 支持最深（头 + NS + IP 段三路），
// 其余几家走响应头指纹（无公开稳定强指纹的按"很可能"报，诚实标注）。
func DefaultProviders() []Provider {
	return []Provider{
		{
			Name: "Cloudflare",
			Headers: []HeaderSig{
				{Key: "cf-ray", Strong: true},
				{Key: "server", Contains: "cloudflare"},
				{Key: "cf-cache-status"},
				{Key: "cf-mitigated"},
			},
			NSSuffixes: []string{".ns.cloudflare.com"},
			Ranges:     cloudflareRanges(),
		},
		{
			Name: "Amazon CloudFront",
			Headers: []HeaderSig{
				{Key: "x-amz-cf-id", Strong: true},
				{Key: "x-amz-cf-pop"},
				{Key: "via", Contains: "cloudfront"},
				{Key: "server", Contains: "cloudfront"},
			},
		},
		{
			Name: "Akamai",
			Headers: []HeaderSig{
				{Key: "x-akamai-request-id", Strong: true},
				{Key: "akamai-grn", Strong: true},
				{Key: "x-akamai-transformed"},
				{Key: "server", Contains: "akamaighost"},
			},
		},
		{
			Name: "Fastly",
			Headers: []HeaderSig{
				// Fastly 基于 Varnish，无公开稳定的强指纹头：x-served-by/via varnish
				// 在其它 Varnish CDN 上也可能出现，故按启发式（很可能）报。
				{Key: "x-served-by", Contains: "cache-"},
				{Key: "via", Contains: "varnish"},
				{Key: "fastly-debug-digest"},
				{Key: "x-fastly-request-id", Strong: true},
			},
		},
	}
}
