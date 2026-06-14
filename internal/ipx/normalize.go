package ipx

import (
	"errors"
	"net/netip"
)

// Normalize 输出 addr 的 compact 或 expanded 形式。
//   - IPv4: 永远返回 canonical 形式（IPv4 没有 expand/compact 概念）
//   - IPv6 expanded=true: 完整 8 段 16 进制（"2001:0db8:0000:..."）
//   - IPv6 expanded=false: RFC 5952 compact 形式（"2001:db8::1"）
func Normalize(addr netip.Addr, expanded bool) (string, error) {
	if !addr.IsValid() {
		return "", errors.New("invalid address")
	}
	if addr.Is4() {
		return addr.String(), nil
	}
	if expanded {
		return addr.StringExpanded(), nil
	}
	return addr.String(), nil
}
