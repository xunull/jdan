// Package pinyinx 是 go-pinyin 的薄封装：把中文转成拼音，非汉字原样穿插保留。
//
// go-pinyin 逐字查 Unihan 读音表（离线），但默认会丢弃没有读音的字符。本包自己分词
// （连续非汉字归成一段字面 token），从而保住句子结构；再套一层样式/多音字/渲染。
// 是 jdan pinyin / t9 / sp / spt9 共用的拼音基建。
package pinyinx

import (
	"strings"

	"github.com/mozillazg/go-pinyin"
)

// styleMap：jdan 的样式名 → go-pinyin 的 Style 常量。
var styleMap = map[string]int{
	"tone":     pinyin.Tone,        // zhōng wén（默认，带调符）
	"num":      pinyin.Tone3,       // zhong1 wen2（数字调）
	"plain":    pinyin.Normal,      // zhong wen（无调）
	"initials": pinyin.Initials,    // zh w（声母）
	"first":    pinyin.FirstLetter, // z w（首字母）
}

// StyleNames 按展示顺序返回可选样式名。
func StyleNames() []string { return []string{"tone", "num", "plain", "initials", "first"} }

// StyleValid 报告样式名是否合法。
func StyleValid(s string) bool { _, ok := styleMap[s]; return ok }

// Token 是一个输出单元：一个汉字（含读音），或一段连续的非汉字（字面）。
type Token struct {
	Text   string   `json:"text"`
	Han    bool     `json:"han"`
	Pinyin []string `json:"pinyin,omitempty"` // 汉字读音（多音字时多个）
}

// Options 控制转换。
type Options struct {
	Style     string // styleMap 的键；非法/空则回退 tone
	Heteronym bool   // 多音字：列出全部读音
}

// Convert 把文本转成 token 序列：能查到读音的字 → 汉字 token，其余连续字符 → 字面 token。
func Convert(text string, opts Options) []Token {
	styleID, ok := styleMap[opts.Style]
	if !ok {
		styleID = pinyin.Tone
	}
	a := pinyin.NewArgs()
	a.Style = styleID
	a.Heteronym = opts.Heteronym

	var tokens []Token
	var buf strings.Builder
	flush := func() {
		if buf.Len() > 0 {
			tokens = append(tokens, Token{Text: buf.String()})
			buf.Reset()
		}
	}
	for _, r := range text {
		reads := pinyin.Pinyin(string(r), a)
		if len(reads) > 0 && len(reads[0]) > 0 {
			flush()
			tokens = append(tokens, Token{Text: string(r), Han: true, Pinyin: reads[0]})
		} else {
			buf.WriteRune(r)
		}
	}
	flush()
	return tokens
}

// Join 渲染成一行：连续拼音音节之间插 sep（多音字用 "/" 连），非汉字段原样保留。
func Join(tokens []Token, sep string) string {
	var b strings.Builder
	prevPinyin := false
	for _, t := range tokens {
		if t.Han {
			if prevPinyin {
				b.WriteString(sep)
			}
			b.WriteString(strings.Join(t.Pinyin, "/"))
			prevPinyin = true
		} else {
			b.WriteString(t.Text)
			prevPinyin = false
		}
	}
	return b.String()
}

// Plain 返回单个字的「无调拼音·首读音」，非汉字/无读音返回 ""。供 t9/sp/spt9 复用。
func Plain(r rune) string {
	a := pinyin.NewArgs()
	a.Style = pinyin.Normal
	ps := pinyin.Pinyin(string(r), a)
	if len(ps) == 0 || len(ps[0]) == 0 {
		return ""
	}
	return ps[0][0]
}
