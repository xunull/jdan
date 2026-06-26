// Package calx 渲染公历日历（月/年）。中文表头、周一/周日起始可选、可选 ISO 周数、
// 今天高亮（由调用方决定是否 Color）。纯 stdlib + 复用已有的 go-runewidth 算宽度。
package calx

import (
	"fmt"
	"regexp"
	"strconv"
	"strings"
	"time"

	"github.com/mattn/go-runewidth"
)

// Options 控制渲染。
type Options struct {
	WeekStart time.Weekday // 一周从哪天起（time.Monday / time.Sunday）
	WeekNum   bool         // 左栏显示 ISO 周数
	Color     bool         // 今天用反显高亮（仅在调用方确认是 TTY 时）
}

const blockWidth = 20 // 7 天 × 2 + 6 个空格

var cnWeekday = map[time.Weekday]string{
	time.Sunday: "日", time.Monday: "一", time.Tuesday: "二",
	time.Wednesday: "三", time.Thursday: "四", time.Friday: "五", time.Saturday: "六",
}

var ansiRe = regexp.MustCompile(`\x1b\[[0-9;]*m`)

// MonthGrid 排出该月的周网格：每周 7 个数，0 表示空格。
func MonthGrid(year int, month time.Month, weekStart time.Weekday) [][]int {
	first := time.Date(year, month, 1, 0, 0, 0, 0, time.UTC)
	offset := (int(first.Weekday()) - int(weekStart) + 7) % 7
	days := time.Date(year, month+1, 0, 0, 0, 0, 0, time.UTC).Day()

	var weeks [][]int
	week := make([]int, 7)
	col := offset
	for d := 1; d <= days; d++ {
		week[col] = d
		col++
		if col == 7 {
			weeks = append(weeks, week)
			week = make([]int, 7)
			col = 0
		}
	}
	if col > 0 {
		weeks = append(weeks, week)
	}
	return weeks
}

// MonthLines 渲染一个月块为固定 8 行（标题 + 表头 + 6 周；不足补空行，便于并排）。
// today>0 表示该月里今天是几号；<=0 表示该月无今天。
func MonthLines(year int, month time.Month, opts Options, today int) []string {
	weeks := MonthGrid(year, month, opts.WeekStart)

	hdr := make([]string, 7)
	for i := range hdr {
		hdr[i] = cnWeekday[time.Weekday((int(opts.WeekStart)+i)%7)]
	}
	header := strings.Join(hdr, " ")
	title := center(fmt.Sprintf("%d 年 %d 月", year, month), blockWidth)

	lines := []string{title, header}
	for i := range 6 {
		if i < len(weeks) {
			lines = append(lines, weekLine(weeks[i], today, opts.Color))
		} else {
			lines = append(lines, strings.Repeat(" ", blockWidth))
		}
	}

	if opts.WeekNum {
		lines = withWeekNumbers(lines, weeks, year, month)
	}
	return lines
}

func weekLine(week []int, today int, color bool) string {
	cells := make([]string, 7)
	for i, d := range week {
		cells[i] = cell(d, today, color)
	}
	return strings.Join(cells, " ")
}

func cell(day, today int, color bool) string {
	if day == 0 {
		return "  "
	}
	s := fmt.Sprintf("%2d", day)
	if color && day == today {
		return "\x1b[7m" + s + "\x1b[0m"
	}
	return s
}

func withWeekNumbers(lines []string, weeks [][]int, year int, month time.Month) []string {
	out := make([]string, len(lines))
	out[0] = "   " + lines[0]
	out[1] = "   " + lines[1]
	for i := range 6 {
		wk := "  "
		if i < len(weeks) {
			if d := firstDay(weeks[i]); d > 0 {
				_, w := time.Date(year, month, d, 0, 0, 0, 0, time.UTC).ISOWeek()
				wk = fmt.Sprintf("%2d", w)
			}
		}
		out[2+i] = wk + " " + lines[2+i]
	}
	return out
}

func firstDay(week []int) int {
	for _, d := range week {
		if d > 0 {
			return d
		}
	}
	return 0
}

