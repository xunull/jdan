// Package spt9x 把全拼音节转成【小鹤双拼】两码，并落到九宫格/T9 数字键。
//
// 小鹤双拼编码复用 shuangpinx（多方案双拼包，规则照 RIME 抄）的 flypy 方案；本包只
// 负责「双拼两码 → 每字母落 T9 键」与渲染。例：中 zhong → 小鹤 "vs" → v(8)s(7) = 87。
//
// 纯逻辑、0 依赖（runewidth 已是仓库依赖，仅用于 CJK 对齐）。
package spt9x

import (
	"encoding/json"
	"fmt"
	"strings"

	"github.com/mattn/go-runewidth"

	"github.com/xunull/jdan/internal/shuangpinx"
)

// Encode 把一个全拼音节转成小鹤双拼两码字母（小写）。ok=false 表示不是合法双拼码
// （非 2 键，如声母缺韵母/杂串）。委托给 shuangpinx 的小鹤方案。
func Encode(py string) (string, bool) {
	return shuangpinx.Flypy().Valid(py)
}

// Unit 是一个输出单元：一个汉字（含拼音+双拼两码）、一个英文单词、或一段数字。
type Unit struct {
	Text   string `json:"text"`
	Pinyin string `json:"pinyin,omitempty"`
	Code   string `json:"code,omitempty"` // 小鹤双拼两码（英文/数字段为空）
	Digits string `json:"digits"`
}

// Result 汇总一次转换。
type Result struct {
	Units   []Unit `json:"units"`
	Skipped int    `json:"skipped"`
}

// DigitString 各单元数字用空格连起来。
func (r Result) DigitString() string {
	parts := make([]string, 0, len(r.Units))
	for _, u := range r.Units {
		if u.Digits != "" {
			parts = append(parts, u.Digits)
		}
	}
	return strings.Join(parts, " ")
}

// Render 渲染逐单元对照（字·拼音·双拼·数字）+ 底部整串。
func (r Result) Render() string {
	if len(r.Units) == 0 {
		return ""
	}
	wt, wp, wc := 0, 0, 0
	for _, u := range r.Units {
		if w := runewidth.StringWidth(u.Text); w > wt {
			wt = w
		}
		if len(u.Pinyin) > wp {
			wp = len(u.Pinyin)
		}
		c := u.Code
		if c == "" {
			c = "—"
		}
		if w := runewidth.StringWidth(c); w > wc {
			wc = w
		}
	}
	var b strings.Builder
	for _, u := range r.Units {
		py := u.Pinyin
		if py == "" {
			py = "—"
		}
		c := u.Code
		if c == "" {
			c = "—"
		}
		fmt.Fprintf(&b, "%s%s  %s%s  %s%s  %s\n",
			u.Text, pad(wt-runewidth.StringWidth(u.Text)),
			py, pad(wp-len(py)),
			c, pad(wc-runewidth.StringWidth(c)),
			u.Digits)
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
