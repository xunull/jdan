package sizex

import (
	"fmt"
	"math"
	"strings"
	"time"

	"github.com/xunull/jdan/internal/termx"
)

// RenderOptions 控制文本渲染。
type RenderOptions struct {
	Color    bool          // 高占比染色（仅 TTY）
	MaxWidth int           // 终端宽度；>0 时截断过长的名称。0 = 不截断
	BarWidth int           // 条形宽度（0 → 默认 17）
	Verbose  bool          // 列出每条无权访问的路径
	Elapsed  time.Duration // 扫描耗时，0 则不显示
}

const (
	defaultBarWidth = 17
	indentPerLevel  = 2
	minNameWidth    = 12
)

// Render 把树渲染成排行榜。
//
// 根行只给总量和文件数，不带条形图和百分比 —— 百分比需要分母，而根的分母
// （父目录或磁盘总量）并不在扫描范围内，硬给一个数字是骗人。子项的百分比
// 分母是它的直接父级，因此同层之和为 100%。
func Render(r *Result, t *Tree, opt RenderOptions) string {
	barW := opt.BarWidth
	if barW == 0 {
		barW = defaultBarWidth
	}

	var sb strings.Builder

	// ---- 头部提示：数字含义与默认不同时必须说清楚 ----
	if !r.Supported {
		sb.WriteString("注：本平台无法测量实际占盘，以下为逻辑大小（Size()）。\n\n")
	} else if r.Apparent {
		sb.WriteString("注：--apparent 模式，以下为逻辑大小而非实际占盘。\n\n")
	}

	// ---- 根行 ----
	rootLine := fmt.Sprintf("%s  %s", t.Name, termx.HumanBytes1(t.Bytes))
	if t.Files > 0 {
		rootLine += fmt.Sprintf("  （%s 个文件）", termx.Comma(t.Files))
	}
	sb.WriteString(rootLine)
	sb.WriteString("\n")

	// ---- 子项表 ----
	rows := flattenRows(t, barW, opt.Color)
	if len(rows) > 0 {
		sb.WriteString("\n")
		if opt.MaxWidth > 0 {
			truncateNames(rows, opt.MaxWidth)
		}
		// header=nil：排行榜没有列名，每行都是数据。
		sb.WriteString(termx.Table(nil, toCells(rows), map[int]bool{1: true}))
	}

	// ---- 页脚 ----
	if foot := footer(r, opt); foot != "" {
		sb.WriteString("\n")
		sb.WriteString(foot)
	}
	return sb.String()
}

// row 是渲染中的一行子项。
type row struct {
	name string // 已含缩进
	size string
	bar  string
}

func toCells(rows []row) [][]string {
	out := make([][]string, len(rows))
	for i, r := range rows {
		out[i] = []string{r.name, r.size, r.bar}
	}
	return out
}

// flattenRows 深度优先展开成行。顺序完全由 BuildTree 排好的 Kids 决定，
// 这里不再排序 —— 排序规则只有一处，避免两边不一致。
func flattenRows(t *Tree, barW int, color bool) []row {
	var rows []row
	var walk func(parent *Tree, level int)
	walk = func(parent *Tree, level int) {
		for _, k := range parent.Kids {
			// 条形和数字必须用同一个百分比，否则边界上会出现「条形满格但写着
			// 99.5%」这种自相矛盾的显示。
			pct := percentOf(k.Bytes, parent.Bytes)
			ipct := int(math.Round(pct))
			indent := strings.Repeat(" ", indentPerLevel*(level+1))
			rows = append(rows, row{
				name: indent + k.displayName(),
				size: termx.HumanBytes1(k.Bytes),
				bar: termx.Colorize(termx.Bar(ipct, barW), ipct, color) + " " +
					termx.Colorize(fmt.Sprintf("%5.1f%%", pct), ipct, color),
			})
			walk(k, level+1)
		}
	}
	walk(t, 0)
	return rows
}

func percentOf(part, whole uint64) float64 {
	if whole == 0 {
		return 0
	}
	return 100 * float64(part) / float64(whole)
}

// truncateNames 在表格自然宽度超过 maxWidth 时，把名称列中间省略号截断。
// 体积和条形是固定宽度的，只能从名称列要空间。
func truncateNames(rows []row, maxWidth int) {
	nameW, sizeW, barW := 0, 0, 0
	for _, r := range rows {
		nameW = max(nameW, termx.VisWidth(r.name))
		sizeW = max(sizeW, termx.VisWidth(r.size))
		barW = max(barW, termx.VisWidth(r.bar))
	}
	total := nameW + sizeW + barW + 4 // 两处两空格分隔
	if total <= maxWidth {
		return
	}
	budget := max(maxWidth-sizeW-barW-4, minNameWidth)
	for i := range rows {
		rows[i].name = termx.TruncMiddle(rows[i].name, budget)
	}
}

// footer 汇总耗时和权限错误。不 sudo 也要能出有用结果，但必须让用户知道
// 数字可能偏小 —— 静默吞掉错误比报错更糟。
func footer(r *Result, opt RenderOptions) string {
	var parts []string
	if opt.Elapsed > 0 {
		parts = append(parts, "用时 "+opt.Elapsed.Round(time.Millisecond).String())
	}
	if n := len(r.Errors); n > 0 {
		msg := fmt.Sprintf("%d 个目录无权访问，结果可能偏小", n)
		if !opt.Verbose {
			msg += "（--verbose 看详情）"
		}
		parts = append(parts, msg)
	}
	if r.Deduped > 0 {
		parts = append(parts, fmt.Sprintf("%s 个硬链接已去重", termx.Comma(uint64(r.Deduped))))
	}
	if len(parts) == 0 {
		return ""
	}
	out := strings.Join(parts, " / ") + "\n"

	if opt.Verbose && len(r.Errors) > 0 {
		var sb strings.Builder
		sb.WriteString(out)
		for _, e := range r.SortedErrors() {
			fmt.Fprintf(&sb, "  %s: %v\n", e.Path, e.Err)
		}
		return sb.String()
	}
	return out
}
