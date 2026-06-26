// Package entropyx 算输入的 Shannon 熵（字节分布的信息量，bits/byte，0–8）。
// 用于判断数据是否加密/压缩/随机、估可压缩性。可选滑窗 sparkline 看高熵区段，
// 可选字符集搜索空间 bits 估算（明确非密码强度评分）。纯 stdlib math。
package entropyx

import (
	"encoding/json"
	"fmt"
	"math"
	"strings"
)

// Result 是一次熵分析结果。
type Result struct {
	Bytes       int
	Distinct    int
	BitsPerByte float64
	TotalBits   float64
	Label       string
	Window      int       // 0 表示未分窗
	Chunks      []float64 // 每个窗口的熵
	PeakValue   float64
	PeakOffset  int
	Charset     int     // 字符集大小（0 表示未算）
	CharsetBits float64 // 搜索空间 bits（长度 × log2(字符集)）
}

// Shannon 返回 data 的 Shannon 熵，单位 bits/byte（0–8）。
func Shannon(data []byte) float64 {
	if len(data) == 0 {
		return 0
	}
	var counts [256]int
	for _, b := range data {
		counts[b]++
	}
	n := float64(len(data))
	h := 0.0
	for _, c := range counts {
		if c == 0 {
			continue
		}
		p := float64(c) / n
		h -= p * math.Log2(p)
	}
	return h
}

func distinctBytes(data []byte) int {
	var seen [256]bool
	n := 0
	for _, b := range data {
		if !seen[b] {
			seen[b] = true
			n++
		}
	}
	return n
}

func label(h float64) string {
	switch {
	case h < 1:
		return "极低：高度重复"
	case h < 4:
		return "低：文本/结构化"
	case h < 6:
		return "中"
	case h < 7.5:
		return "高"
	default:
		return "极高：疑似加密/压缩/随机"
	}
}

// Analyze 算整体熵；window>0 时再按窗口切块逐块算熵 + 记峰值。
func Analyze(data []byte, window int) Result {
	h := Shannon(data)
	r := Result{
		Bytes:       len(data),
		Distinct:    distinctBytes(data),
		BitsPerByte: h,
		TotalBits:   h * float64(len(data)),
		Label:       label(h),
		Window:      window,
	}
	if window > 0 && len(data) > 0 {
		for off := 0; off < len(data); off += window {
			end := min(off+window, len(data))
			ch := Shannon(data[off:end])
			r.Chunks = append(r.Chunks, ch)
			if ch > r.PeakValue {
				r.PeakValue = ch
				r.PeakOffset = off
			}
		}
	}
	return r
}

var sparkBlocks = []rune("▁▂▃▄▅▆▇█")

// Sparkline 把每块熵（0–8）映射成块字符。
func Sparkline(chunks []float64) string {
	var b strings.Builder
	for _, c := range chunks {
		idx := int(c / 8 * float64(len(sparkBlocks)-1))
		idx = max(idx, 0)
		idx = min(idx, len(sparkBlocks)-1)
		b.WriteRune(sparkBlocks[idx])
	}
	return b.String()
}

// CharsetBits 按字符类别估算搜索空间：长度 × log2(字符集大小)。
// 这是「理论搜索空间」，不是密码强度评分（强度要字典/模式检查）。
func CharsetBits(s string) (int, float64) {
	var lower, upper, digit, space, sym bool
	for _, r := range s {
		switch {
		case r >= 'a' && r <= 'z':
			lower = true
		case r >= 'A' && r <= 'Z':
			upper = true
		case r >= '0' && r <= '9':
			digit = true
		case r == ' ':
			space = true
		default:
			sym = true
		}
	}
	size := 0
	if lower {
		size += 26
	}
	if upper {
		size += 26
	}
	if digit {
		size += 10
	}
	if sym {
		size += 32 // 约可打印符号数
	}
	if space {
		size++
	}
	if size == 0 {
		return 0, 0
	}
	n := len([]rune(s))
	return size, float64(n) * math.Log2(float64(size))
}

// FormatText 渲染成文本。
func (r Result) FormatText(showCharset bool) string {
	var b strings.Builder
	fmt.Fprintf(&b, "bytes:    %d\n", r.Bytes)
	fmt.Fprintf(&b, "entropy:  %.2f bits/byte   (%s)\n", r.BitsPerByte, r.Label)
	fmt.Fprintf(&b, "total:    %.1f bits\n", r.TotalBits)
	fmt.Fprintf(&b, "distinct: %d / 256 字节值\n", r.Distinct)
	if len(r.Chunks) > 0 {
		fmt.Fprintf(&b, "\nsparkline (窗口 %dB):\n%s\n", r.Window, Sparkline(r.Chunks))
		fmt.Fprintf(&b, "峰值 %.2f @ 偏移 0x%X\n", r.PeakValue, r.PeakOffset)
	}
	if showCharset && r.Charset > 0 {
		fmt.Fprintf(&b, "charset:  %d 符号集 ≈ %.1f bits（搜索空间，非强度评分）\n", r.Charset, r.CharsetBits)
	}
	return b.String()
}

// FormatJSON 渲染成结构化输出。
func (r Result) FormatJSON() (string, error) {
	out := map[string]any{
		"bytes":         r.Bytes,
		"distinct":      r.Distinct,
		"bits_per_byte": r.BitsPerByte,
		"total_bits":    r.TotalBits,
		"label":         r.Label,
	}
	if r.Window > 0 {
		out["window"] = r.Window
		out["chunks"] = r.Chunks
		out["peak_value"] = r.PeakValue
		out["peak_offset"] = r.PeakOffset
	}
	if r.Charset > 0 {
		out["charset_size"] = r.Charset
		out["charset_bits"] = r.CharsetBits
	}
	data, err := json.MarshalIndent(out, "", "  ")
	if err != nil {
		return "", err
	}
	return string(data), nil
}
