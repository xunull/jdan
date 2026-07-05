package ipx

import (
	"errors"
	"fmt"
	"math/big"
	"net/netip"
	"sort"
)

// RangeToCIDRs 把任意闭区间 [start, end]（含端点、不必对齐边界）分解成最小数量的
// CIDR 集合。start/end 必须同族且 start <= end。经典「iprange → cidr」算法：每步
// 取从 start 起、既满足对齐、又不越过 end 的最大 2^k 块。
//
// 例：192.168.1.5 .. 192.168.1.20 →
//
//	192.168.1.5/32, 192.168.1.6/31, 192.168.1.8/29, 192.168.1.16/30, 192.168.1.20/32
func RangeToCIDRs(start, end netip.Addr) ([]netip.Prefix, error) {
	start = start.Unmap()
	end = end.Unmap()
	if !start.IsValid() || !end.IsValid() {
		return nil, errors.New("invalid address")
	}
	if start.Is4() != end.Is4() {
		return nil, errors.New("address family mismatch (mix of IPv4 and IPv6)")
	}
	bits := 32
	if start.Is6() {
		bits = 128
	}
	lo := addrToInt(start)
	hi := addrToInt(end)
	if lo.Cmp(hi) > 0 {
		return nil, errors.New("start address is greater than end address")
	}

	var out []netip.Prefix
	one := big.NewInt(1)
	cur := new(big.Int).Set(lo)
	for cur.Cmp(hi) <= 0 {
		// 剩余量 span = hi - cur + 1，最大可用块 = 2^(span.BitLen()-1)
		span := new(big.Int).Sub(hi, cur)
		span.Add(span, one)
		maxBySpan := span.BitLen() - 1
		// 对齐：cur 尾部 0 位数（cur==0 不受限）
		maxByAlign := bits
		if cur.Sign() != 0 {
			maxByAlign = int(cur.TrailingZeroBits())
		}
		hostBits := min(maxByAlign, maxBySpan)
		prefixLen := bits - hostBits
		p, err := intToAddr(cur, start.Is4()).Prefix(prefixLen)
		if err != nil {
			return nil, err
		}
		out = append(out, p)
		cur = new(big.Int).Add(cur, new(big.Int).Lsh(one, uint(hostBits)))
	}
	return out, nil
}

// Aggregate 把一组 prefix（含 /32、/128 单地址）合并成最小的 CIDR 覆盖集：重叠或
// 相邻的网段被并起来。IPv4 与 IPv6 各自聚合，结果先 IPv4 后 IPv6、按地址升序。
//
// 例：10.0.0.0/25 + 10.0.0.128/25 → 10.0.0.0/24
func Aggregate(prefixes []netip.Prefix) ([]netip.Prefix, error) {
	if len(prefixes) == 0 {
		return nil, errors.New("no prefixes given")
	}
	type rng struct{ lo, hi *big.Int }
	var v4, v6 []rng
	for _, p := range prefixes {
		if !p.IsValid() {
			return nil, fmt.Errorf("invalid prefix: %s", p)
		}
		p = p.Masked()
		r := rng{lo: addrToInt(p.Addr()), hi: addrToInt(lastAddr(p))}
		if p.Addr().Is4() {
			v4 = append(v4, r)
		} else {
			v6 = append(v6, r)
		}
	}

	var out []netip.Prefix
	for _, group := range []struct {
		rs  []rng
		is4 bool
	}{{v4, true}, {v6, false}} {
		rs := group.rs
		if len(rs) == 0 {
			continue
		}
		sort.Slice(rs, func(i, j int) bool { return rs[i].lo.Cmp(rs[j].lo) < 0 })
		// 合并重叠/相邻区间：next.lo <= cur.hi + 1
		merged := []rng{rs[0]}
		one := big.NewInt(1)
		for _, r := range rs[1:] {
			last := &merged[len(merged)-1]
			adj := new(big.Int).Add(last.hi, one)
			if r.lo.Cmp(adj) <= 0 { // 重叠或相邻
				if r.hi.Cmp(last.hi) > 0 {
					last.hi = r.hi
				}
			} else {
				merged = append(merged, r)
			}
		}
		for _, r := range merged {
			cidrs, err := RangeToCIDRs(intToAddr(r.lo, group.is4), intToAddr(r.hi, group.is4))
			if err != nil {
				return nil, err
			}
			out = append(out, cidrs...)
		}
	}
	return out, nil
}

// addrToInt 把地址转成大整数（4 或 16 字节大端）。
func addrToInt(a netip.Addr) *big.Int {
	if a.Is4() {
		b := a.As4()
		return new(big.Int).SetBytes(b[:])
	}
	b := a.As16()
	return new(big.Int).SetBytes(b[:])
}

// intToAddr 把大整数还原成地址（is4 决定 4 / 16 字节，左侧补零）。
func intToAddr(n *big.Int, is4 bool) netip.Addr {
	if is4 {
		var b [4]byte
		n.FillBytes(b[:])
		return netip.AddrFrom4(b)
	}
	var b [16]byte
	n.FillBytes(b[:])
	return netip.AddrFrom16(b)
}
