// Package t9x 把字母（拼音或英文）映射成九宫格/T9 键盘的数字键，并渲染对照结果。
//
// T9 键位：2 abc / 3 def / 4 ghi / 5 jkl / 6 mno / 7 pqrs / 8 tuv / 9 wxyz。
// 本包只做「字母串 → 数字」这层纯逻辑 + 渲染，0 依赖（runewidth 已是仓库依赖，
// 仅用于 CJK 对齐）。汉字→拼音那一步（需要数据字典）由 CLI 层调 go-pinyin 完成。
package t9x

import (
	"encoding/json"
	"fmt"
	"strings"

	"github.com/mattn/go-runewidth"
)

// letterDigit[c-'a'] = 该字母对应的 T9 数字键。
var letterDigit = buildLetterDigit()

func buildLetterDigit() [26]byte {
	groups := []struct {
		digit   byte
		letters string
	}{
		{'2', "abc"}, {'3', "def"}, {'4', "ghi"}, {'5', "jkl"},
		{'6', "mno"}, {'7', "pqrs"}, {'8', "tuv"}, {'9', "wxyz"},
	}
	var m [26]byte
	for _, g := range groups {
		for i := range len(g.letters) {
			m[g.letters[i]-'a'] = g.digit
		}
	}
	return m
}

// LetterDigit 返回单个字母（a-z/A-Z）对应的 T9 数字键；非字母返回 0,false。
func LetterDigit(b byte) (byte, bool) {
	c := b
	if c >= 'A' && c <= 'Z' {
		c += 'a' - 'A'
	}
	if c >= 'a' && c <= 'z' {
		return letterDigit[c-'a'], true
	}
	return 0, false
}

// LettersToDigits 把一串字母（拼音或英文单词）映射成 T9 数字串；非字母字符跳过。
func LettersToDigits(s string) string {
	var b strings.Builder
	for i := range len(s) {
		if d, ok := LetterDigit(s[i]); ok {
			b.WriteByte(d)
		}
	}
	return b.String()
}

// Unit 是一个输出单元：一个汉字、一个英文单词、或一段阿拉伯数字。
type Unit struct {
	Text   string `json:"text"`             // 源文本
	Pinyin string `json:"pinyin,omitempty"` // 汉字的拼音（英文/数字段为空）
	Digits string `json:"digits"`           // T9 数字键
}

// Result 汇总一次转换。
type Result struct {
	Units   []Unit `json:"units"`
	Skipped int    `json:"skipped"` // 跳过的无法映射字符数（不含空格/标点）
}

// DigitString 把各单元的数字用空格连起来（整串按键序列）。
func (r Result) DigitString() string {
	parts := make([]string, 0, len(r.Units))
	for _, u := range r.Units {
		if u.Digits != "" {
			parts = append(parts, u.Digits)
		}
	}
	return strings.Join(parts, " ")
}

// Render 渲染逐单元对照表 + 底部整串数字。
func (r Result) Render() string {
	if len(r.Units) == 0 {
		return ""
	}
	wt, wp := 0, 0
	for _, u := range r.Units {
		if w := runewidth.StringWidth(u.Text); w > wt {
			wt = w
		}
		p := u.Pinyin
		if p == "" {
			p = "—"
		}
		if w := runewidth.StringWidth(p); w > wp {
			wp = w
		}
	}
	var b strings.Builder
	for _, u := range r.Units {
		p := u.Pinyin
		if p == "" {
			p = "—"
		}
		fmt.Fprintf(&b, "%s%s  %s%s  %s\n",
			u.Text, pad(wt-runewidth.StringWidth(u.Text)),
			p, pad(wp-runewidth.StringWidth(p)), u.Digits)
	}
	b.WriteString(strings.Repeat("─", 5) + "\n")
	b.WriteString(r.DigitString() + "\n")
	return b.String()
}

// FormatJSON 输出结构化结果。
func (r Result) FormatJSON() (string, error) {
	if r.Units == nil {
		r.Units = []Unit{}
	}
	data, err := json.MarshalIndent(r, "", "  ")
	if err != nil {
		return "", err
	}
	return string(data), nil
}

func pad(n int) string {
	if n <= 0 {
		return ""
	}
	return strings.Repeat(" ", n)
}
