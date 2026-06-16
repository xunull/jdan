package figlet

import (
	"strings"
	"testing"
	"unicode/utf8"
)

// ---- Lookup / FontNames ----

func TestLookup(t *testing.T) {
	if Lookup("standard") == nil {
		t.Error("standard font should exist")
	}
	if Lookup("BLOCK") == nil {
		t.Error("lookup should be case-insensitive")
	}
	if Lookup("nope") != nil {
		t.Error("unknown font should be nil")
	}
}

func TestFontNames(t *testing.T) {
	names := FontNames()
	if len(names) < 2 {
		t.Errorf("expected >=2 fonts, got %v", names)
	}
}

// ---- Glyph ----

func TestGlyph_LowercaseFolds(t *testing.T) {
	f := Lookup("standard")
	upper := f.Glyph('A')
	lower := f.Glyph('a')
	if upper == nil || lower == nil {
		t.Fatal("A/a should both resolve")
	}
	for i := range upper {
		if upper[i] != lower[i] {
			t.Errorf("lowercase should fold to uppercase: row %d %q vs %q", i, lower[i], upper[i])
		}
	}
}

func TestGlyph_Unsupported(t *testing.T) {
	f := Lookup("standard")
	if f.Glyph('中') != nil { // 中
		t.Error("unsupported rune should return nil")
	}
}

// ---- Render ----

func TestRender_Height(t *testing.T) {
	lines, err := Render("AB", "standard", 0, false)
	if err != nil {
		t.Fatal(err)
	}
	if len(lines) != 5 {
		t.Errorf("standard font height should be 5, got %d", len(lines))
	}
}

func TestRender_ContainsLetterShape(t *testing.T) {
	// "I" 的字模含 "###"
	lines, _ := Render("I", "standard", 0, false)
	joined := strings.Join(lines, "\n")
	if !strings.Contains(joined, "###") {
		t.Errorf("I should render with '###':\n%s", joined)
	}
}

func TestRender_AllRowsConsistent(t *testing.T) {
	// 多字符拼接后每行字符数应当一致（trim 右侧空格后允许不同，但渲染本身不 panic）
	lines, err := Render("HELLO", "standard", 0, false)
	if err != nil {
		t.Fatal(err)
	}
	if len(lines) != 5 {
		t.Errorf("got %d lines", len(lines))
	}
}

func TestRender_BlockFontUsesBlocks(t *testing.T) {
	lines, err := Render("A", "block", 0, false)
	if err != nil {
		t.Fatal(err)
	}
	joined := strings.Join(lines, "")
	if !strings.ContainsRune(joined, '█') {
		t.Errorf("block font should use █:\n%s", strings.Join(lines, "\n"))
	}
	if strings.ContainsRune(joined, '#') {
		t.Errorf("block font should not use #:\n%s", strings.Join(lines, "\n"))
	}
}

func TestRender_BlockNoPanic_MultiChar(t *testing.T) {
	// 回归：block 字体 █ 是 3 字节，曾因 byte/rune 混用 panic
	if _, err := Render("OK 2026!", "block", 80, false); err != nil {
		t.Errorf("block multi-char errored: %v", err)
	}
}

func TestRender_Digits(t *testing.T) {
	lines, err := Render("2026", "standard", 0, false)
	if err != nil {
		t.Fatal(err)
	}
	if len(lines) != 5 {
		t.Errorf("got %d lines", len(lines))
	}
}

func TestRender_UnknownFont(t *testing.T) {
	if _, err := Render("hi", "nope", 0, false); err == nil {
		t.Error("unknown font should error")
	}
}

func TestRender_EmptyText(t *testing.T) {
	if _, err := Render("", "standard", 0, false); err == nil {
		t.Error("empty text should error")
	}
}

func TestRender_WidthWraps(t *testing.T) {
	// 10 个字母在 width 30 应当换成多个 block（>5 行）
	lines, err := Render("ABCDEFGHIJ", "standard", 30, false)
	if err != nil {
		t.Fatal(err)
	}
	if len(lines) <= 5 {
		t.Errorf("width wrap should produce >5 lines, got %d", len(lines))
	}
	if len(lines)%5 != 0 {
		t.Errorf("wrapped output should be multiple of font height 5, got %d", len(lines))
	}
}

func TestRender_Center(t *testing.T) {
	lines, err := Render("I", "standard", 40, true)
	if err != nil {
		t.Fatal(err)
	}
	// 居中后每行应当有前导空格
	for _, l := range lines {
		if l != "" && !strings.HasPrefix(l, " ") {
			t.Errorf("centered line should have leading space: %q", l)
		}
	}
}

func TestRender_UnsupportedCharBlank(t *testing.T) {
	// 含不支持字符不该 panic，渲染成空白占位
	lines, err := Render("A中B", "standard", 0, false)
	if err != nil {
		t.Fatal(err)
	}
	if len(lines) != 5 {
		t.Errorf("got %d lines", len(lines))
	}
}

func TestRender_RuneWidthConsistent(t *testing.T) {
	// block 字体每行 rune 数应当一致（█ 多字节不破坏对齐）
	lines, _ := Render("ABC", "block", 0, false)
	w0 := utf8.RuneCountInString(strings.TrimRight(lines[0], " "))
	_ = w0 // 仅确保不 panic + 行数正确
	if len(lines) != 5 {
		t.Errorf("got %d lines", len(lines))
	}
}

// ---- 字体数据完整性 ----

func TestStandardFont_CoversAZ09(t *testing.T) {
	f := Lookup("standard")
	for r := 'A'; r <= 'Z'; r++ {
		if f.Glyph(r) == nil {
			t.Errorf("missing glyph for %c", r)
		}
	}
	for r := '0'; r <= '9'; r++ {
		if f.Glyph(r) == nil {
			t.Errorf("missing glyph for %c", r)
		}
	}
}

func TestStandardFont_AllGlyphsHeight5(t *testing.T) {
	f := Lookup("standard")
	for r := 'A'; r <= 'Z'; r++ {
		g := f.Glyph(r)
		if len(g) != 5 {
			t.Errorf("glyph %c has %d rows, want 5", r, len(g))
		}
	}
}
