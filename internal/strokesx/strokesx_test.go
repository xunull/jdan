package strokesx

import (
	"testing"
	"unicode"
)

func TestStrokeCount_KnownChars(t *testing.T) {
	cases := map[rune]int{
		'一': 1,
		'永': 5,
		'龙': 5,  // 简体
		'龍': 16, // 繁体 —— 与简体不同码点、不同画数
		'中': 4,
		'鑫': 24, // 生僻（起名用）
		'龗': 33, // 更生僻
	}
	for r, want := range cases {
		n, ok := StrokeCount(r)
		if !ok {
			t.Errorf("U+%X %c 应在表中", r, r)
			continue
		}
		if n != want {
			t.Errorf("U+%X %c = %d 画，应为 %d", r, r, n, want)
		}
	}
}

// 繁简是不同码点、各有各的笔画，不能混。
func TestStrokeCount_TraditionalDiffersFromSimplified(t *testing.T) {
	s, sok := StrokeCount('龙')
	tr, tok := StrokeCount('龍')
	if !sok || !tok {
		t.Fatal("龙/龍 都应在表中")
	}
	if s == tr {
		t.Errorf("繁简画数不该相同：龙=%d 龍=%d", s, tr)
	}
	if s != 5 || tr != 16 {
		t.Errorf("龙应 5、龍应 16，得到 %d / %d", s, tr)
	}
}

// uint8 边界：实测最大 84 画（U+3106C 𱁬），不是设计初稿以为的 64。
func TestStrokeCount_MaxStrokeChar(t *testing.T) {
	n, ok := StrokeCount(0x3106C)
	if !ok {
		t.Fatal("U+3106C 应在表中")
	}
	if n != 84 {
		t.Errorf("U+3106C = %d 画，应为 84（uint8 边界用例）", n)
	}
	// 扫全表确认没有超过 uint8 的值溜进来
	max := 0
	for _, v := range strokeN {
		if int(v) > max {
			max = int(v)
		}
	}
	if max != 84 {
		t.Errorf("全表最大画数 = %d，应为 84", max)
	}
}

func TestStrokeCount_NotInTable(t *testing.T) {
	// 非汉字：拉丁字母、数字、标点
	for _, r := range []rune{'A', '5', '，', ' ', '!', '😀'} {
		if _, ok := StrokeCount(r); ok {
			t.Errorf("U+%X 不该在汉字笔画表中", r)
		}
	}
}

// 核心：先查表、再判汉字。兼容表意字（U+F900–FAFF）和扩展 H/I/J 在手写的
// 「CJK + 扩展 A-G」区间之外，但在笔画表里 —— 若先判区间会把它们漏掉。
func TestStringStrokes_TableFirstCoversCompatIdeographs(t *testing.T) {
	// U+F900 是兼容表意字（豈的兼容形），在表里有笔画。
	if n, ok := StrokeCount(0xF900); !ok {
		t.Errorf("兼容表意字 U+F900 应在表中（table-first 才不会漏），得到 ok=%v", ok)
	} else {
		t.Logf("U+F900 = %d 画", n)
	}

	// 端到端：含兼容字的串，该字必须被计入而不是被当非汉字跳过。
	res := StringStrokes("中豈文")
	if len(res.Chars) != 3 {
		t.Errorf("应有 3 个汉字（含兼容字 U+F900），得到 %d 个", len(res.Chars))
	}
	for _, c := range res.Chars {
		if !c.Known {
			t.Errorf("U+%X 应被查到而非记为未知", c.Rune)
		}
	}
}

func TestStringStrokes_PerCharAndTotal(t *testing.T) {
	res := StringStrokes("龙凤呈祥")
	if len(res.Chars) != 4 {
		t.Fatalf("应有 4 个字，得到 %d", len(res.Chars))
	}
	wantEach := []int{5, 4, 7, 10}
	for i, c := range res.Chars {
		if c.Strokes != wantEach[i] {
			t.Errorf("第 %d 字 %c = %d，应为 %d", i, c.Rune, c.Strokes, wantEach[i])
		}
	}
	if res.Total != 26 {
		t.Errorf("总画数 = %d，应为 5+4+7+10=26", res.Total)
	}
	if res.Unknown != 0 {
		t.Errorf("Unknown 应为 0，得到 %d", res.Unknown)
	}
}

