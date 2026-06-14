package ipx

import (
	"errors"
	"fmt"
	"math/big"
	"net/netip"
)

// maxSplitCount 是子网划分的硬上限，防止用户不小心 split /8 → /24 产生 65536+ 输出。
const maxSplitCount = 1 << 16

// Split 把 parent prefix 按 newBits 长度切分。
// 例：Split(10.0.0.0/22, 24) → [10.0.0.0/24, 10.0.1.0/24, 10.0.2.0/24, 10.0.3.0/24]
//
//   - newBits >= parent.Bits() 且 <= maxBits(IPv4=32, IPv6=128)
//   - 数量 = 2^(newBits - parent.Bits())，硬上限 maxSplitCount 防止 OOM
func Split(parent netip.Prefix, newBits int) ([]netip.Prefix, error) {
	if !parent.IsValid() {
		return nil, errors.New("invalid parent prefix")
	}
	parent = parent.Masked()
	addr := parent.Addr()
	maxBits := 32
	if addr.Is6() {
		maxBits = 128
	}
	if newBits < parent.Bits() {
		return nil, fmt.Errorf("new prefix length %d must be >= parent length %d", newBits, parent.Bits())
	}
	if newBits > maxBits {
		return nil, fmt.Errorf("new prefix length %d exceeds max %d", newBits, maxBits)
	}

	delta := newBits - parent.Bits()
	count := new(big.Int).Lsh(big.NewInt(1), uint(delta))
	if !count.IsInt64() || count.Int64() > maxSplitCount {
		return nil, fmt.Errorf("too many subnets (%s); refusing for sanity (max %d)", count.String(), maxSplitCount)
	}
	n := int(count.Int64())

	out := make([]netip.Prefix, 0, n)
	cur, err := addr.Prefix(newBits)
	if err != nil {
		return nil, err
	}
	cur = cur.Masked()
	for range n {
		out = append(out, cur)
		// 下一个子网：当前的 last addr + 1 = 下一个 network base
		next := lastAddr(cur).Next()
		if !next.IsValid() {
			break
		}
		cur, err = next.Prefix(newBits)
		if err != nil {
			break
		}
		cur = cur.Masked()
	}
	return out, nil
}
