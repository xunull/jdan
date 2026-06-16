package figlet

import (
	"fmt"
	"strings"
	"unicode/utf8"
)

// DefaultFont 是默认字体名。
const DefaultFont = "standard"

// glyphCell 是一个待渲染字符的字模 + 宽度。
type glyphCell struct {
	rows []string
	w    int
}

// Render 把 text 渲染成 ASCII art（多行）。
//   - font: 字体名（standard / block），找不到返回错误
//   - width: 最大宽度，超过自动换到下一"块"（多行堆叠）；<=0 不换行
//   - center: 在 width 内居中（width>0 时生效）
//
// 不支持的字符用等高空白（宽度 1）占位。
func Render(text, fontName string, width int, center bool) ([]string, error) {
	f := Lookup(fontName)
	if f == nil {
		return nil, fmt.Errorf("unknown font %q (available: %s)", fontName, strings.Join(sortedNames(), ", "))
	}
	if text == "" {
		return nil, fmt.Errorf("empty text")
	}

	cells := make([]glyphCell, 0, len(text))
	for _, r := range text {
		g := f.Glyph(r)
		if g == nil {
			blank := make([]string, f.Height)
			for i := range blank {
				blank[i] = " "
			}
			cells = append(cells, glyphCell{rows: blank, w: 1})
			continue
		}
		cells = append(cells, glyphCell{rows: g, w: utf8.RuneCountInString(g[0])})
	}

	// 按 width 分组（每组拼成 f.Height 行）
	var out []string
	var line []glyphCell
	lineWidth := 0
	flush := func() {
		if len(line) == 0 {
			return
		}
		block := renderBlock(line, f.Height)
		if center && width > 0 {
			block = centerBlock(block, width)
		}
		out = append(out, block...)
		line = nil
		lineWidth = 0
	}
	for _, g := range cells {
		add := g.w
		if len(line) > 0 {
			add++ // gutter
		}
		if width > 0 && lineWidth+add > width && len(line) > 0 {
			flush()
			add = g.w
		}
		line = append(line, g)
		lineWidth += add
	}
	flush()
	return out, nil
}

// renderBlock 把一行 glyph 横向拼接成 height 行，glyph 间留 1 空格 gutter。
func renderBlock(line []glyphCell, height int) []string {
	rows := make([]string, height)
	for i := range height {
		var sb strings.Builder
		for j, g := range line {
			if j > 0 {
				sb.WriteByte(' ')
			}
			r := ""
			if i < len(g.rows) {
				r = g.rows[i]
			}
			if pad := g.w - utf8.RuneCountInString(r); pad > 0 {
				r += strings.Repeat(" ", pad)
			}
			sb.WriteString(r)
		}
		rows[i] = strings.TrimRight(sb.String(), " ")
	}
	return rows
}

// centerBlock 在 width 内居中每一行。
func centerBlock(block []string, width int) []string {
	out := make([]string, len(block))
	for i, line := range block {
		pad := max((width-len([]rune(line)))/2, 0)
		out[i] = strings.Repeat(" ", pad) + line
	}
	return out
}

func sortedNames() []string {
	names := FontNames()
	for i := range names {
		for j := i + 1; j < len(names); j++ {
			if names[j] < names[i] {
				names[i], names[j] = names[j], names[i]
			}
		}
	}
	return names
}
