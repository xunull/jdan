package sijiaox

import (
	"reflect"
	"testing"
	"unicode"
)

// 黄金用例：期望值全部对照真 Unihan kFourCornerCode 核过。
func TestFourCorner_Known(t *testing.T) {
	cases := map[rune][]string{
		'口': {"6000.0"},
		'一': {"1000.0"},
		'王': {"1010.4"},
		'囗': {"6000.0"},
		'专': {"5030"},             // 无附号，原样 4 位
		'业': {"3210"},             // 无附号
		'你': {"2729.0", "2729.2"}, // 多值：两个都要在
	}
	for r, want := range cases {
		got, ok := FourCorner(r)
		if !ok {
			t.Errorf("U+%X %c 应在表中", r, r)
			continue
		}
		if !reflect.DeepEqual(got, want) {
			t.Errorf("U+%X %c = %v，应为 %v", r, r, got, want)
		}
	}
}

// 多值绝不能被截断（gen 的核心不变量：与 gen_strokes 的 val[:i] 相反）。
func TestFourCorner_MultiValueNotTruncated(t *testing.T) {
	got, ok := FourCorner('你')
	if !ok {
		t.Fatal("你 应在表中")
	}
	if len(got) != 2 {
		t.Errorf("你 应有 2 个码（多值不截断），得 %d：%v", len(got), got)
	}
	// 全表扫一遍：确认有一批多值字都保留了 2 个码。
	multi := 0
	for _, s := range sijiaoCode {
		if len(splitFields(s)) >= 2 {
			multi++
		}
	}
	if multi < 100 {
		t.Errorf("全表多值字只 %d 个，疑似被截断（应约 149）", multi)
	}
}

func splitFields(s string) []string {
	var out []string
	cur := ""
	for _, c := range s {
		if c == ' ' {
			if cur != "" {
				out = append(out, cur)
				cur = ""
			}
			continue
		}
		cur += string(c)
	}
	if cur != "" {
		out = append(out, cur)
	}
	return out
}

func TestFourCorner_NotInTable(t *testing.T) {
	// 非汉字不该在表中。
	for _, r := range []rune{'A', '5', '，', ' ', '!', '😀'} {
		if _, ok := FourCorner(r); ok {
			t.Errorf("U+%X 不该在四角号码表中", r)
		}
	}
}

func TestStringCodes_PerChar(t *testing.T) {
	res := StringCodes("口业专")
	if len(res.Chars) != 3 {
		t.Fatalf("应有 3 字，得 %d", len(res.Chars))
	}
	want := [][]string{{"6000.0"}, {"3210"}, {"5030"}}
	for i, c := range res.Chars {
		if !reflect.DeepEqual(c.Codes, want[i]) {
			t.Errorf("第 %d 字 %c = %v，应为 %v", i, c.Rune, c.Codes, want[i])
		}
	}
	if res.Unknown != 0 {
		t.Errorf("Unknown 应为 0，得 %d", res.Unknown)
	}
}

// 非汉字混排：字母/标点/emoji 跳过，不计入、也不算未知。
func TestStringCodes_SkipsNonHan(t *testing.T) {
	res := StringCodes("Hi 口 2024！😀")
	if len(res.Chars) != 1 {
		t.Errorf("应只统计 1 个汉字（口），得 %d：%+v", len(res.Chars), res.Chars)
	}
	if res.Unknown != 0 {
		t.Errorf("非汉字不该计入 Unknown，得 %d", res.Unknown)
	}
}

// 未知汉字：是汉字但表里没有（本表仅 17k，很常见）→ Known=false、计入 Unknown。
func TestStringCodes_UnknownHan(t *testing.T) {
	var unknownHan rune
	for r := rune(0x4E00); r < 0x9FFF; r++ {
		if unicode.Is(unicode.Han, r) {
			if _, ok := FourCorner(r); !ok {
				unknownHan = r
				break
			}
		}
	}
	if unknownHan == 0 {
		t.Skip("BMP 基本区没找到表外汉字，跳过")
	}
	res := StringCodes(string([]rune{'口', unknownHan, '业'}))
	if res.Unknown != 1 {
		t.Errorf("应有 1 个未知汉字，得 %d", res.Unknown)
	}
	var sawUnknown bool
	for _, c := range res.Chars {
		if c.Rune == unknownHan {
			sawUnknown = true
			if c.Known {
				t.Error("未知字 Known 应为 false")
			}
			if c.Codes != nil {
				t.Errorf("未知字 Codes 应为 nil，得 %v", c.Codes)
			}
		}
	}
	if !sawUnknown {
		t.Error("未知汉字应仍出现在 Chars 里（Known=false）")
	}
}

func TestStringCodes_Empty(t *testing.T) {
	res := StringCodes("")
	if len(res.Chars) != 0 || res.Unknown != 0 {
		t.Errorf("空串应全 0，得 %+v", res)
	}
	res = StringCodes("abc 123 !@#")
	if len(res.Chars) != 0 {
		t.Errorf("纯非汉字应无统计，得 %+v", res)
	}
}

// 表必须严格升序（二分查找的前提）。
func TestDict_SortedAndParallel(t *testing.T) {
	if len(sijiaoCP) != len(sijiaoCode) {
		t.Fatalf("两条 slice 长度不等：%d vs %d", len(sijiaoCP), len(sijiaoCode))
	}
	for i := 1; i < len(sijiaoCP); i++ {
		if sijiaoCP[i] <= sijiaoCP[i-1] {
			t.Fatalf("sijiaoCP 在 %d 处非严格升序：0x%X 后是 0x%X", i, sijiaoCP[i-1], sijiaoCP[i])
		}
	}
}

func TestCount(t *testing.T) {
	if n := Count(); n < 16000 || n > 17500 {
		t.Errorf("表应收录约 1.69 万字，实际 %d", n)
	}
}
