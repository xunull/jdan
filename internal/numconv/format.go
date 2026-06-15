package numconv

import (
	"fmt"
	"math/bits"
	"strconv"
	"strings"
)

// Result 是一个 uint64 的全进制 + 位信息。
type Result struct {
	Decimal  uint64 `json:"decimal"`
	Hex      string `json:"hex"`       // 带 0x 前缀，大写
	Binary   string `json:"binary"`    // 带 0b 前缀
	Octal    string `json:"octal"`     // 带 0o 前缀
	BitsSet  int    `json:"bits_set"`  // popcount（置位的 bit 数）
	BitWidth int    `json:"bit_width"` // 最高有效位位置（0 → 0，1 → 1，256 → 9）
}

// Convert 计算一个 uint64 的 Result。
func Convert(v uint64) Result {
	return Result{
		Decimal:  v,
		Hex:      "0x" + strings.ToUpper(strconv.FormatUint(v, 16)),
		Binary:   "0b" + strconv.FormatUint(v, 2),
		Octal:    "0o" + strconv.FormatUint(v, 8),
		BitsSet:  bits.OnesCount64(v),
		BitWidth: bits.Len64(v),
	}
}

// HexWidth 返回零填充到 width 位（二进制）的 hex 串，便于寄存器对齐。
// width <= 0 时不填充。
func BinaryPadded(v uint64, width int) string {
	s := strconv.FormatUint(v, 2)
	if width > len(s) {
		s = strings.Repeat("0", width-len(s)) + s
	}
	return "0b" + s
}

// HexPadded 返回零填充到 width 位（十六进制 nibble 数）的 hex 串。
func HexPadded(v uint64, width int) string {
	s := strings.ToUpper(strconv.FormatUint(v, 16))
	if width > len(s) {
		s = strings.Repeat("0", width-len(s)) + s
	}
	return "0x" + s
}

// SetBits 返回所有置位的 bit 位置（从低到高，0-based）。
func SetBits(v uint64) []int {
	var out []int
	for i := range 64 {
		if v&(uint64(1)<<i) != 0 {
			out = append(out, i)
		}
	}
	return out
}

// BitRows 渲染位展示：bit 编号行 + 值行，便于看 flag / mask。
// width 决定显示多少位（默认按 BitWidth，至少 1）。
func BitRows(v uint64, width int) string {
	if width <= 0 {
		width = bits.Len64(v)
		if width == 0 {
			width = 1
		}
	}
	if width > 64 {
		width = 64
	}
	var idxRow, valRow strings.Builder
	idxRow.WriteString("bit:  ")
	valRow.WriteString("val:  ")
	for i := width - 1; i >= 0; i-- {
		fmt.Fprintf(&idxRow, "%d ", i%10) // 个位数编号（多位时只显示个位，紧凑）
		if v&(uint64(1)<<i) != 0 {
			valRow.WriteString("1 ")
		} else {
			valRow.WriteString("0 ")
		}
	}
	return strings.TrimRight(idxRow.String(), " ") + "\n" +
		strings.TrimRight(valRow.String(), " ")
}
