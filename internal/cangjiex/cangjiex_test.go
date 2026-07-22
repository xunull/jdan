package cangjiex

import (
	"testing"
	"unicode"
)

// 黄金用例：码 + 字根，期望值全部对照真 Unihan kCangjie + 标准仓颉字根表核过。
func TestCangjie_KnownWithRoots(t *testing.T) {
	cases := []struct {
		r          rune
		code, root string
	}{
		{'明', "AB", "日月"},
		{'你', "ONF", "人弓火"},
		{'一', "M", "一"},
		{'變', "VFOK", "女火人大"},
		{'龗', "MBRRP", "一月口口心"},
	}
	for _, c := range cases {
		code, ok := Cangjie(c.r)
		if !ok {
			t.Errorf("%c 应在表中", c.r)
			continue
		}
		if code != c.code {
			t.Errorf("%c 码 = %q，应为 %q", c.r, code, c.code)
		}
		if got := Roots(code); got != c.root {
			t.Errorf("%c 字根 = %q，应为 %q", c.r, got, c.root)
		}
	}
}

// 25 键字根表必须覆盖 A-Y（含 X，无 Z），每个字母都能翻。
func TestRootTable_Complete(t *testing.T) {
	for c := byte('A'); c <= byte('Y'); c++ {
		if c == 'Z' {
			continue
		}
		if _, ok := rootOf[c]; !ok {
			t.Errorf("字根表缺字母 %c", c)
		}
	}
	if len(rootOf) != 25 {
		t.Errorf("字根表应 25 键，实际 %d", len(rootOf))
	}
	if _, ok := rootOf['Z']; ok {
		t.Error("Z 不该在字根表（仓颉不用 Z）")
	}
}

// Roots 对未知字母原样保留，不 panic。
func TestRoots_UnknownByte(t *testing.T) {
	if got := Roots("A1B"); got != "日1月" {
		t.Errorf("未知字符应原样保留：得 %q，应 日1月", got)
	}
	if got := Roots(""); got != "" {
		t.Errorf("空码应得空，得 %q", got)
	}
}

func TestCangjie_NotInTable(t *testing.T) {
	for _, r := range []rune{'A', '5', '，', ' ', '!', '😀'} {
		if _, ok := Cangjie(r); ok {
			t.Errorf("U+%X 不该在仓颉表中", r)
		}
	}
}

func TestStringCodes_PerChar(t *testing.T) {
	res := StringCodes("明一")
	if len(res.Chars) != 2 {
		t.Fatalf("应有 2 字，得 %d", len(res.Chars))
	}
	if res.Chars[0].Code != "AB" || res.Chars[0].Roots != "日月" {
		t.Errorf("首字应 明 AB 日月，得 %+v", res.Chars[0])
	}
	if res.Chars[1].Code != "M" || res.Chars[1].Roots != "一" {
		t.Errorf("次字应 一 M 一，得 %+v", res.Chars[1])
	}
	if res.Unknown != 0 {
		t.Errorf("Unknown 应 0，得 %d", res.Unknown)
	}
}

func TestStringCodes_SkipsNonHan(t *testing.T) {
	res := StringCodes("Hi 明 2024！😀")
	if len(res.Chars) != 1 {
		t.Errorf("应只统计 1 个汉字（明），得 %d：%+v", len(res.Chars), res.Chars)
	}
	if res.Unknown != 0 {
		t.Errorf("非汉字不该计入 Unknown，得 %d", res.Unknown)
	}
}

func TestStringCodes_UnknownHan(t *testing.T) {
	var unknownHan rune
	for r := rune(0x4E00); r < 0x9FFF; r++ {
		if unicode.Is(unicode.Han, r) {
			if _, ok := Cangjie(r); !ok {
				unknownHan = r
				break
			}
		}
	}
	if unknownHan == 0 {
		t.Skip("BMP 基本区没找到表外汉字，跳过")
	}
	res := StringCodes(string([]rune{'明', unknownHan, '一'}))
	if res.Unknown != 1 {
		t.Errorf("应有 1 个未知汉字，得 %d", res.Unknown)
	}
	for _, c := range res.Chars {
		if c.Rune == unknownHan {
			if c.Known || c.Code != "" || c.Roots != "" {
				t.Errorf("未知字应 Known=false、码/根为空，得 %+v", c)
			}
		}
	}
}

func TestStringCodes_Empty(t *testing.T) {
	res := StringCodes("")
	if len(res.Chars) != 0 || res.Unknown != 0 {
		t.Errorf("空串应全 0，得 %+v", res)
	}
	res = StringCodes("abc 123")
	if len(res.Chars) != 0 {
		t.Errorf("纯非汉字应无统计，得 %+v", res)
	}
}

// 表必须严格升序（二分查找的前提）。
func TestDict_SortedAndParallel(t *testing.T) {
	if len(cangjieCP) != len(cangjieCode) {
		t.Fatalf("两条 slice 长度不等：%d vs %d", len(cangjieCP), len(cangjieCode))
	}
	for i := 1; i < len(cangjieCP); i++ {
		if cangjieCP[i] <= cangjieCP[i-1] {
			t.Fatalf("cangjieCP 在 %d 处非严格升序：0x%X 后是 0x%X", i, cangjieCP[i-1], cangjieCP[i])
		}
	}
}

// 码只应含 A-Y（含 X，无 Z），且长 1-5。
func TestDict_CodesValid(t *testing.T) {
	maxLen := 0
	for _, code := range cangjieCode {
		if len(code) == 0 {
			t.Fatal("有空码")
		}
		if len(code) > maxLen {
			maxLen = len(code)
		}
		for i := 0; i < len(code); i++ {
			c := code[i]
			if c < 'A' || c > 'Y' {
				t.Fatalf("码 %q 含非法字母 %c", code, c)
			}
		}
	}
	if maxLen != 5 {
		t.Errorf("码最长应 5，实际 %d", maxLen)
	}
}

func TestCount(t *testing.T) {
	if n := Count(); n < 29000 || n > 29500 {
		t.Errorf("表应收录约 2.9 万字，实际 %d", n)
	}
}
