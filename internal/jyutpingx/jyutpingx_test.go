package jyutpingx

import (
	"testing"
	"unicode"
)

// 黄金用例：期望值对照真 Unihan kCantonese 核过。
func TestJyutping_Known(t *testing.T) {
	cases := map[rune]string{
		'你': "nei5",
		'好': "hou2",
		'我': "ngo5",
		'愛': "oi3",
		'世': "sai3",
		'界': "gaai3",
		'大': "daai6",
		'明': "ming4",
	}
	for r, want := range cases {
		got, ok := Jyutping(r)
		if !ok {
			t.Errorf("%c 应在表中", r)
			continue
		}
		if got != want {
			t.Errorf("%c = %q，应为 %q", r, got, want)
		}
	}
}

func TestJyutping_NotInTable(t *testing.T) {
	for _, r := range []rune{'A', '5', '，', ' ', '!', '😀'} {
		if _, ok := Jyutping(r); ok {
			t.Errorf("U+%X 不该在粤拼表中", r)
		}
	}
}

func TestStringReadings_PerChar(t *testing.T) {
	res := StringReadings("你好")
	if len(res.Chars) != 2 {
		t.Fatalf("应有 2 字，得 %d", len(res.Chars))
	}
	if res.Chars[0].Reading != "nei5" || res.Chars[1].Reading != "hou2" {
		t.Errorf("你好 应 nei5/hou2，得 %q/%q", res.Chars[0].Reading, res.Chars[1].Reading)
	}
	if res.Unknown != 0 {
		t.Errorf("Unknown 应 0，得 %d", res.Unknown)
	}
}

func TestStringReadings_Phrase(t *testing.T) {
	res := StringReadings("我爱广东")
	want := []string{"ngo5", "oi3", "gwong2", "dung1"}
	if len(res.Chars) != 4 {
		t.Fatalf("应有 4 字，得 %d", len(res.Chars))
	}
	for i, c := range res.Chars {
		if c.Reading != want[i] {
			t.Errorf("第 %d 字 %c = %q，应为 %q", i, c.Rune, c.Reading, want[i])
		}
	}
}

func TestStringReadings_SkipsNonHan(t *testing.T) {
	res := StringReadings("Hi 你 2024！😀")
	if len(res.Chars) != 1 {
		t.Errorf("应只统计 1 个汉字（你），得 %d：%+v", len(res.Chars), res.Chars)
	}
	if res.Unknown != 0 {
		t.Errorf("非汉字不该计入 Unknown，得 %d", res.Unknown)
	}
}

func TestStringReadings_UnknownHan(t *testing.T) {
	var unknownHan rune
	for r := rune(0x4E00); r < 0x9FFF; r++ {
		if unicode.Is(unicode.Han, r) {
			if _, ok := Jyutping(r); !ok {
				unknownHan = r
				break
			}
		}
	}
	if unknownHan == 0 {
		t.Skip("BMP 基本区没找到表外汉字，跳过")
	}
	res := StringReadings(string([]rune{'你', unknownHan, '好'}))
	if res.Unknown != 1 {
		t.Errorf("应有 1 个未知汉字，得 %d", res.Unknown)
	}
	for _, c := range res.Chars {
		if c.Rune == unknownHan {
			if c.Known || c.Reading != "" {
				t.Errorf("未知字应 Known=false、读音空，得 %+v", c)
			}
		}
	}
}

func TestStringReadings_Empty(t *testing.T) {
	res := StringReadings("")
	if len(res.Chars) != 0 || res.Unknown != 0 {
		t.Errorf("空串应全 0，得 %+v", res)
	}
	res = StringReadings("abc 123")
	if len(res.Chars) != 0 {
		t.Errorf("纯非汉字应无统计，得 %+v", res)
	}
}

// 表必须严格升序（二分查找的前提）。
func TestDict_SortedAndParallel(t *testing.T) {
	if len(jyutCP) != len(jyutReading) {
		t.Fatalf("两条 slice 长度不等：%d vs %d", len(jyutCP), len(jyutReading))
	}
	for i := 1; i < len(jyutCP); i++ {
		if jyutCP[i] <= jyutCP[i-1] {
			t.Fatalf("jyutCP 在 %d 处非严格升序：0x%X 后是 0x%X", i, jyutCP[i-1], jyutCP[i])
		}
	}
}

// 读音只应含 a-z0-9（Jyutping 字符集）。
func TestDict_ReadingsValid(t *testing.T) {
	for _, rd := range jyutReading {
		if rd == "" {
			t.Fatal("有空读音")
		}
		for _, c := range rd {
			if !(c >= 'a' && c <= 'z') && !(c >= '0' && c <= '9') {
				t.Fatalf("读音 %q 含非法字符 %q", rd, c)
			}
		}
	}
}

func TestCount(t *testing.T) {
	if n := Count(); n < 29000 || n > 30500 {
		t.Errorf("表应收录约 2.99 万字，实际 %d", n)
	}
}
