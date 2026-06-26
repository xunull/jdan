package entropyx

import (
	"encoding/json"
	"math"
	"strings"
	"testing"
)

func approx(a, b, eps float64) bool { return math.Abs(a-b) <= eps }

// ---- Shannon 已知向量 ----

func TestShannon_Empty(t *testing.T) {
	if h := Shannon(nil); h != 0 {
		t.Errorf("empty → %v, want 0", h)
	}
}

func TestShannon_SingleValue(t *testing.T) {
	if h := Shannon([]byte("aaaaaa")); h != 0 {
		t.Errorf("single distinct byte → %v, want 0", h)
	}
}

func TestShannon_TwoValuesHalf(t *testing.T) {
	// 两个值各占一半 → 恰好 1.0 bit/byte
	if h := Shannon([]byte{0, 1, 0, 1}); !approx(h, 1.0, 1e-9) {
		t.Errorf("two-value half → %v, want 1.0", h)
	}
}

func TestShannon_UniformAll256(t *testing.T) {
	data := make([]byte, 256)
	for i := range data {
		data[i] = byte(i)
	}
	if h := Shannon(data); !approx(h, 8.0, 1e-9) {
		t.Errorf("uniform 256 → %v, want 8.0", h)
	}
}

func TestShannon_KnownText(t *testing.T) {
	if h := Shannon([]byte("hello world")); !approx(h, 2.845, 0.01) {
		t.Errorf("'hello world' → %v, want ~2.845", h)
	}
}

// ---- Analyze ----

func TestAnalyze_Overall(t *testing.T) {
	r := Analyze([]byte("aaaa"), 0)
	if r.Bytes != 4 || r.Distinct != 1 || r.BitsPerByte != 0 {
		t.Errorf("%+v", r)
	}
	if r.TotalBits != 0 {
		t.Errorf("total bits = %v", r.TotalBits)
	}
	if len(r.Chunks) != 0 {
		t.Error("window=0 should produce no chunks")
	}
}

func TestAnalyze_Windows(t *testing.T) {
	data := make([]byte, 50)
	r := Analyze(data, 16)
	// 50 / 16 = 3 满块 + 1 尾块 = 4 块
	if len(r.Chunks) != 4 {
		t.Errorf("want 4 chunks, got %d", len(r.Chunks))
	}
}

func TestAnalyze_Peak(t *testing.T) {
	// 前 16B 全同（熵 0），后 16B 两值交替（熵 1）→ 峰值在偏移 16
	data := append(make([]byte, 16), []byte{0, 1, 0, 1, 0, 1, 0, 1, 0, 1, 0, 1, 0, 1, 0, 1}...)
	r := Analyze(data, 16)
	if r.PeakOffset != 16 {
		t.Errorf("peak offset = %d, want 16", r.PeakOffset)
	}
	if !approx(r.PeakValue, 1.0, 1e-9) {
		t.Errorf("peak value = %v, want 1.0", r.PeakValue)
	}
}

// ---- 标签分档 ----

func TestLabels(t *testing.T) {
	cases := []struct {
		h    float64
		want string
	}{
		{0.5, "极低"}, {2, "低"}, {5, "中"}, {7, "高"}, {7.9, "极高"},
	}
	for _, c := range cases {
		if got := label(c.h); !strings.HasPrefix(got, c.want) {
			t.Errorf("label(%v) = %q, want prefix %q", c.h, got, c.want)
		}
	}
}

// ---- Sparkline ----

func TestSparkline_Monotonic(t *testing.T) {
	s := Sparkline([]float64{0, 8})
	r := []rune(s)
	if len(r) != 2 || r[0] != '▁' || r[1] != '█' {
		t.Errorf("sparkline boundaries wrong: %q", s)
	}
}

func TestSparkline_Increasing(t *testing.T) {
	s := []rune(Sparkline([]float64{0, 2, 4, 6, 8}))
	for i := 1; i < len(s); i++ {
		if s[i] < s[i-1] {
			t.Errorf("sparkline should be non-decreasing for increasing input: %s", string(s))
		}
	}
}

// ---- CharsetBits ----

func TestCharsetBits(t *testing.T) {
	if size, _ := CharsetBits("abc"); size != 26 {
		t.Errorf("lowercase → %d, want 26", size)
	}
	if size, _ := CharsetBits("abc123"); size != 36 {
		t.Errorf("lower+digit → %d, want 36", size)
	}
	if size, bits := CharsetBits("aB3$"); size != 26+26+10+32 || !approx(bits, 4*math.Log2(94), 0.01) {
		t.Errorf("all classes → size %d bits %v", size, bits)
	}
	if size, _ := CharsetBits(""); size != 0 {
		t.Errorf("empty → %d, want 0", size)
	}
}

// ---- 输出 ----

func TestFormatJSON(t *testing.T) {
	r := Analyze([]byte("hello"), 2)
	r.Charset, r.CharsetBits = CharsetBits("hello")
	s, err := r.FormatJSON()
	if err != nil {
		t.Fatal(err)
	}
	var v map[string]any
	if err := json.Unmarshal([]byte(s), &v); err != nil {
		t.Fatalf("invalid JSON: %v\n%s", err, s)
	}
	for _, k := range []string{"bits_per_byte", "label", "window", "chunks", "charset_bits"} {
		if _, ok := v[k]; !ok {
			t.Errorf("JSON missing key %q", k)
		}
	}
}

func TestFormatText_Charset(t *testing.T) {
	r := Analyze([]byte("hello"), 0)
	if strings.Contains(r.FormatText(false), "charset") {
		t.Error("charset line should be hidden when not requested")
	}
	r.Charset, r.CharsetBits = CharsetBits("hello")
	if !strings.Contains(r.FormatText(true), "charset") {
		t.Error("charset line should show when requested")
	}
}
