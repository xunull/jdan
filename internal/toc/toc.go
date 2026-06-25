// Package toc 实现 jdan toc 命令的核心：从 Markdown 标题生成目录，anchor 跟
// GitHub 渲染规则一致。0 新依赖（纯 stdlib）。
package toc

import (
	"fmt"
	"strings"
	"unicode"
)

// Heading 是一个 Markdown 标题。
type Heading struct {
	Level  int
	Text   string // 原始标题文字（含反引号等）
	Anchor string // GitHub 风格 anchor（已去重）
}

// toc 标记：--inplace 在这两者之间回填。
const (
	MarkerStart = "<!-- toc -->"
	MarkerEnd   = "<!-- /toc -->"
)

// ParseHeadings 解析全部 ATX 标题（跳过代码围栏），每个带去重后的 anchor。
// anchor 去重按全文出现顺序，跟 GitHub 一致。
func ParseHeadings(md string) []Heading {
	var out []Heading
	sl := slugger{}
	inFence := false
	fenceChar := byte(0)

	for line := range strings.SplitSeq(md, "\n") {
		trimmed := strings.TrimSpace(line)
		if marker, ok := fenceMarker(trimmed); ok {
			if !inFence {
				inFence, fenceChar = true, marker
			} else if marker == fenceChar {
				inFence = false
			}
			continue
		}
		if inFence {
			continue
		}
		level, text, ok := parseATX(line)
		if !ok {
			continue
		}
		out = append(out, Heading{Level: level, Text: text, Anchor: sl.slug(text)})
	}
	return out
}

// fenceMarker 报告 trimmed line 是否是 ``` 或 ~~~ 围栏，返回围栏字符。
func fenceMarker(trimmed string) (byte, bool) {
	if strings.HasPrefix(trimmed, "```") {
		return '`', true
	}
	if strings.HasPrefix(trimmed, "~~~") {
		return '~', true
	}
	return 0, false
}

// parseATX 解析一行 ATX 标题，返回级别、标题文字、是否是标题。
func parseATX(line string) (int, string, bool) {
	s := strings.TrimLeft(line, " ")
	level := 0
	for level < len(s) && s[level] == '#' {
		level++
	}
	if level == 0 || level > 6 {
		return 0, "", false
	}
	// # 后必须是空白或行尾
	if level < len(s) && s[level] != ' ' && s[level] != '\t' {
		return 0, "", false
	}
	text := strings.TrimLeft(s[level:], " \t")
	text = stripClosingHashes(strings.TrimRight(text, " \t"))
	if text == "" {
		return 0, "", false
	}
	return level, text, true
}

// stripClosingHashes 去掉 ATX 收尾 # 序列（必须前面是空白，故 "C#" 不被误删）。
func stripClosingHashes(text string) string {
	k := len(text)
	for k > 0 && text[k-1] == '#' {
		k--
	}
	if k < len(text) && (k == 0 || text[k-1] == ' ' || text[k-1] == '\t') {
		return strings.TrimRight(text[:k], " \t")
	}
	return text
}

// Slug 把标题文字转成 GitHub 风格 anchor（不去重）：lowercase，保留字母/数字/
// 标记/连字符/下划线，空格转连字符，其余删除（反引号、标点等）。
func Slug(text string) string {
	var b strings.Builder
	for _, r := range strings.ToLower(text) {
		switch {
		case unicode.IsLetter(r), unicode.IsNumber(r), unicode.IsMark(r):
			b.WriteRune(r)
		case r == ' ':
			b.WriteByte('-')
		case r == '-' || r == '_':
			b.WriteRune(r)
		}
	}
	return b.String()
}

// slugger 按全文顺序对重复 anchor 加 -1/-2 后缀（同 GitHub）。
type slugger map[string]int

func (s slugger) slug(text string) string {
	base := Slug(text)
	n, seen := s[base]
	if !seen {
		s[base] = 0
		return base
	}
	n++
	s[base] = n
	return fmt.Sprintf("%s-%d", base, n)
}

// Render 把标题按 [min,max] 级别过滤后渲染成缩进的 bullet 列表。
func Render(headings []Heading, min, max int) string {
	var f []Heading
	for _, h := range headings {
		if h.Level >= min && h.Level <= max {
			f = append(f, h)
		}
	}
	if len(f) == 0 {
		return ""
	}
	base := f[0].Level
	for _, h := range f {
		if h.Level < base {
			base = h.Level
		}
	}
	var b strings.Builder
	for _, h := range f {
		indent := strings.Repeat("  ", h.Level-base)
		fmt.Fprintf(&b, "%s- [%s](#%s)\n", indent, h.Text, h.Anchor)
	}
	return strings.TrimRight(b.String(), "\n")
}

// Insert 把 toc 回填到 content 的 <!-- toc --> ... <!-- /toc --> 之间。
// 缺标记报错。幂等（重复运行结果相同）。
func Insert(content, toc string) (string, error) {
	si := strings.Index(content, MarkerStart)
	ei := strings.Index(content, MarkerEnd)
	if si < 0 || ei < 0 || ei < si {
		return "", fmt.Errorf("找不到 %s ... %s 标记（请先在文件里加好）", MarkerStart, MarkerEnd)
	}
	before := content[:si+len(MarkerStart)]
	after := content[ei:]
	mid := ""
	if toc != "" {
		mid = toc + "\n"
	}
	return before + "\n" + mid + after, nil
}
