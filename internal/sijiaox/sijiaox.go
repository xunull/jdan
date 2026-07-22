// Package sijiaox 查汉字的四角号码（王云五检字法）。
//
// 数据是 Unicode Unihan 的 kFourCornerCode 字段（公开），由 _tools/gen_sijiao.go
// 生成 sijiao_dict.go。只做**正查**（字→码）；反查（码→字）本版不做，见设计文档。
//
// 四角号码看字四个角的笔形取 4 位主码 + 1 位附号 → NNNN.N（无附号则 NNNN）。
// 这只能查表：四角靠字形，从码点算不出来（同笔顺——没有开放的字形/笔形数据）。
package sijiaox

import (
	"sort"
	"strings"
	"unicode"
)

// FourCorner 返回单字的四角号码。一个字可能有 1-2 个码（多值，如 你→2729.0 2729.2）。
// ok=false 表示该字不在表中。
//
// 二分查找 sijiaoCP（升序），命中则把 sijiaoCode 对应位的空格分隔串拆成 []string。
func FourCorner(r rune) (codes []string, ok bool) {
	i := sort.Search(len(sijiaoCP), func(i int) bool { return sijiaoCP[i] >= r })
	if i < len(sijiaoCP) && sijiaoCP[i] == r {
		return strings.Fields(sijiaoCode[i]), true
	}
	return nil, false
}

// CharCode 是逐字结果。
type CharCode struct {
	Rune  rune
	Codes []string // 1-2 个，如 ["1010.4"] 或 ["2729.0","2729.2"]；表外为 nil
	Known bool     // false = 是汉字但表里查不到
}

// Result 是整串的逐字四角号码。四角号码不可求和，故无 Total。
type Result struct {
	Chars   []CharCode // 只含汉字（非汉字被跳过，不进这里）
	Unknown int        // 是汉字但表里查不到的个数
}

// StringCodes 逐字返回四角号码。非汉字（字母/数字/标点/emoji）跳过不计。
//
// 查表顺序（同 strokesx）：**先查表，查不到再判是不是汉字**。本表仅约 1.7 万字、
// 比 strokes 的 10 万稀得多，Known=false 会更频繁触发，这个顺序更要守住：
//   - 先查表：命中就计入，即便 unicode.Han 落后于表也不漏。
//   - 表里没有时才用 unicode.Is(unicode.Han, r)：是汉字→记「未知」；不是→跳过。
func StringCodes(s string) Result {
	var res Result
	for _, r := range s {
		if codes, ok := FourCorner(r); ok {
			res.Chars = append(res.Chars, CharCode{Rune: r, Codes: codes, Known: true})
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
func Count() int { return len(sijiaoCP) }
