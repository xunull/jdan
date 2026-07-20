// Package wifix 做 WiFi 信道干扰分析：频段判定、信道重叠、带宽展开、
// 以及同信道/邻信道两种干扰的量化。
//
// 这一层全是纯函数，不碰 IO。它也是整个功能里唯一「算错了看起来还很合理」
// 的地方 —— 带宽展开错了会得到一组完全错误但形状正常的信道集合，加权错了
// 会给出物理上不成立却排得出名次的建议。所以规则都写成查表和显式公式，
// 不用「往后数 N 个」这类看着对的算术。
package wifix

import (
	"fmt"
	"slices"
)

// Band 是频段。必须按 macOS 报告的 band 字符串判定，不能按信道号推断 ——
// 6GHz 的信道号也从 1 开始，与 2.4GHz 直接碰撞。
type Band int

const (
	BandUnknown Band = iota
	Band24
	Band5
	Band6
)

func (b Band) String() string {
	switch b {
	case Band24:
		return "2.4GHz"
	case Band5:
		return "5GHz"
	case Band6:
		return "6GHz"
	default:
		return "未知"
	}
}

// ParseBand 把 macOS 的 band 字符串（"2GHz" / "5GHz" / "6GHz"）转成 Band。
func ParseBand(s string) Band {
	switch s {
	case "2GHz", "2.4GHz":
		return Band24
	case "5GHz":
		return Band5
	case "6GHz":
		return Band6
	default:
		return BandUnknown
	}
}

// CenterFreqMHz 返回信道中心频率。
//
// 2.4GHz：2407 + 5n，但 **信道 14 是例外**（2484 MHz，不是 2477）。14 是
// 日本专用且仅 802.11b DSSS。不特判的话 JP locale 的 Mac 会算错 7MHz。
// 5GHz：5000 + 5n。
// 6GHz：5950 + 5n（802.11ax 6E 的 UNII-5~8）。
func CenterFreqMHz(b Band, ch int) (int, error) {
	switch b {
	case Band24:
		if ch == 14 {
			return 2484, nil
		}
		if ch < 1 || ch > 13 {
			return 0, fmt.Errorf("2.4GHz 信道 %d 超出 1-14", ch)
		}
		return 2407 + 5*ch, nil
	case Band5:
		if ch < 1 || ch > 196 {
			return 0, fmt.Errorf("5GHz 信道 %d 超出范围", ch)
		}
		return 5000 + 5*ch, nil
	case Band6:
		if ch < 1 || ch > 233 {
			return 0, fmt.Errorf("6GHz 信道 %d 超出范围", ch)
		}
		return 5950 + 5*ch, nil
	}
	return 0, fmt.Errorf("未知频段")
}

// ---- 5GHz / 6GHz 对齐块 ----
//
// 802.11 的 40/80/160MHz 信道位于**固定栅格**上，不能任意起始。macOS 报告的
// 是 primary channel，不是块起始 —— 实测扫描里出现过 "44 (5GHz, 160MHz)"。
//
// 所以展开规则必须是「在该宽度的合法块表里找包含 primary 的那一块」，
// 而不是「primary 往后数 N 个」。后者对 44@80MHz 会给出 {44,48,52,56}，
// 正确答案是 {36,40,44,48} —— 错得完全，但看起来很合理。

var blocks5GHz = map[int][][]int{
	20: {
		{36}, {40}, {44}, {48}, {52}, {56}, {60}, {64},
		{100}, {104}, {108}, {112}, {116}, {120}, {124}, {128},
		{132}, {136}, {140}, {144},
		{149}, {153}, {157}, {161}, {165},
	},
	40: {
		{36, 40}, {44, 48}, {52, 56}, {60, 64},
		{100, 104}, {108, 112}, {116, 120}, {124, 128},
		{132, 136}, {140, 144},
		{149, 153}, {157, 161},
	},
	80: {
		{36, 40, 44, 48}, {52, 56, 60, 64},
		{100, 104, 108, 112}, {116, 120, 124, 128},
		{132, 136, 140, 144},
		{149, 153, 157, 161},
	},
	// 全世界只有这两个合法的 160MHz 块。
	160: {
		{36, 40, 44, 48, 52, 56, 60, 64},
		{100, 104, 108, 112, 116, 120, 124, 128},
	},
}

