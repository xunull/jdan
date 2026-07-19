// Package diskx 汇报磁盘使用（df 式）。采集层按平台分文件（syscall，0 新依赖），
// 过滤/格式化/渲染/染色全是纯函数，便于用注入的 []Mount 测试。
package diskx

import (
	"fmt"
	"strings"

	"github.com/xunull/jdan/internal/termx"
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

// 伪文件系统：默认隐藏（-a 显示）。
var pseudoFS = map[string]bool{
	"devfs": true, "autofs": true, "tmpfs": true, "devtmpfs": true,
	"proc": true, "sysfs": true, "cgroup": true, "cgroup2": true,
	"mqueue": true, "debugfs": true, "tracefs": true, "securityfs": true,
	"pstore": true, "bpf": true, "configfs": true, "fusectl": true,
	"hugetlbfs": true, "ramfs": true, "binfmt_misc": true, "nsfs": true,
	"devpts": true, "fdescfs": true, "kernfs": true,
}

// Filter 默认隐藏伪文件系统、0 容量项、TimeMachine 本地快照；all=true 全留。
func Filter(mounts []Mount, all bool) []Mount {
	if all {
		return mounts
	}
	out := make([]Mount, 0, len(mounts))
	for _, m := range mounts {
		if pseudoFS[m.Fstype] || m.Total() == 0 || isLocalSnapshot(m) {
			continue
		}
		out = append(out, m)
	}
	return out
}

// isLocalSnapshot 判断是不是 macOS TimeMachine 本地快照（设备名极长、容量与
// 数据卷重复，纯噪音）。
func isLocalSnapshot(m Mount) bool {
	return strings.HasPrefix(m.Device, "com.apple.TimeMachine.") ||
		strings.HasPrefix(m.Mountpoint, "/Volumes/com.apple.TimeMachine.localsnapshots/")
}

// RenderOptions 控制表格渲染。
type RenderOptions struct {
	Inodes   bool // 显示 inode 用量而非字节
	Bytes    bool // 原始字节而非人类可读
	Color    bool // 高占用染色（仅 TTY）
	BarWidth int  // 使用率条宽度（0 → 默认 9）
	MaxWidth int  // 终端宽度；>0 时把过宽的设备/挂载点列中间省略号截断。0 = 不截断（管道/--json/--no-trunc）
}

// Render 把挂载列表渲染成对齐表格。
func Render(mounts []Mount, opt RenderOptions) string {
	width := opt.BarWidth
	if width == 0 {
		width = 9
	}
	sizeFmt := termx.HumanBytes
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
		usecol := termx.Colorize(fmt.Sprintf("%3d%%", pct), pct, opt.Color) + " " +
			termx.Colorize(termx.Bar(pct, width), pct, opt.Color)
		rows = append(rows, []string{m.Device, size, used, avail, usecol, m.Mountpoint})
	}
	if opt.MaxWidth > 0 {
		truncateColumns(header, rows, opt.MaxWidth)
	}
	return termx.Table(header, rows, rightAlignCol)
}

const (
	colDevice = 0
	colMount  = 5
	minColW   = 12 // 截断后列的最小宽度，太窄就没意义
)

// truncateColumns 在表格自然宽度超过 maxWidth 时，把设备(0)和挂载点(5)两列
// 中间省略号截断（数字列不动）。设备名通常短、优先全显，余下宽度给挂载点；
// 两列都很长时对半分。原地改写 rows。
func truncateColumns(header []string, rows [][]string, maxWidth int) {
	cols := len(header)
	nat := make([]int, cols)
	for c := range cols {
		nat[c] = termx.VisWidth(header[c])
	}
	for _, r := range rows {
		for c := range cols {
			nat[c] = max(nat[c], termx.VisWidth(r[c]))
		}
	}

	total := 2 * (cols - 1) // 列间分隔
	for _, w := range nat {
		total += w
	}
	if total <= maxWidth {
		return // 放得下，不截断
	}

	fixed := 0
	for c := range cols {
		if c != colDevice && c != colMount {
			fixed += nat[c]
		}
	}
	budget := maxWidth - fixed - 2*(cols-1)
	budget = max(budget, 2*minColW) // 终端极窄时优雅退化

	n0, n5 := nat[colDevice], nat[colMount]
	var w0, w5 int
	switch {
	case n0+n5 <= budget:
		w0, w5 = n0, n5
	case n0 <= budget-minColW:
		w0, w5 = n0, budget-n0 // 设备全显，剩下给挂载点
	default:
		w0 = max(minColW, budget/2)
		w5 = budget - w0
	}

	for _, r := range rows {
		r[colDevice] = termx.TruncMiddle(r[colDevice], w0)
		r[colMount] = termx.TruncMiddle(r[colMount], w5)
	}
}

// 数值列右对齐（容量/已用/可用），其余左对齐。
var rightAlignCol = map[int]bool{1: true, 2: true, 3: true}

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
