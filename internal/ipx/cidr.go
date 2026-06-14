package ipx

import (
	"encoding/binary"
	"encoding/json"
	"errors"
	"math/big"
	"net/netip"
)

// CIDRInfo 是 CIDR 的全部派生信息。
type CIDRInfo struct {
	Prefix      netip.Prefix
	Network     netip.Addr // first address（network base）
	Broadcast   netip.Addr // last address（IPv4 broadcast / IPv6 last addr）
	FirstHost   netip.Addr // 第一个 usable host（/31 + /32 特殊处理）
	LastHost    netip.Addr // 最后一个 usable host
	Netmask     netip.Addr // IPv4 only: 255.255.255.0 等
	Wildcard    netip.Addr // IPv4 only: 0.0.0.255 等
	PrefixLen   int
	TotalAddrs  *big.Int // 2^(addrBits - prefixBits)，IPv6 /0 可达 2^128
	UsableAddrs *big.Int // IPv4 时 total-2（/31 + /32 例外）；IPv6 == total
}

// ComputeCIDR 计算一个 prefix 的全部派生字段。
//   - prefix 必须 valid（ParsePrefix 验证过）
//   - 自动 Masked() 对齐到 network base
//   - IPv4 /31 (RFC 3021 point-to-point) 和 /32 算 usable = total
func ComputeCIDR(p netip.Prefix) (CIDRInfo, error) {
	if !p.IsValid() {
		return CIDRInfo{}, errors.New("invalid prefix")
	}
	p = p.Masked()
	addr := p.Addr()
	bits := p.Bits()
	var totalBits int
	switch {
	case addr.Is4():
		totalBits = 32
	case addr.Is6():
		totalBits = 128
	default:
		return CIDRInfo{}, errors.New("address is neither IPv4 nor IPv6")
	}

	info := CIDRInfo{
		Prefix:     p,
		PrefixLen:  bits,
		Network:    addr,
		Broadcast:  lastAddr(p),
		TotalAddrs: new(big.Int).Lsh(big.NewInt(1), uint(totalBits-bits)),
	}

	if addr.Is4() {
		info.Netmask = netmaskV4(bits)
		info.Wildcard = wildcardV4(bits)
		// /31 (RFC 3021) 和 /32：全部地址都算 usable host
		if info.TotalAddrs.Cmp(big.NewInt(2)) > 0 {
			info.FirstHost = info.Network.Next()
			info.LastHost = info.Broadcast.Prev()
			info.UsableAddrs = new(big.Int).Sub(info.TotalAddrs, big.NewInt(2))
		} else {
			info.FirstHost = info.Network
			info.LastHost = info.Broadcast
			info.UsableAddrs = new(big.Int).Set(info.TotalAddrs)
		}
	} else {
		// IPv6 没有 broadcast 概念，整个 prefix 都算 host
		info.FirstHost = info.Network
		info.LastHost = info.Broadcast
		info.UsableAddrs = new(big.Int).Set(info.TotalAddrs)
	}
	return info, nil
}

// lastAddr 算 prefix 的最后一个地址。
// 算法：network 字节 OR 反掩码（host bits 全 1）。
func lastAddr(p netip.Prefix) netip.Addr {
	addr := p.Masked().Addr()
	bits := p.Bits()
	if addr.Is4() {
		b := addr.As4()
		hostBits := 32 - bits
		n := binary.BigEndian.Uint32(b[:])
		var mask uint32
		if hostBits >= 32 {
			mask = 0xFFFFFFFF
		} else {
			mask = (uint32(1) << hostBits) - 1
		}
		n |= mask
		var out [4]byte
		binary.BigEndian.PutUint32(out[:], n)
		return netip.AddrFrom4(out)
	}
	b := addr.As16()
	hostBits := 128 - bits
	// 从尾部 byte 往前 OR
	for i := 15; i >= 0 && hostBits > 0; i-- {
		if hostBits >= 8 {
			b[i] = 0xFF
			hostBits -= 8
		} else {
			b[i] |= byte((1 << hostBits) - 1)
			hostBits = 0
		}
	}
	return netip.AddrFrom16(b)
}

func netmaskV4(bits int) netip.Addr {
	var mask uint32
	if bits >= 32 {
		mask = 0xFFFFFFFF
	} else if bits <= 0 {
		mask = 0
	} else {
		mask = uint32(0xFFFFFFFF) << (32 - bits)
	}
	var b [4]byte
	binary.BigEndian.PutUint32(b[:], mask)
	return netip.AddrFrom4(b)
}

func wildcardV4(bits int) netip.Addr {
	var wild uint32
	if bits >= 32 {
		wild = 0
	} else if bits <= 0 {
		wild = 0xFFFFFFFF
	} else {
		wild = (uint32(1) << (32 - bits)) - 1
	}
	var b [4]byte
	binary.BigEndian.PutUint32(b[:], wild)
	return netip.AddrFrom4(b)
}

// cidrInfoJSON 是 JSON 输出 schema。
// big.Int 必须以 string 输出（IPv6 /0 = 2^128 远超 JSON number 范围）。
type cidrInfoJSON struct {
	Prefix      string `json:"prefix"`
	Version     int    `json:"version"`
	Network     string `json:"network"`
	Broadcast   string `json:"broadcast,omitempty"`
	FirstHost   string `json:"first_host"`
	LastHost    string `json:"last_host"`
	Netmask     string `json:"netmask,omitempty"`
	Wildcard    string `json:"wildcard,omitempty"`
	PrefixLen   int    `json:"prefix_len"`
	TotalAddrs  string `json:"total_addrs"`
	UsableAddrs string `json:"usable_addrs"`
}

// MarshalJSON 输出 stable schema（big.Int 转 string，IPv4-only 字段 omitempty）。
func (info CIDRInfo) MarshalJSON() ([]byte, error) {
	j := cidrInfoJSON{
		Prefix:      info.Prefix.String(),
		Network:     info.Network.String(),
		FirstHost:   info.FirstHost.String(),
		LastHost:    info.LastHost.String(),
		PrefixLen:   info.PrefixLen,
		TotalAddrs:  info.TotalAddrs.String(),
		UsableAddrs: info.UsableAddrs.String(),
	}
	if info.Prefix.Addr().Is4() {
		j.Version = 4
		j.Broadcast = info.Broadcast.String()
		j.Netmask = info.Netmask.String()
		j.Wildcard = info.Wildcard.String()
	} else {
		j.Version = 6
	}
	return json.Marshal(j)
}