// Expand 返回该 AP 实际占用的 20MHz 子信道集合。
//
// 这是整个模型的地基：后面所有重叠计算都基于「两个 AP 的子信道集合有多少
// 交集」，而不是信道号的算术关系。
//
// 边界情况：
//   - 165 是孤立的 20MHz-only 信道，不参与任何 40/80/160 组合
//   - 144 与 149 之间有 25MHz 空隙（5720 vs 5745），不能跨断档做算术
//   - 找不到合法块时降级为 {ch} 而非报错 —— 宁可低估干扰也不要崩
func Expand(b Band, ch, widthMHz int) []int {
	switch b {
	case Band24:
		// 2.4GHz 的重叠是连续的部分重叠，不用子信道集合建模，
		// 由 Overlap24 处理。这里返回单信道。
		return []int{ch}
	case Band5, Band6:
		if widthMHz <= 20 {
			return []int{ch}
		}
		for _, blk := range blocks5GHz[widthMHz] {
			if slices.Contains(blk, ch) {
				out := make([]int, len(blk))
				copy(out, blk)
				return out
			}
		}
		// 未知宽度或不在任何合法块里（如 165@80）→ 只算自己。
		return []int{ch}
	}
	return []int{ch}
}

// CanHostWidth 判断该信道能否承载指定带宽。
//
// 5GHz/6GHz 的宽信道在固定栅格上，不是每个信道都能当任意宽度的 primary：
// ch165 是孤立的 20MHz-only 信道，ch144 之后有 25MHz 断档。把这类信道当成
// 候选会给出「换到 165」这种建议 —— 用户当前跑 80MHz，换过去带宽掉 4 倍，
// 而它显得空恰恰因为它只占一个 20MHz 信道。
//
// 2.4GHz 走连续重叠模型，Expand 恒返回单信道，这里一律放行。
func CanHostWidth(b Band, ch, widthMHz int) bool {
	if b == Band24 {
		return true
	}
	if widthMHz <= 20 {
		return true
	}
	want := widthMHz / 20
	return len(Expand(b, ch, widthMHz)) == want
}

// ---- 2.4GHz 重叠 ----

// Overlap24 返回两个 2.4GHz 信道的重叠度 [0,1]。
//
// 信道间隔只有 5MHz，而 20MHz 标称带宽的实际占用更宽：DSSS 约 22MHz，
// OFDM 的频谱模板在 ±11MHz 处才降到 -20dBr。所以有效占用按 ~25MHz 算：
//
//	重叠 ⟺ 5·|n-m| < 25 ⟺ |n-m| ≤ 4
//	重叠度 = (5 - |n-m|) / 5
//
// 这正是 1/6/11 相隔 5 的由来。
//
// 注意 1/5/9/13 相隔 4，按这个规则是**弱重叠（0.2）**而非完全不重叠 ——
// 它是「用轻微邻道干扰换一个额外可用信道」的权衡方案，不是干净方案。
func Overlap24(a, b int) float64 {
	d := a - b
	if d < 0 {
		d = -d
	}
	if d >= 5 {
		return 0
	}
	return float64(5-d) / 5
}

// ---- 跨频段/跨宽度重叠 ----

// AP 是一个被扫描到的接入点，只保留干扰计算需要的字段。
type AP struct {
	Band     Band
	Channel  int // primary channel
	WidthMHz int
	RSSI     int // dBm，负数
}

// Overlap 返回邻居 n 对本机 self 的重叠度 [0,1]。
//
// 5GHz/6GHz：把两者都展开成 20MHz 子信道集合，重叠度 = 交集大小 / **本机**
// 集合大小。分母用本机而非邻居，因为我们问的是「本机的频谱有多少被占」。
//
//	本机 36@80  → {36,40,44,48}
//	邻居 44@20  → {44}      → 交集 1/4 = 0.25
//	邻居 44@160 → {36..64}  → 交集 4/4 = 1.0
//
// 2.4GHz：走 Overlap24 的连续重叠模型。
//
// 跨频段一律返回 0。
func Overlap(self, n AP) float64 {
	if self.Band != n.Band || self.Band == BandUnknown {
		return 0
	}
	if self.Band == Band24 {
		return Overlap24(self.Channel, n.Channel)
	}

	mine := Expand(self.Band, self.Channel, self.WidthMHz)
	theirs := Expand(n.Band, n.Channel, n.WidthMHz)
	if len(mine) == 0 {
		return 0
	}
	set := make(map[int]bool, len(theirs))
	for _, c := range theirs {
		set[c] = true
	}
	hit := 0
	for _, c := range mine {
		if set[c] {
			hit++
		}
	}
	return float64(hit) / float64(len(mine))
}
