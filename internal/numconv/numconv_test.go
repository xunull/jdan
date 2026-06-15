package numconv

import (
	"reflect"
	"testing"
)

// ---- DetectBase ----

func TestDetectBase(t *testing.T) {
	cases := []struct {
		in       string
		wantBase Base
		wantRest string
	}{
		{"255", BaseDec, "255"},
		{"0xFF", BaseHex, "FF"},
		{"0XFF", BaseHex, "FF"},
		{"0b1010", BaseBin, "1010"},
		{"0B1010", BaseBin, "1010"},
		{"0o755", BaseOct, "755"},
		{"0O755", BaseOct, "755"},
		{"0755", BaseOct, "755"}, // C 风格八进制
		{"0", BaseDec, "0"},      // 单个 0 是十进制
	}
	for _, c := range cases {
		b, rest := DetectBase(c.in)
		if b != c.wantBase || rest != c.wantRest {
			t.Errorf("DetectBase(%q) = (%v, %q), want (%v, %q)", c.in, b, rest, c.wantBase, c.wantRest)
		}
	}
}

// ---- ParseValue ----

func TestParseValue_AllBases(t *testing.T) {
	cases := map[string]uint64{
		"255":        255,
		"0xFF":       255,
		"0b11111111": 255,
		"0o377":      255,
		"0755":       493,
		"0":          0,
	}
	for in, want := range cases {
		got, _, err := ParseValue(in)
		if err != nil {
			t.Errorf("ParseValue(%q) errored: %v", in, err)
			continue
		}
		if got != want {
			t.Errorf("ParseValue(%q) = %d, want %d", in, got, want)
		}
	}
}

func TestParseValue_MaxUint64(t *testing.T) {
	got, _, err := ParseValue("18446744073709551615")
	if err != nil {
		t.Fatal(err)
	}
	if got != ^uint64(0) {
		t.Errorf("got %d, want max uint64", got)
	}
}

func TestParseValue_Overflow(t *testing.T) {
	_, _, err := ParseValue("18446744073709551616") // max+1
	if err == nil {
		t.Error("overflow should error")
	}
}

func TestParseValue_Negative(t *testing.T) {
	if _, _, err := ParseValue("-5"); err == nil {
		t.Error("negative should error")
	}
}

func TestParseValue_Invalid(t *testing.T) {
	if _, _, err := ParseValue("0xGG"); err == nil {
		t.Error("invalid hex should error")
	}
	if _, _, err := ParseValue("0x"); err == nil {
		t.Error("empty after prefix should error")
	}
}

func TestParseValue_Underscores(t *testing.T) {
	got, _, err := ParseValue("0xFF_FF")
	if err != nil {
		t.Fatal(err)
	}
	if got != 0xFFFF {
		t.Errorf("got %d, want 65535", got)
	}
}

// ---- Convert ----

func TestConvert(t *testing.T) {
	r := Convert(255)
	if r.Decimal != 255 {
		t.Errorf("decimal = %d", r.Decimal)
	}
	if r.Hex != "0xFF" {
		t.Errorf("hex = %s", r.Hex)
	}
	if r.Binary != "0b11111111" {
		t.Errorf("binary = %s", r.Binary)
	}
	if r.Octal != "0o377" {
		t.Errorf("octal = %s", r.Octal)
	}
	if r.BitsSet != 8 {
		t.Errorf("bits_set = %d, want 8", r.BitsSet)
	}
	if r.BitWidth != 8 {
		t.Errorf("bit_width = %d, want 8", r.BitWidth)
	}
}

func TestConvert_Zero(t *testing.T) {
	r := Convert(0)
	if r.Hex != "0x0" || r.Binary != "0b0" || r.Octal != "0o0" {
		t.Errorf("zero conversion wrong: %+v", r)
	}
	if r.BitsSet != 0 || r.BitWidth != 0 {
		t.Errorf("zero bits: set=%d width=%d", r.BitsSet, r.BitWidth)
	}
}

func TestConvert_BitWidth(t *testing.T) {
	cases := map[uint64]int{
		0:          0,
		1:          1,
		2:          2,
		255:        8,
		256:        9,
		0x80000000: 32,
	}
	for v, want := range cases {
		if got := Convert(v).BitWidth; got != want {
			t.Errorf("BitWidth(%d) = %d, want %d", v, got, want)
		}
	}
}

