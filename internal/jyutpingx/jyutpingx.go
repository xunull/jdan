// Package jyutpingx 查汉字的粤拼（Jyutping 粤语读音）。
//
// 数据是 Unicode Unihan 的 kCantonese 字段（Jyutping 粤拼，公开），由 _tools/gen_jyutping.go
// 生成 jyutping_dict.go。只做正查（字→读音），**每字一个主读音**（单读音）。
//
// 诚实划界：kCantonese 每字只存一个主读音，列不了多音字的其它读法（不像 pinyin 的 --heteronym），
// 也不做词级上下文消歧。要多音+词级需换 rime-cantonese 等更全的粤语词典。见设计文档。
package jyutpingx

import (
	"sort"
	"unicode"
)

// Jyutping 返回单字的粤拼读音（Unihan 主读音）。ok=false 表示该字不在表中。
//
// 二分查找 jyutCP（升序），命中则取 jyutReading 对应位。
func Jyutping(r rune) (reading string, ok bool) {
	i := sort.Search(len(jyutCP), func(i int) bool { return jyutCP[i] >= r })
	if i < len(jyutCP) && jyutCP[i] == r {
		return jyutReading[i], true
	}
	return "", false
}

// CharReading 是逐字结果。
type CharReading struct {
	Rune    rune
	Reading string // 粤拼，如 "nei5"；表外为空
	Known   bool   // false = 是汉字但表里查不到
}

// Result 是整串的逐字粤拼。无 Total。
type Result struct {
	Chars   []CharReading // 只含汉字（非汉字被跳过，不进这里）
	Unknown int           // 是汉字但表里查不到的个数
}

// StringReadings 逐字返回粤拼。非汉字（字母/数字/标点/emoji）跳过不计。
//
// 查表顺序同 strokesx/sijiaox/cangjiex：先查表、查不到再 unicode.Is(unicode.Han,r) 分未知/跳过。
func StringReadings(s string) Result {
	var res Result
	for _, r := range s {
		if reading, ok := Jyutping(r); ok {
			res.Chars = append(res.Chars, CharReading{Rune: r, Reading: reading, Known: true})
			continue
		}
		if unicode.Is(unicode.Han, r) {
			res.Chars = append(res.Chars, CharReading{Rune: r, Known: false})
			res.Unknown++
		}
		// 非汉字：跳过。
	}
	return res
}

// Count 返回表里收录的汉字总数（供诊断/测试）。
func Count() int { return len(jyutCP) }
