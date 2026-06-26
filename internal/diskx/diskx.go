// Package diskx 汇报磁盘使用（df 式）。采集层按平台分文件（syscall，0 新依赖），
// 过滤/格式化/渲染/染色全是纯函数，便于用注入的 []Mount 测试。
package diskx

import (
	"fmt"
	"regexp"
	"strings"

	"github.com/mattn/go-runewidth"
)

// Mount 是一个挂载点的容量信息（块为单位，乘 BlockSize 得字节）。
type Mount struct {
	Device     string
	Mountpoint string
	Fstype     string
	BlockSize  uint64
	Blocks     uint64 // 总块数
	Bfree      uint64 // 空闲块（含 root 保留）
	Bavail     uint64 // 非 root 可用块
	Files      uint64 // 总 inode 数
	Ffree      uint64 // 空闲 inode 数
}

// Total 返回总字节数。
func (m Mount) Total() uint64 { return m.Blocks * m.BlockSize }

// Used 返回已用字节数。
func (m Mount) Used() uint64 { return (m.Blocks - m.Bfree) * m.BlockSize }

// Avail 返回非 root 可用字节数。
func (m Mount) Avail() uint64 { return m.Bavail * m.BlockSize }

// UsePercent 对齐 df：已用 / (已用 + 可用)，向上取整。
func (m Mount) UsePercent() int {
	used := m.Blocks - m.Bfree
	denom := used + m.Bavail
	if denom == 0 {
		return 0
	}
	return int((used*100 + denom - 1) / denom)
}

// InodePercent 对齐 df -i：已用 inode / 总 inode，向上取整。
func (m Mount) InodePercent() int {
	if m.Files == 0 {
		return 0
	}
	used := m.Files - m.Ffree
	return int((used*100 + m.Files - 1) / m.Files)
}

// HumanBytes 把字节数转成 1024 进制人类可读（贴 df -h：1.0Ki / 14Gi / 1.5Ti）。
func HumanBytes(n uint64) string {
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
	if v < 10 {
		return fmt.Sprintf("%.1f%s", v, units[i])
	}
	return fmt.Sprintf("%.0f%s", v, units[i])
}

// 伪文件系统：默认隐藏（-a 显示）。
var pseudoFS = map[string]bool{
	"devfs": true, "autofs": true, "tmpfs": true, "devtmpfs": true,
	"proc": true, "sysfs": true, "cgroup": true, "cgroup2": true,
	"mqueue": true, "debugfs": true, "tracefs": true, "securityfs": true,
	"pstore": true, "bpf": true, "configfs": true, "fusectl": true,
	"hugetlbfs": true, "ramfs": true, "binfmt_misc": true, "nsfs": true,
	"devpts": true, "fdescfs": true, "kernfs": true,
}

// Filter 默认隐藏伪文件系统和 0 容量项；all=true 全留。
func Filter(mounts []Mount, all bool) []Mount {
	if all {
		return mounts
	}
	out := make([]Mount, 0, len(mounts))
	for _, m := range mounts {
		if pseudoFS[m.Fstype] || m.Total() == 0 {
			continue
		}
		out = append(out, m)
	}
	return out
}

// RenderOptions 控制表格渲染。
type RenderOptions struct {
	Inodes   bool // 显示 inode 用量而非字节
	Bytes    bool // 原始字节而非人类可读
	Color    bool // 高占用染色（仅 TTY）
	BarWidth int  // 使用率条宽度（0 → 默认 9）
}

// Render 把挂载列表渲染成对齐表格。
func Render(mounts []Mount, opt RenderOptions) string {
	width := opt.BarWidth
	if width == 0 {
		width = 9
	}
	sizeFmt := HumanBytes
	if opt.Bytes {
		sizeFmt = func(n uint64) string { return fmt.Sprintf("%d", n) }
	}

	var header []string
	if opt.Inodes {
		header = []string{"文件系统", "Inode", "已用", "可用", "使用率", "挂载点"}
	} else {
		header = []string{"文件系统", "容量", "已用", "可用", "使用率", "挂载点"}
	}

	rows := make([][]string, 0, len(mounts))
	for _, m := range mounts {
		var size, used, avail string
		var pct int
		if opt.Inodes {
			size = fmt.Sprintf("%d", m.Files)
			used = fmt.Sprintf("%d", m.Files-m.Ffree)
			avail = fmt.Sprintf("%d", m.Ffree)
			pct = m.InodePercent()
		} else {
			size = sizeFmt(m.Total())
			used = sizeFmt(m.Used())
			avail = sizeFmt(m.Avail())
			pct = m.UsePercent()
		}
		// 百分比右对齐到固定 4 宽（"  4%" / " 86%" / "100%"），条形左缘才能对齐成竖列。
		usecol := colorize(fmt.Sprintf("%3d%%", pct), pct, opt.Color) + " " +
			colorize(bar(pct, width), pct, opt.Color)
		rows = append(rows, []string{m.Device, size, used, avail, usecol, m.Mountpoint})
	}
	return table(header, rows)
}

func bar(pct, width int) string {
	pct = min(max(pct, 0), 100)
	filled := (pct*width + 50) / 100
	return strings.Repeat("█", filled) + strings.Repeat("░", width-filled)
}

func colorize(s string, pct int, color bool) string {
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

// 数值列右对齐（容量/已用/可用），其余左对齐。
var rightAlignCol = map[int]bool{1: true, 2: true, 3: true}

func table(header []string, rows [][]string) string {
	cols := len(header)
	widths := make([]int, cols)
	for c := range cols {
		widths[c] = visWidth(header[c])
	}
	for _, r := range rows {
		for c := range cols {
			widths[c] = max(widths[c], visWidth(r[c]))
		}
	}
	var sb strings.Builder
	writeRow := func(cells []string) {
		parts := make([]string, cols)
		for c := range cols {
			parts[c] = pad(cells[c], widths[c], rightAlignCol[c])
		}
		sb.WriteString(strings.TrimRight(strings.Join(parts, "  "), " "))
		sb.WriteByte('\n')
	}
	writeRow(header)
	for _, r := range rows {
		writeRow(r)
	}
	return sb.String()
}

func pad(s string, width int, right bool) string {
	gap := width - visWidth(s)
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

func visWidth(s string) int { return narrowWidth.StringWidth(ansiRe.ReplaceAllString(s, "")) }

// JSONData 返回适合 JSON 序列化的结构。
func JSONData(mounts []Mount) []map[string]any {
	out := make([]map[string]any, 0, len(mounts))
	for _, m := range mounts {
		out = append(out, map[string]any{
			"device":         m.Device,
			"mountpoint":     m.Mountpoint,
			"fstype":         m.Fstype,
			"size":           m.Total(),
			"used":           m.Used(),
			"avail":          m.Avail(),
			"use_percent":    m.UsePercent(),
			"inodes":         m.Files,
			"inodes_used":    m.Files - m.Ffree,
			"inodes_percent": m.InodePercent(),
		})
	}
	return out
}
