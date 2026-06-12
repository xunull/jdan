package qrcode

import (
	"bytes"
	"encoding/json"
	"image/png"
	"strings"
	"testing"
)

func TestRenderTerminal_Deterministic(t *testing.T) {
	a, err := RenderTerminal("hello world", Options{ECC: Medium})
	if err != nil {
		t.Fatal(err)
	}
	b, err := RenderTerminal("hello world", Options{ECC: Medium})
	if err != nil {
		t.Fatal(err)
	}
	if a != b {
		t.Error("same input should produce same output")
	}
}

func TestRenderTerminal_UsesHalfBlock(t *testing.T) {
	s, err := RenderTerminal("test", Options{ECC: Medium})
	if err != nil {
		t.Fatal(err)
	}
	// 半角块至少出现一种：▀ ▄ █
	if !strings.ContainsAny(s, "▀▄█") {
		t.Errorf("expected half-block chars in output")
	}
}

func TestRenderTerminal_FullBlock(t *testing.T) {
	s, err := RenderTerminal("test", Options{ECC: Medium, FullBlock: true})
	if err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(s, "██") {
		t.Error("full-block mode should emit ██")
	}
	// 全角模式下不应该出现半角字符
	for _, ch := range []rune{'▀', '▄'} {
		if strings.ContainsRune(s, ch) {
			t.Errorf("full-block mode leaked half-block %q", string(ch))
		}
	}
}

func TestRenderTerminal_Invert(t *testing.T) {
	normal, err := RenderTerminal("x", Options{ECC: Medium})
	if err != nil {
		t.Fatal(err)
	}
	inverted, err := RenderTerminal("x", Options{ECC: Medium, Invert: true})
	if err != nil {
		t.Fatal(err)
	}
	if normal == inverted {
		t.Error("invert should change output")
	}
}

func TestRenderTerminal_RejectsEmpty(t *testing.T) {
	if _, err := RenderTerminal("", Options{}); err == nil {
		t.Error("empty input should error")
	}
}

func TestRenderTerminal_HasQuietZonePadding(t *testing.T) {
	// 输出第一行和最后一行应当是纯空格（quiet zone padding）
	s, err := RenderTerminal("x", Options{ECC: Medium})
	if err != nil {
		t.Fatal(err)
	}
	lines := strings.Split(strings.TrimRight(s, "\n"), "\n")
	if len(lines) < 3 {
		t.Fatalf("too few lines: %d", len(lines))
	}
	first, last := lines[0], lines[len(lines)-1]
	if strings.TrimSpace(first) != "" {
		t.Errorf("first line should be quiet-zone space, got %q", first)
	}
	if strings.TrimSpace(last) != "" {
		t.Errorf("last line should be quiet-zone space, got %q", last)
	}
}

func TestRenderPNG_ValidPNG(t *testing.T) {
	b, err := RenderPNG("hello", Options{ECC: Medium}, 200)
	if err != nil {
		t.Fatal(err)
	}
	img, err := png.Decode(bytes.NewReader(b))
	if err != nil {
		t.Errorf("output is not valid PNG: %v", err)
	}
	if img.Bounds().Dx() != 200 || img.Bounds().Dy() != 200 {
		t.Errorf("expected 200x200 PNG, got %dx%d", img.Bounds().Dx(), img.Bounds().Dy())
	}
}

func TestRenderPNG_DefaultSize(t *testing.T) {
	b, err := RenderPNG("hello", Options{ECC: Medium}, 0)
	if err != nil {
		t.Fatal(err)
	}
	img, err := png.Decode(bytes.NewReader(b))
	if err != nil {
		t.Fatal(err)
	}
	if img.Bounds().Dx() != 256 {
		t.Errorf("default size should be 256, got %d", img.Bounds().Dx())
	}
}

func TestRenderPNG_RejectsEmpty(t *testing.T) {
	if _, err := RenderPNG("", Options{}, 256); err == nil {
		t.Error("empty input should error")
	}
}

