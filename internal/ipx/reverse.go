package ipx

import (
	"fmt"
	"net/netip"
	"strings"
)

// ReverseDNS 算 PTR 名（DNS 反向查询 RR 的 owner name）：
//   - IPv4: 倒序 4 个 octet + ".in-addr.arpa"
//     192.168.1.42 → "42.1.168.192.in-addr.arpa"
//   - IPv6: 倒序 32 个 nibble（每 4 位）+ ".ip6.arpa"
//     2001:db8::1 → "1.0.0.0...(32 nibbles)...0.8.b.d.0.1.0.0.2.ip6.arpa"
func ReverseDNS(addr netip.Addr) string {
	if addr.Is4() {
		b := addr.As4()
		return fmt.Sprintf("%d.%d.%d.%d.in-addr.arpa", b[3], b[2], b[1], b[0])
	}
	b := addr.As16()
	var sb strings.Builder
	sb.Grow(72)
	// 32 nibbles：每个 byte 拆成 high + low nibble，按倒序输出
	for i := 15; i >= 0; i-- {
		fmt.Fprintf(&sb, "%x.%x.", b[i]&0x0F, (b[i]>>4)&0x0F)
	}
	sb.WriteString("ip6.arpa")
	return sb.String()
}