// Render 把一个月块的行拼成最终输出（去掉尾部空行）。
func Render(lines []string) string {
	end := len(lines)
	for end > 0 && strings.TrimSpace(lines[end-1]) == "" {
		end--
	}
	return strings.Join(lines[:end], "\n") + "\n"
}

// RenderBlocks 把多个月块每 perRow 个并排渲染（年历/三联排用）。
func RenderBlocks(blocks [][]string, perRow int) string {
	var sb strings.Builder
	for i := 0; i < len(blocks); i += perRow {
		group := blocks[i:min(i+perRow, len(blocks))]

		widths := make([]int, len(group))
		for j, b := range group {
			for _, ln := range b {
				widths[j] = max(widths[j], visualWidth(ln))
			}
		}
		nLines := 0
		for _, b := range group {
			nLines = max(nLines, len(b))
		}
		for li := 0; li < nLines; li++ {
			parts := make([]string, len(group))
			for j, b := range group {
				ln := ""
				if li < len(b) {
					ln = b[li]
				}
				parts[j] = padVisual(ln, widths[j])
			}
			sb.WriteString(strings.TrimRight(strings.Join(parts, "   "), " "))
			sb.WriteString("\n")
		}
		sb.WriteString("\n")
	}
	return strings.TrimRight(sb.String(), "\n") + "\n"
}

func visualWidth(s string) int {
	return runewidth.StringWidth(ansiRe.ReplaceAllString(s, ""))
}

func padVisual(s string, width int) string {
	if w := visualWidth(s); w < width {
		return s + strings.Repeat(" ", width-w)
	}
	return s
}

func center(s string, width int) string {
	w := runewidth.StringWidth(s)
	if w >= width {
		return s
	}
	left := (width - w) / 2
	return strings.Repeat(" ", left) + s + strings.Repeat(" ", width-w-left)
}

// CenterWidth 给定可见宽度居中（年标题用）。
func CenterWidth(s string, width int) string { return center(s, width) }

// BlockWidth 返回单个月块的内容宽度（不含周数列）。
func BlockWidth() int { return blockWidth }

// lunarCellW 是农历月历每个格子的宽度（要容纳「闰六月」这种 6 列宽的农历月名）。
const lunarCellW = 6

// MonthLinesSub 渲染农历月历：每个公历日占两行（上公历数字、下农历副标签）。
// sub(day) 返回该公历日的副标签（如农历月名「五月」或日名「初二」）；空串则该格留白。
// calx 不依赖农历逻辑，副标签全靠调用方的回调注入。
func MonthLinesSub(year int, month time.Month, opts Options, today int, sub func(day int) string) []string {
	weeks := MonthGrid(year, month, opts.WeekStart)
	totalW := lunarCellW * 7

	hdr := make([]string, 7)
	for i := range 7 {
		hdr[i] = center(cnWeekday[time.Weekday((int(opts.WeekStart)+i)%7)], lunarCellW)
	}
	lines := []string{
		center(fmt.Sprintf("%d 年 %d 月", year, month), totalW),
		strings.Join(hdr, ""),
	}

	for i := range len(weeks) {
		var dayRow, subRow strings.Builder
		for _, day := range weeks[i] {
			if day == 0 {
				blank := strings.Repeat(" ", lunarCellW)
				dayRow.WriteString(blank)
				subRow.WriteString(blank)
				continue
			}
			dayRow.WriteString(lunarDayCell(day, today, opts.Color))
			subRow.WriteString(center(sub(day), lunarCellW))
		}
		lines = append(lines, strings.TrimRight(dayRow.String(), " "), strings.TrimRight(subRow.String(), " "))
	}
	return lines
}

// lunarDayCell 把公历日数字居中到格子宽度；今天可反显（ANSI 不计入可见宽度）。
func lunarDayCell(day, today int, color bool) string {
	s := strconv.Itoa(day)
	left := (lunarCellW - len(s)) / 2
	right := lunarCellW - len(s) - left
	if color && day == today {
		s = "\x1b[7m" + s + "\x1b[0m"
	}
	return strings.Repeat(" ", left) + s + strings.Repeat(" ", right)
}
