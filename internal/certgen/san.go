package certgen

import (
	"net"
	"strings"
)

// SANs 是分类后的 Subject Alternative Names。
type SANs struct {
	DNS []string
	IPs []net.IP
}

// BuildSANs 从主参数 + 额外 DNS/IP 列表组装 SAN。
//   - primary 是命令主参数（localhost / example.local / 127.0.0.1）：
//     若是 IP 字面量进 IPs，否则进 DNS
//   - extraDNS / extraIP 是 --san / --ip 的 csv 拆分结果
//
// 自动去重，保持插入顺序。
func BuildSANs(primary string, extraDNS, extraIP []string) SANs {
	var s SANs
	seenDNS := map[string]bool{}
	seenIP := map[string]bool{}

	addDNS := func(name string) {
		name = strings.TrimSpace(name)
		if name == "" || seenDNS[name] {
			return
		}
		seenDNS[name] = true
		s.DNS = append(s.DNS, name)
	}
	addIP := func(raw string) {
		raw = strings.TrimSpace(raw)
		if raw == "" {
			return
		}
		ip := net.ParseIP(raw)
		if ip == nil || seenIP[ip.String()] {
			return
		}
		seenIP[ip.String()] = true
		s.IPs = append(s.IPs, ip)
	}

	// primary：IP 字面量 → IP SAN，否则 DNS SAN
	primary = strings.TrimSpace(primary)
	if primary != "" {
		if ip := net.ParseIP(primary); ip != nil {
			addIP(primary)
		} else {
			addDNS(primary)
		}
	}
	for _, d := range extraDNS {
		addDNS(d)
	}
	for _, ip := range extraIP {
		addIP(ip)
	}
	return s
}

