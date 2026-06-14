// Package ipx 实现 jdan ip 命令的核心：IP/CIDR 计算 + RFC 分类 + 子网划分 +
// IPv6 normalize。全部基于 net/netip（Go 1.18+ 的现代 IP 类型，不可变 +
// value-comparable）。0 新依赖。
package ipx

import (
	"net/netip"
)

// Classification 是 IP 的 RFC 分类标签集合。
// 多个可能同时 true（少见，比如 multicast + link-local）。
type Classification struct {
	Private       bool `json:"private,omitempty"`        // RFC 1918 (v4) + RFC 4193 ULA (v6)
	Loopback      bool `json:"loopback,omitempty"`       // 127/8 + ::1
	Multicast     bool `json:"multicast,omitempty"`      // 224/4 + ff00::/8
	LinkLocal     bool `json:"link_local,omitempty"`     // 169.254/16 + fe80::/10
	Unspecified   bool `json:"unspecified,omitempty"`    // 0.0.0.0 + ::
	Documentation bool `json:"documentation,omitempty"`  // RFC 5737 v4 + RFC 3849 v6 (2001:db8::/32)
	UniqueLocal   bool `json:"unique_local,omitempty"`   // IPv6 fc00::/7
	CGNAT         bool `json:"cgnat,omitempty"`          // IPv4 100.64.0.0/10 (RFC 6598)
	GlobalUnicast bool `json:"global_unicast,omitempty"` // routable on public internet
}

// 预解析的 RFC range，避免每次调用 ParsePrefix。
var (
	docV4Ranges = []netip.Prefix{
		netip.MustParsePrefix("192.0.2.0/24"),    // TEST-NET-1
		netip.MustParsePrefix("198.51.100.0/24"), // TEST-NET-2
		netip.MustParsePrefix("203.0.113.0/24"),  // TEST-NET-3
	}
	docV6         = netip.MustParsePrefix("2001:db8::/32") // RFC 3849
	cgnatV4       = netip.MustParsePrefix("100.64.0.0/10") // RFC 6598
	uniqueLocalV6 = netip.MustParsePrefix("fc00::/7")      // RFC 4193
)

// Classify 对 addr 应用所有 RFC 分类。netip.Addr 自带 IsPrivate/IsLoopback 等
// 几个标准 helper；其他（doc/CGNAT/ULA 单独标）自己判。
func Classify(addr netip.Addr) Classification {
	return Classification{
		Private:       addr.IsPrivate(),
		Loopback:      addr.IsLoopback(),
		Multicast:     addr.IsMulticast(),
		LinkLocal:     addr.IsLinkLocalUnicast() || addr.IsLinkLocalMulticast(),
		Unspecified:   addr.IsUnspecified(),
		GlobalUnicast: addr.IsGlobalUnicast(),
		Documentation: isDocumentation(addr),
		UniqueLocal:   isUniqueLocal(addr),
		CGNAT:         isCGNAT(addr),
	}
}

func isDocumentation(addr netip.Addr) bool {
	if addr.Is4() {
		for _, p := range docV4Ranges {
			if p.Contains(addr) {
				return true
			}
		}
		return false
	}
	return docV6.Contains(addr)
}

func isUniqueLocal(addr netip.Addr) bool {
	if !addr.Is6() || addr.Is4In6() {
		return false
	}
	return uniqueLocalV6.Contains(addr)
}

func isCGNAT(addr netip.Addr) bool {
	return addr.Is4() && cgnatV4.Contains(addr)
}