// 非汉字混排：字母/标点/emoji 跳过，不计入、也不算「未知汉字」。
func TestStringStrokes_SkipsNonHan(t *testing.T) {
	res := StringStrokes("Hello 世界 2024！😀")
	// 只有 世界 两个汉字
	if len(res.Chars) != 2 {
		t.Errorf("应只统计 2 个汉字，得到 %d：%+v", len(res.Chars), res.Chars)
	}
	if res.Unknown != 0 {
		t.Errorf("非汉字不该计入 Unknown，得到 %d", res.Unknown)
	}
	world, _ := StrokeCount('世')
	realm, _ := StrokeCount('界')
	if res.Total != world+realm {
		t.Errorf("总数应为 世+界 = %d，得到 %d", world+realm, res.Total)
	}
}

// 未知汉字：是汉字但表里没有 → 记为 Known=false、计入 Unknown、不进 Total。
func TestStringStrokes_UnknownHan(t *testing.T) {
	// 找一个是汉字但不在表里的码点。unicode.Han 覆盖的区间里挑一个表外的。
	var unknownHan rune
	for r := rune(0x4E00); r < 0x9FFF; r++ {
		if unicode.Is(unicode.Han, r) {
			if _, ok := StrokeCount(r); !ok {
				unknownHan = r
				break
			}
		}
	}
	if unknownHan == 0 {
		t.Skip("BMP 基本区里没找到表外汉字（表覆盖很全），跳过")
	}

	res := StringStrokes(string([]rune{'中', unknownHan, '文'}))
	if res.Unknown != 1 {
		t.Errorf("应有 1 个未知汉字，得到 %d", res.Unknown)
	}
	// 总数只含已知的两个字
	zhong, _ := StrokeCount('中')
	wen, _ := StrokeCount('文')
	if res.Total != zhong+wen {
		t.Errorf("总数应只累加已知字 = %d，得到 %d", zhong+wen, res.Total)
	}
	// 未知字应出现在 Chars 里但 Known=false
	var sawUnknown bool
	for _, c := range res.Chars {
		if c.Rune == unknownHan {
			sawUnknown = true
			if c.Known {
				t.Error("未知字的 Known 应为 false")
			}
			if c.Strokes != 0 {
				t.Errorf("未知字的 Strokes 应为 0，得到 %d", c.Strokes)
			}
		}
	}
	if !sawUnknown {
		t.Error("未知汉字应仍出现在 Chars 里（标为 Known=false）")
	}
}

func TestStringStrokes_Empty(t *testing.T) {
	res := StringStrokes("")
	if len(res.Chars) != 0 || res.Total != 0 || res.Unknown != 0 {
		t.Errorf("空串应全 0，得到 %+v", res)
	}
	// 纯非汉字
	res = StringStrokes("abc 123 !@#")
	if len(res.Chars) != 0 || res.Total != 0 {
		t.Errorf("纯非汉字应无统计，得到 %+v", res)
	}
}

// 表必须严格升序（二分查找的前提）。
func TestDict_SortedAndParallel(t *testing.T) {
	if len(strokeCP) != len(strokeN) {
		t.Fatalf("两条 slice 长度不等：%d vs %d", len(strokeCP), len(strokeN))
	}
	for i := 1; i < len(strokeCP); i++ {
		if strokeCP[i] <= strokeCP[i-1] {
			t.Fatalf("strokeCP 在 %d 处非严格升序：0x%X 后是 0x%X", i, strokeCP[i-1], strokeCP[i])
		}
	}
}

func TestCount(t *testing.T) {
	if Count() < 100000 {
		t.Errorf("表应收录 10 万+ 字，实际 %d", Count())
	}
}
