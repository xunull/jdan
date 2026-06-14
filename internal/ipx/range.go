package ipx

import (
	"errors"
	"fmt"
	"math/big"
	"net/netip"
)

// RangeResult 是 Range 的输出。
type RangeResult struct {
	Addrs    []netip.Addr `json:"addrs"`
	Total    *big.Int     `json:"-"`
	Returned int          `json:"returned"`
}

// Range 列出 prefix 内的前 limit 个地址。
//   - limit > 0: 返回最多 limit 个
//   - limit == 0: 列全部（仅当总数 <= 2^31 时，否则返回 ErrTooLarge）
//   - 防御性：硬上限 1<<24（约 1600 万）防止 OOM；超过返回错误让 CLI 提示用 --limit
func Range(p netip.Prefix, limit int) (*RangeResult, error) {
	if !p.IsValid() {
		return nil, errors.New("invalid prefix")
	}
	info, err := ComputeCIDR(p)
	if err != nil {
		return nil, err
	}
	total := info.TotalAddrs

	var want int
	switch {
	case limit > 0:
		want = limit
		if big.NewInt(int64(limit)).Cmp(total) > 0 {
			want = int(total.Int64())
		}
	case limit == 0:
		const hardCap = 1 << 20 // 1M，防 OOM（足够 /12 IPv4 即可）
		if !total.IsInt64() || total.Int64() > hardCap {
			return nil, fmt.Errorf("CIDR too large to enumerate (%s addresses, max %d); use --limit N", total.String(), hardCap)
		}
		want = int(total.Int64())
	default:
		return nil, errors.New("limit must be >= 0")
	}

	out := make([]netip.Addr, 0, want)
	cur := info.Network
	for i := 0; i < want; i++ {
		out = append(out, cur)
		cur = cur.Next()
		if !cur.IsValid() {
			break
		}
	}
	return &RangeResult{
		Addrs:    out,
		Total:    total,
		Returned: len(out),
	}, nil
}
