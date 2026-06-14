package whois

import (
	"errors"
	"fmt"
	"net/netip"
	"strings"
)

// IANARoot 是 IANA WHOIS server。tldServers 没命中时走 IANA 拿真实 TLD server。
// 是 var（不是 const）让测试能 monkey-patch 指向 mock；生产代码应当视为常量。
var IANARoot = "whois.iana.org"

// ARINRoot 是 IP WHOIS 起点。ARIN 管 NA 区段，其他 RIR 通过 referral 跟随。
// 也是 var 以便测试 override。
var ARINRoot = "whois.arin.net"

// detectKind 返回 target 是 domain / ipv4 / ipv6。
func detectKind(target string) (Kind, error) {
	if addr, err := netip.ParseAddr(target); err == nil {
		if addr.Is4() || addr.Is4In6() {
			return KindIPv4, nil
		}
		return KindIPv6, nil
	}
	if strings.Contains(target, ".") {
		return KindDomain, nil
	}
	return "", fmt.Errorf("cannot detect target type: %q (need domain or IP)", target)
}

// extractTLD 取 domain 最末段作为 TLD。
//   - "example.com" → "com"
//   - "x.example.co.uk" → "uk"（不处理多段 TLD；映射表也用顶级 TLD）
//   - 大小写不敏感，输出全小写
func extractTLD(domain string) string {
	domain = strings.TrimSuffix(strings.ToLower(strings.TrimSpace(domain)), ".")
	if idx := strings.LastIndex(domain, "."); idx >= 0 {
		return domain[idx+1:]
	}
	return domain
}

// tldServers 内置 TLD → WHOIS server 映射，覆盖 ~50 个最常用 TLD。
// 没命中时调用方走 IANA fallback。
//
// 来源：IANA root zone database 的 whois 字段（截至 2026 年常用 TLD 列表）。
// gTLD 数量在持续增长，全列覆盖意义不大；常用的列上来，长尾 IANA 兜底。
var tldServers = map[string]string{
	// Legacy gTLDs
	"com":  "whois.verisign-grs.com",
	"net":  "whois.verisign-grs.com",
	"org":  "whois.publicinterestregistry.org",
	"info": "whois.afilias.net",
	"biz":  "whois.nic.biz",
	"edu":  "whois.educause.edu",
	"gov":  "whois.dotgov.gov",
	"mil":  "whois.nic.mil",
	"int":  "whois.iana.org",

	// Popular new gTLDs
	"io":     "whois.nic.io",
	"ai":     "whois.nic.ai",
	"app":    "whois.nic.google",
	"dev":    "whois.nic.google",
	"xyz":    "whois.nic.xyz",
	"co":     "whois.nic.co",
	"me":     "whois.nic.me",
	"tv":     "whois.nic.tv",
	"cc":     "whois.nic.cc",
	"us":     "whois.nic.us",
	"tech":   "whois.nic.tech",
	"online": "whois.nic.online",
	"site":   "whois.nic.site",
	"store":  "whois.nic.store",

	// Asia ccTLDs
	"cn": "whois.cnnic.cn",
	"jp": "whois.jprs.jp",
	"kr": "whois.kr",
	"tw": "whois.twnic.net.tw",
	"hk": "whois.hkirc.hk",
	"sg": "whois.sgnic.sg",
	"in": "whois.registry.in",
	"id": "whois.id",
	"vn": "whois.vnnic.vn",
	"th": "whois.thnic.co.th",

	// Europe ccTLDs
	"uk": "whois.nic.uk",
	"de": "whois.denic.de",
	"fr": "whois.nic.fr",
	"nl": "whois.domain-registry.nl",
	"es": "whois.nic.es",
	"it": "whois.nic.it",
	"ru": "whois.tcinet.ru",
	"ch": "whois.nic.ch",
	"se": "whois.iis.se",
	"no": "whois.norid.no",
	"pl": "whois.dns.pl",
	"be": "whois.dns.be",
	"at": "whois.nic.at",
	"eu": "whois.eu",

	// Americas ccTLDs
	"ca": "whois.cira.ca",
	"br": "whois.registro.br",
	"mx": "whois.mx",
	"ar": "whois.nic.ar",
	"cl": "whois.nic.cl",

	// Oceania / Africa
	"au": "whois.auda.org.au",
	"nz": "whois.srs.net.nz",
	"za": "whois.registry.net.za",
}

// RoutingFor 决定 target 应该查哪个 server。返回 (server, kind, error)。
//   - IPv4/IPv6: 起点是 ARIN（Lookup 内部跟 referral）
//   - Domain: tldServers 命中 → 直接返回；否则返回 IANA root（fallback）
func RoutingFor(target string) (string, Kind, error) {
	kind, err := detectKind(target)
	if err != nil {
		return "", "", err
	}
	switch kind {
	case KindIPv4, KindIPv6:
		return ARINRoot, kind, nil
	case KindDomain:
		tld := extractTLD(target)
		if s, ok := tldServers[tld]; ok {
			return s, kind, nil
		}
		return IANARoot, kind, nil
	}
	return "", "", errors.New("unreachable")
}

// ParseIANAReferral 从 IANA root response 里提取真实 TLD WHOIS server。
// IANA 响应通常含 "whois: whois.real-server.com" 或 "refer: ..."
func ParseIANAReferral(raw string) string {
	for line := range strings.SplitSeq(raw, "\n") {
		line = strings.TrimSpace(line)
		lower := strings.ToLower(line)
		switch {
		case strings.HasPrefix(lower, "whois:"):
			return strings.TrimSpace(line[len("whois:"):])
		case strings.HasPrefix(lower, "refer:"):
			return strings.TrimSpace(line[len("refer:"):])
		}
	}
	return ""
}

// ParseReferral 从 ARIN/RIPE 等 IP WHOIS 响应里提取 ReferralServer。
//   - ARIN: "ReferralServer: whois://whois.ripe.net"
//   - 一些 server 用 "rwhois://"，也剥掉
func ParseReferral(raw string) string {
	for line := range strings.SplitSeq(raw, "\n") {
		line = strings.TrimSpace(line)
		lower := strings.ToLower(line)
		if strings.HasPrefix(lower, "referralserver:") {
			v := strings.TrimSpace(line[len("referralserver:"):])
			v = strings.TrimPrefix(v, "whois://")
			v = strings.TrimPrefix(v, "rwhois://")
			v = strings.TrimSuffix(v, "/")
			return v
		}
	}
	return ""
}
