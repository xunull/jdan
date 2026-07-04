// Package spt9x 把全拼音节转成【小鹤双拼(flypy)】两码，并落到九宫格/T9 数字键。
//
// 小鹤双拼：每个音节 = 一键声母 + 一键韵母（共 2 码）。声母 zh/ch/sh 分别记作
// v/i/u，其余声母=自身；韵母按下表映射到单个字母（权威来源：RIME
// rime-double-pinyin 的 double_pinyin_flypy.schema.yaml，逐条抄死）。
// 两个字母再各自落到标准 T9 键（复用 t9x）。例：中 zhong → zh=v,ong=s = "vs" → 87。
//
// 纯逻辑、0 依赖（runewidth 已是仓库依赖，仅用于 CJK 对齐）。
package spt9x

import (
	"encoding/json"
	"fmt"
	"strings"

	"github.com/mattn/go-runewidth"
)

// finalLetter：韵母 → 小鹤字母。照 RIME flypy 规则逐条列出。
var finalLetter = map[string]byte{
	// 单韵母
	"a": 'a', "o": 'o', "e": 'e', "i": 'i', "u": 'u', "v": 'v',
	// 复/鼻韵母
	"iu": 'q', "ei": 'w', "uan": 'r', "ue": 't', "ve": 't', "un": 'y',
	"uo": 'o', "ie": 'p', "ong": 's', "iong": 's', "ing": 'k', "uai": 'k',
	"ai": 'd', "en": 'f', "eng": 'g', "iang": 'l', "uang": 'l', "ang": 'h',
	"ian": 'm', "an": 'j', "ou": 'z', "ia": 'x', "ua": 'x', "iao": 'n',
	"ao": 'c', "ui": 'v', "in": 'b', "vn": 'y', "van": 'r',
}

var twoInitials = map[string]bool{"zh": true, "ch": true, "sh": true}

func initialLetter(sm string) byte {
	switch sm {
	case "zh":
		return 'v'
	case "ch":
		return 'i'
	case "sh":
		return 'u'
	}
	return sm[0] // 单声母 = 自身
}

// split 把音节拆成 (声母, 韵母)。a/o/e 开头视为零声母（声母为空）。
func split(py string) (sm, ym string) {
	if len(py) >= 2 && twoInitials[py[:2]] {
		return py[:2], py[2:]
	}
	switch py[0] {
	case 'a', 'o', 'e':
		return "", py // 零声母
	default:
		return py[:1], py[1:]
	}
}

// Encode 把一个全拼音节转成小鹤双拼两码字母（小写）。ok=false 表示无法解析。
func Encode(py string) (string, bool) {
	py = strings.ToLower(strings.TrimSpace(py))
	py = strings.ReplaceAll(py, "ü", "v")
	if py == "" {
		return "", false
	}
	if py == "er" {
		return "er", true // 特例：儿化/而
	}
	sm, ym := split(py)
	fl, ok := finalLetter[ym]
	if !ok {
		return "", false
	}
	if sm == "" {
		// 零声母：首字母(a/o/e) + 韵母键
		return string(py[0]) + string(fl), true
	}
	return string(initialLetter(sm)) + string(fl), true
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