// ---- SetBits ----

func TestSetBits(t *testing.T) {
	got := SetBits(0b10110)
	want := []int{1, 2, 4}
	if !reflect.DeepEqual(got, want) {
		t.Errorf("SetBits = %v, want %v", got, want)
	}
	if SetBits(0) != nil {
		t.Errorf("SetBits(0) should be nil")
	}
}

// ---- BinaryPadded ----

func TestBinaryPadded(t *testing.T) {
	if got := BinaryPadded(0xFF, 16); got != "0b0000000011111111" {
		t.Errorf("got %s", got)
	}
	// width 小于实际位数时不截断
	if got := BinaryPadded(0xFF, 4); got != "0b11111111" {
		t.Errorf("got %s", got)
	}
}

// ---- EvalBitExpr 二元 ----

func TestEvalBitExpr_Binary(t *testing.T) {
	cases := []struct {
		expr string
		want uint64
	}{
		{"0xFF AND 0x0F", 0x0F},
		{"5 OR 2", 7},
		{"0b1010 XOR 0b0110", 0b1100},
		{"1 << 8", 256},
		{"0xFF00 >> 4", 0x0FF0},
		{"255 & 15", 15},
		{"5 | 2", 7},
		{"12 ^ 10", 6},
	}
	for _, c := range cases {
		r, err := EvalBitExpr(c.expr, 64)
		if err != nil {
			t.Errorf("EvalBitExpr(%q) errored: %v", c.expr, err)
			continue
		}
		if r.Result != c.want {
			t.Errorf("EvalBitExpr(%q) = %d, want %d", c.expr, r.Result, c.want)
		}
	}
}

func TestEvalBitExpr_TightTokens(t *testing.T) {
	// 紧贴的运算符也要切对
	r, err := EvalBitExpr("1<<4", 64)
	if err != nil {
		t.Fatal(err)
	}
	if r.Result != 16 {
		t.Errorf("1<<4 = %d, want 16", r.Result)
	}
}

func TestEvalBitExpr_ShiftOverflow(t *testing.T) {
	// 移位 >= 64 应当得 0（不 panic）
	r, _ := EvalBitExpr("1 << 64", 64)
	if r.Result != 0 {
		t.Errorf("1<<64 = %d, want 0", r.Result)
	}
}

// ---- EvalBitExpr 单目 NOT ----

func TestEvalBitExpr_NOT_Width8(t *testing.T) {
	r, err := EvalBitExpr("NOT 0xFF", 8)
	if err != nil {
		t.Fatal(err)
	}
	if r.Result != 0 {
		t.Errorf("NOT 0xFF (8-bit) = %d, want 0", r.Result)
	}
}

func TestEvalBitExpr_NOT_Width64(t *testing.T) {
	r, _ := EvalBitExpr("NOT 0", 64)
	if r.Result != ^uint64(0) {
		t.Errorf("NOT 0 (64-bit) = %d, want max uint64", r.Result)
	}
}

func TestEvalBitExpr_NOT_SymbolAlias(t *testing.T) {
	r, err := EvalBitExpr("~ 0x0F", 8)
	if err != nil {
		t.Fatal(err)
	}
	if r.Result != 0xF0 {
		t.Errorf("~0x0F (8-bit) = %#x, want 0xF0", r.Result)
	}
}

// ---- EvalBitExpr 错误 ----

func TestEvalBitExpr_UnknownOp(t *testing.T) {
	if _, err := EvalBitExpr("5 FOO 3", 64); err == nil {
		t.Error("unknown operator should error")
	}
}

func TestEvalBitExpr_WrongTokenCount(t *testing.T) {
	if _, err := EvalBitExpr("5", 64); err == nil {
		t.Error("single token should error")
	}
	if _, err := EvalBitExpr("5 AND 3 OR 2", 64); err == nil {
		t.Error("too many tokens should error")
	}
}

func TestEvalBitExpr_NOTBinaryMisuse(t *testing.T) {
	// "5 NOT 3" 不合法（NOT 是单目）
	if _, err := EvalBitExpr("5 NOT 3", 64); err == nil {
		t.Error("NOT as binary should error")
	}
}

func TestEvalBitExpr_BadOperand(t *testing.T) {
	if _, err := EvalBitExpr("0xGG AND 1", 64); err == nil {
		t.Error("bad operand should error")
	}
}