func TestRenderSVG_ValidStructure(t *testing.T) {
	s, err := RenderSVG("hello world", Options{ECC: Medium}, 8)
	if err != nil {
		t.Fatal(err)
	}
	if !strings.HasPrefix(s, "<?xml") {
		t.Error("SVG missing XML declaration")
	}
	if !strings.Contains(s, "<svg") {
		t.Error("SVG missing <svg> root")
	}
	if !strings.HasSuffix(s, "</svg>") {
		t.Error("SVG missing </svg> close")
	}
	if !strings.Contains(s, `fill="#000000"`) {
		t.Error("SVG missing black fill for foreground modules")
	}
	if !strings.Contains(s, `fill="#ffffff"`) {
		t.Error("SVG missing white background")
	}
}

func TestRenderSVG_Invert(t *testing.T) {
	s, err := RenderSVG("hello", Options{ECC: Medium, Invert: true}, 8)
	if err != nil {
		t.Fatal(err)
	}
	// 反色模式下：背景黑、前景白
	if !strings.Contains(s, `fill="#000000"`) {
		t.Error("inverted SVG missing black background")
	}
	if !strings.Contains(s, `fill="#ffffff"`) {
		t.Error("inverted SVG missing white foreground")
	}
}

func TestRenderSVG_RejectsEmpty(t *testing.T) {
	if _, err := RenderSVG("", Options{}, 8); err == nil {
		t.Error("empty input should error")
	}
}

func TestECCLevels_AffectModuleCount(t *testing.T) {
	// 同样数据 + 更高 ECC → 需要更多 module（更大的 QR 版本）
	low, err := Describe("hello world this is a longer test string", Options{ECC: Low})
	if err != nil {
		t.Fatal(err)
	}
	high, err := Describe("hello world this is a longer test string", Options{ECC: Highest})
	if err != nil {
		t.Fatal(err)
	}
	if high.Modules <= low.Modules {
		t.Errorf("higher ECC should yield more modules; L=%d H=%d", low.Modules, high.Modules)
	}
}

func TestParseECC(t *testing.T) {
	for _, tc := range []struct {
		in   string
		want ECC
	}{
		{"L", Low},
		{"l", Low},
		{" M ", Medium},
		{"Q", High},
		{"h", Highest},
	} {
		got, err := ParseECC(tc.in)
		if err != nil {
			t.Errorf("ParseECC(%q) error: %v", tc.in, err)
			continue
		}
		if got != tc.want {
			t.Errorf("ParseECC(%q) = %v, want %v", tc.in, got, tc.want)
		}
	}
	for _, bad := range []string{"X", "", "MM", "1"} {
		if _, err := ParseECC(bad); err == nil {
			t.Errorf("ParseECC(%q) should error", bad)
		}
	}
}

func TestDescribe_JSONSerializable(t *testing.T) {
	info, err := Describe("hello", Options{ECC: Medium})
	if err != nil {
		t.Fatal(err)
	}
	b, err := json.Marshal(info)
	if err != nil {
		t.Fatalf("Info not JSON-serializable: %v", err)
	}
	for _, want := range []string{`"data":"hello"`, `"ecc":"M"`, `"modules":`} {
		if !bytes.Contains(b, []byte(want)) {
			t.Errorf("JSON missing %s: %s", want, string(b))
		}
	}
	if info.Modules <= 0 {
		t.Errorf("Modules should be positive, got %d", info.Modules)
	}
}

func TestECCString_AllLevels(t *testing.T) {
	for _, tc := range []struct {
		e    ECC
		want string
	}{
		{Low, "L"},
		{Medium, "M"},
		{High, "Q"},
		{Highest, "H"},
	} {
		info, _ := Describe("x", Options{ECC: tc.e})
		if info.ECC != tc.want {
			t.Errorf("ECC %d string = %q, want %q", tc.e, info.ECC, tc.want)
		}
	}
}

func TestDefaultOptions(t *testing.T) {
	o := DefaultOptions()
	if o.ECC != Medium {
		t.Errorf("default ECC should be Medium, got %v", o.ECC)
	}
	if o.Invert {
		t.Error("default should not invert")
	}
	if o.FullBlock {
		t.Error("default should be half-block")
	}
}
