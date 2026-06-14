package ipx

import (
	"encoding/binary"
	"fmt"
	"math/big"
	"net/netip"
	"strings"
)

// AddrInfo 是单 IP 地址（非 CIDR）的综合派生信息。
type AddrInfo struct {
	Address        string         `json:"address"`
	Version        int            `json:"version"`
	Compact        string         `json:"compact,omitempty"`  // IPv6 only
	Expanded       string         `json:"expanded,omitempty"` // IPv6 only
	Hex            string         `json:"hex"`
	Decimal        string         `json:"decimal"`
	Binary         string         `json:"binary"`
	ReverseDNS     string         `json:"reverse_dns"`
	Classification Classification `json:"classification"`
}

// ComputeAddrInfo 计算单 IP 的全部派生字段。
func ComputeAddrInfo(addr netip.Addr) AddrInfo {
	info := AddrInfo{
		Address:        addr.String(),
		ReverseDNS:     ReverseDNS(addr),
		Classification: Classify(addr),
	}
	if addr.Is4() {
		info.Version = 4
		b := addr.As4()
		n := binary.BigEndian.Uint32(b[:])
		info.Hex = fmt.Sprintf("0x%08X", n)
		info.Decimal = fmt.Sprintf("%d", n)
		info.Binary = fmt.Sprintf("%08b.%08b.%08b.%08b", b[0], b[1], b[2], b[3])
	} else {
		info.Version = 6
		info.Compact = addr.String()
		info.Expanded = addr.StringExpanded()
		b := addr.As16()
		var hex strings.Builder
		hex.WriteString("0x")
		for _, by := range b {
			fmt.Fprintf(&hex, "%02X", by)
		}
		info.Hex = hex.String()
		info.Decimal = new(big.Int).SetBytes(b[:]).String()
		// Binary: 8 个 16-bit group 用 ":" 分隔
		var bin strings.Builder
		for i := 0; i < 16; i += 2 {
			if i > 0 {
				bin.WriteByte(':')
			}
			group := uint16(b[i])<<8 | uint16(b[i+1])
			fmt.Fprintf(&bin, "%016b", group)
		}
		info.Binary = bin.String()
	}
	return info
}
