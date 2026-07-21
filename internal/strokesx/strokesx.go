// Package strokesx 查汉字笔画数。
//
// 数据是 Unicode Unihan 的 kTotalStrokes 字段（公有领域），由
// _tools/gen_strokes.go 生成 strokes_dict.go。只做笔画**数**，不做笔顺
// （横竖撇捺折序列）—— 笔顺没有权威的开放机读数据，见设计文档 Premise 2。
package strokesx

import (
	"sort"
	"unicode"
)

// StrokeCount 返回单字笔画数。ok=false 表示该字不在表中。
//
// 二分查找 strokeCP（升序），命中则取 strokeN 对应位。
func StrokeCount(r rune) (n int, ok bool) {
	i := sort.Search(len(strokeCP), func(i int) bool { return strokeCP[i] >= r })
	if i < len(strokeCP) && strokeCP[i] == r {
		return int(strokeN[i]), true
	}
	return 0, false
}

// CharStroke 是逐字结果。
type CharStroke struct {
	Rune    rune
	Strokes int
	Known   bool // false = 是汉字但表里查不到
}

// Result 是整串的逐字笔画。
type Result struct {
	Chars   []CharStroke // 只含汉字（非汉字被跳过，不进这里）
	Total   int          // 已知汉字的笔画总和
	Unknown int          // 是汉字但表里查不到的个数
}

// StringStrokes 逐字返回笔画。非汉字（字母/数字/标点/emoji）跳过不计。
//
// 查表顺序是关键（见设计文档）：**先查表，查不到再判是不是汉字**。
//
//   - 先查表：命中就计入。这样即便 unicode.Han 落后于表的 Unicode 版本
//     （表来自 Unicode 17，Go 工具链可能还没收录最新扩展区），已经在表里的
//     字也不会被漏。
//   - 表里没有时才用 unicode.Is(unicode.Han, r) 分类：是汉字 → 记为「未知」；
//     不是 → 跳过。用标准库而非手写码点区间，因为手写「CJK + 扩展 A-G」会漏
//     掉兼容表意字和新扩展区（实测漏 9.8%）。
func StringStrokes(s string) Result {
	var res Result
	for _, r := range s {
		if n, ok := StrokeCount(r); ok {
			res.Chars = append(res.Chars, CharStroke{Rune: r, Strokes: n, Known: true})
			res.Total += n
			continue
		}
		if unicode.Is(unicode.Han, r) {
			// 是汉字但表里没有：记为未知，计入 Unknown，但不进 Total。
			res.Chars = append(res.Chars, CharStroke{Rune: r, Known: false})
			res.Unknown++
		}
		// 非汉字：跳过，什么都不做。
	}
	return res
}

// Count 返回表里收录的汉字总数（供诊断/测试）。
func Count() int { return len(strokeCP) }
