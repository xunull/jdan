// Package figlet 实现 jdan figlet 命令的核心：把文字渲染成 ASCII art 大横幅。
//
// 内置字体是 Go 源码里的 map[rune][]string（每个字符 N 行），覆盖 A-Z / 0-9 /
// 空格 / 常见标点。不加载外部 .flf 文件，0 运行时依赖。lowercase 折叠到
// uppercase，不支持的字符渲染成空白占位。
package figlet

import "strings"

// Font 是一套字体：字符高度 + 每个 rune 的字模（已按字模自身宽度对齐）。
type Font struct {
	Name   string
	Height int
	glyphs map[rune][]string
}

// Glyph 返回某个字符的字模（N 行，已 padding 到等宽）。lowercase 折叠到
// uppercase；不支持的字符返回 nil（调用方用空白占位）。
func (f *Font) Glyph(r rune) []string {
	if g, ok := f.glyphs[r]; ok {
		return g
	}
	if r >= 'a' && r <= 'z' {
		if g, ok := f.glyphs[r-32]; ok {
			return g
		}
	}
	return nil
}

// fonts 是注册的字体表。
var fonts = map[string]*Font{}

// FontNames 返回已注册字体名（排序由调用方负责）。
func FontNames() []string {
	out := make([]string, 0, len(fonts))
	for name := range fonts {
		out = append(out, name)
	}
	return out
}

// Lookup 返回指定字体，不存在返回 nil。
func Lookup(name string) *Font {
	return fonts[strings.ToLower(strings.TrimSpace(name))]
}

// registerBitmap 把一份原始字模（每 rune 一个 []string，行数 = height，行宽
// 可不齐）规整成等宽字模并注册成字体。transform 可对每个字符做替换（block
// 字体把 '#' 换成 '█'）。
func registerBitmap(name string, height int, raw map[rune][]string, transform func(string) string) {
	g := make(map[rune][]string, len(raw))
	for r, rows := range raw {
		// 宽度取该字模最长行
		w := 0
		for _, line := range rows {
			if len(line) > w {
				w = len(line)
			}
		}
		padded := make([]string, height)
		for i := range height {
			line := ""
			if i < len(rows) {
				line = rows[i]
			}
			line += strings.Repeat(" ", w-len(line))
			if transform != nil {
				line = transform(line)
			}
			padded[i] = line
		}
		g[r] = padded
	}
	fonts[name] = &Font{Name: name, Height: height, glyphs: g}
}

func init() {
	registerBitmap("standard", 5, standardGlyphs, nil)
	// block 字体复用 standard 字模，把 '#' 换成实心块
	registerBitmap("block", 5, standardGlyphs, func(s string) string {
		return strings.ReplaceAll(s, "#", "█")
	})
}
