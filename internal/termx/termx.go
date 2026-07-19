// Package termx 是终端渲染的共享纯函数层：字节格式化、使用率条、染色、
// 可见宽度测量、中间省略截断、对齐表格。
//
// 从 diskx 抽出来共享，而不是让每个命令各写一份。理由在 visWidth：CJK locale
// 下 runewidth 默认把 █ U+2588 判成 2 列（ambiguous→wide），与终端实际渲染
// （1 列）不符，不锁死就会整列错位。重写必然踩同一个坑，见 TestVisWidth_
// AmbiguousBlocksNarrowUnderCJK。
package termx

import (
	"fmt"
	"regexp"
	"strings"

	"github.com/mattn/go-runewidth"
)

// HumanBytes 把字节数转成 1024 进制人类可读，≥10 时只保留整数位
// （贴 df -h：1.0Ki / 14Gi / 1.5Ti）。
func HumanBytes(n uint64) string { return humanBytes(n, false) }

// HumanBytes1 与 HumanBytes 同进制同后缀，但始终保留一位小数
// （86.3Gi / 31.2Gi）。排行榜场景需要这个分辨力：HumanBytes 会把
// 31.2Gi 和 24.8Gi 压成 31Gi 和 25Gi，相邻项就看不出差距了。
func HumanBytes1(n uint64) string { return humanBytes(n, true) }

func humanBytes(n uint64, precise bool) string {
	const unit = 1024
	if n < unit {
		return fmt.Sprintf("%dB", n)
	}
	units := []string{"Ki", "Mi", "Gi", "Ti", "Pi", "Ei"}
	v := float64(n)
	i := -1
	for v >= unit && i < len(units)-1 {
		v /= unit
		i++
	}
	if precise || v < 10 {
		return fmt.Sprintf("%.1f%s", v, units[i])
	}
	return fmt.Sprintf("%.0f%s", v, units[i])
}

// Comma 给大数字加千位分隔符（412883 → 412,883）。
func Comma(n uint64) string {
	s := fmt.Sprintf("%d", n)
	if len(s) <= 3 {
		return s
	}
	var sb strings.Builder
	lead := len(s) % 3
	if lead > 0 {
		sb.WriteString(s[:lead])
	}
	for i := lead; i < len(s); i += 3 {
		if sb.Len() > 0 {
			sb.WriteByte(',')
		}
		sb.WriteString(s[i : i+3])
	}
	return sb.String()
}

// Bar 渲染 width 宽的使用率条，pct 四舍五入到格。
func Bar(pct, width int) string {
	pct = min(max(pct, 0), 100)
	filled := (pct*width + 50) / 100
	return strings.Repeat("█", filled) + strings.Repeat("░", width-filled)
}

// Colorize 按占用率染色：≥90% 红，≥75% 黄，其余不染。color=false 原样返回。
func Colorize(s string, pct int, color bool) string {
	if !color {
		return s
	}
	switch {
	case pct >= 90:
		return "\x1b[31m" + s + "\x1b[0m" // 红
	case pct >= 75:
		return "\x1b[33m" + s + "\x1b[0m" // 黄
	default:
		return s
	}
}

// TruncMiddle 把字符串中间省略号截断到可见宽度 maxW（两头都保留信息）。
func TruncMiddle(s string, maxW int) string {
	if maxW <= 0 {
		return ""
	}
	if VisWidth(s) <= maxW {
		return s
	}
	if maxW == 1 {
		return "…"
	}
	budget := maxW - 1 // 给省略号留 1 列
	head := budget - budget/2
	tail := budget / 2
	return takeHead(s, head) + "…" + takeTail(s, tail)
}

func takeHead(s string, maxW int) string {
	w := 0
	var b strings.Builder
	for _, r := range s {
		rw := narrowWidth.RuneWidth(r)
		if w+rw > maxW {
			break
		}
		b.WriteRune(r)
		w += rw
	}
	return b.String()
}

func takeTail(s string, maxW int) string {
	runes := []rune(s)
	w := 0
	i := len(runes)
	for i > 0 {
		rw := narrowWidth.RuneWidth(runes[i-1])
		if w+rw > maxW {
			break
		}
		w += rw
		i--
	}
	return string(runes[i:])
}

// Table 把 header + rows 渲染成对齐表格。rightAlign 指定哪些列号右对齐
// （数值列），nil 表示全部左对齐。
//
// header 为 nil 时不渲染表头行，列宽只由 rows 决定 —— 排行榜那种「每行
// 都是数据、没有列名」的版式需要这个。传 []string{"", ""} 是不行的：那会
// 渲染出一个空行。
func Table(header []string, rows [][]string, rightAlign map[int]bool) string {
	cols := len(header)
	if cols == 0 {
		for _, r := range rows {
			cols = max(cols, len(r))
		}
	}
	if cols == 0 {
		return ""
	}
	widths := make([]int, cols)
	for c := range header {
		widths[c] = VisWidth(header[c])
	}
	for _, r := range rows {
		for c := range cols {
			if c < len(r) {
				widths[c] = max(widths[c], VisWidth(r[c]))
			}
		}
	}
	var sb strings.Builder
	writeRow := func(cells []string) {
		parts := make([]string, cols)
		for c := range cols {
			cell := ""
			if c < len(cells) {
				cell = cells[c]
			}
			parts[c] = Pad(cell, widths[c], rightAlign[c])
		}
		sb.WriteString(strings.TrimRight(strings.Join(parts, "  "), " "))
		sb.WriteByte('\n')
	}
	if header != nil {
		writeRow(header)
	}
	for _, r := range rows {
		writeRow(r)
	}
	return sb.String()
}

// Pad 把 s 填充到可见宽度 width；right=true 右对齐。
func Pad(s string, width int, right bool) string {
	gap := width - VisWidth(s)
	if gap <= 0 {
		return s
	}
	if right {
		return strings.Repeat(" ", gap) + s
	}
	return s + strings.Repeat(" ", gap)
}

var ansiRe = regexp.MustCompile(`\x1b\[[0-9;]*m`)

// narrowWidth 强制 East-Asian-ambiguous 字符按 1 列算（使用率条的 █ U+2588 在 CJK
// locale 下会被默认判成 2 列，但终端实际渲染成 1 列，不锁死就会整列错位）。
// 真正的宽字符（中文表头「容量」等是 East_Asian_Width=W）不受影响，仍按 2 列算。
var narrowWidth = &runewidth.Condition{EastAsianWidth: false}

// VisWidth 返回 s 去掉 ANSI 转义后的终端可见宽度。
func VisWidth(s string) int { return narrowWidth.StringWidth(ansiRe.ReplaceAllString(s, "")) }
