// Package cangjiex 查汉字的仓颉码（朱邦復输入法，台/港主流）。
//
// 数据是 Unicode Unihan 的 kCangjie 字段（仓颉三代，公开），由 _tools/gen_cangjie.go
// 生成 cangjie_dict.go。只做正查（字→码）；并用固定 25 键字根表把字母码翻成字根显示。
//
// 仓颉把一个字拆成 1-5 个字根，每根对应一个字母键（A-Y，含 X=難）。明=日+月=AB。
// 拆成哪几个字根靠字形、从码点算不出来（同笔顺）—— 只能查表。字母↔字根则是固定映射。
package cangjiex

import (
	"sort"
	"strings"
	"unicode"
)

// rootOf 是 25 键字母→字根映射（仓颉键盘，固定）。X=難，无 Z。
var rootOf = map[byte]rune{
	'A': '日', 'B': '月', 'C': '金', 'D': '木', 'E': '水', 'F': '火', 'G': '土',
	'H': '竹', 'I': '戈', 'J': '十', 'K': '大', 'L': '中', 'M': '一', 'N': '弓',
	'O': '人', 'P': '心', 'Q': '手', 'R': '口',
	'S': '尸', 'T': '廿', 'U': '山', 'V': '女', 'W': '田', 'Y': '卜', 'X': '難',
}

// Cangjie 返回单字的仓颉字母码。ok=false 表示该字不在表中。
//
// 二分查找 cangjieCP（升序），命中则取 cangjieCode 对应位（单值整串）。
func Cangjie(r rune) (code string, ok bool) {
	i := sort.Search(len(cangjieCP), func(i int) bool { return cangjieCP[i] >= r })
	if i < len(cangjieCP) && cangjieCP[i] == r {
		return cangjieCode[i], true
	}
	return "", false
}

// Roots 把字母码翻成字根串："AB"→"日月"。码是 ASCII 字母（单字节），逐字节翻；
// 未知字母原样保留（防越界，实际不该出现）。
func Roots(code string) string {
	var b strings.Builder
	for i := 0; i < len(code); i++ {
		if r, ok := rootOf[code[i]]; ok {
			b.WriteRune(r)
		} else {
			b.WriteByte(code[i])
		}
	}
	return b.String()
}

// CharCode 是逐字结果。
type CharCode struct {
	Rune  rune
	Code  string // 字母码，如 "AB"；表外为空
	Roots string // 字根，如 "日月"；表外为空
	Known bool   // false = 是汉字但表里查不到
}

// Result 是整串的逐字仓颉码。无 Total。
type Result struct {
	Chars   []CharCode // 只含汉字（非汉字被跳过，不进这里）
	Unknown int        // 是汉字但表里查不到的个数
}

// StringCodes 逐字返回仓颉码。非汉字（字母/数字/标点/emoji）跳过不计。
//
// 查表顺序同 strokesx/sijiaox：先查表、查不到再 unicode.Is(unicode.Han,r) 分未知/跳过。
func StringCodes(s string) Result {
	var res Result
	for _, r := range s {
		if code, ok := Cangjie(r); ok {
			res.Chars = append(res.Chars, CharCode{Rune: r, Code: code, Roots: Roots(code), Known: true})
			continue
		}
		if unicode.Is(unicode.Han, r) {
			res.Chars = append(res.Chars, CharCode{Rune: r, Known: false})
			res.Unknown++
		}
		// 非汉字：跳过。
	}
	return res
}

// Count 返回表里收录的汉字总数（供诊断/测试）。
func Count() int { return len(cangjieCP) }
