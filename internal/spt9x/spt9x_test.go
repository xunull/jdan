package spt9x

import (
	"encoding/json"
	"strings"
	"testing"
)

func TestEncode_WithInitial(t *testing.T) {
	// 声母 + 韵母（照 RIME flypy 核对）
	cases := map[string]string{
		"dan":    "dj", // d + an(j)   ← 官方实例：键 3+5
		"zhong":  "vs", // zh(v) + ong(s)
		"wen":    "wf", // w + en(f)
		"ni":     "ni", // n + i
		"hao":    "hc", // h + ao(c)
		"guo":    "go", // g + uo(o)
		"xiang":  "xl", // x + iang(l)
		"zhuang": "vl", // zh(v) + uang(l)
		"lve":    "lt", // l + üe(ve→t)
		"dui":    "dv", // d + ui(v)
		"ping":   "pk", // p + ing(k)
		"chi":    "ii", // ch(i) + i
		"shu":    "uu", // sh(u) + u
	}
	for py, want := range cases {
		if got, ok := Encode(py); !ok || got != want {
			t.Errorf("Encode(%q) = %q,%v, want %q", py, got, ok, want)
		}
	}
}

func TestEncode_ZeroInitial(t *testing.T) {
	// 零声母：首字母 + 韵母键
	cases := map[string]string{
		"an":  "aj", // a + an(j)
		"ai":  "ad", // a + ai(d)
		"ao":  "ac", // a + ao(c)
		"e":   "ee", // e + e
		"a":   "aa",
		"o":   "oo",
		"ou":  "oz", // o + ou(z)
		"ei":  "ew", // e + ei(w)
		"ang": "ah", // a + ang(h)
		"er":  "er", // 特例
	}
	for py, want := range cases {
		if got, ok := Encode(py); !ok || got != want {
			t.Errorf("Encode(%q) = %q,%v, want %q", py, got, ok, want)
		}
	}
}

func TestEncode_Unknown(t *testing.T) {
	for _, bad := range []string{"", "xyz", "b", "  "} {
		if _, ok := Encode(bad); ok {
			t.Errorf("Encode(%q) 应为 false", bad)
		}
	}
}

func TestResult_DigitString(t *testing.T) {
	r := Result{Units: []Unit{
		{Text: "中", Pinyin: "zhong", Code: "vs", Digits: "87"},
		{Text: "文", Pinyin: "wen", Code: "wf", Digits: "93"},
	}}
	if got := r.DigitString(); got != "87 93" {
		t.Errorf("DigitString = %q, want 87 93", got)
	}
}

func TestResult_Render(t *testing.T) {
	r := Result{Units: []Unit{
		{Text: "中", Pinyin: "zhong", Code: "vs", Digits: "87"},
		{Text: "hi", Digits: "44"},
	}}
	s := r.Render()
	if !strings.Contains(s, "zhong") || !strings.Contains(s, "vs") || !strings.Contains(s, "87") {
		t.Errorf("汉字行缺列:\n%s", s)
	}
	if !strings.Contains(s, "87 44") {
		t.Errorf("缺底部整串:\n%s", s)
	}
}

func TestFormatJSON(t *testing.T) {
	r := Result{Units: []Unit{{Text: "中", Pinyin: "zhong", Code: "vs", Digits: "87"}}}
	s, err := r.FormatJSON()
	if err != nil {
		t.Fatal(err)
	}
	var v struct {
		Units []struct{ Text, Pinyin, Code, Digits string }
	}
	if err := json.Unmarshal([]byte(s), &v); err != nil {
		t.Fatalf("非法 json:\n%s", s)
	}
	if len(v.Units) != 1 || v.Units[0].Code != "vs" || v.Units[0].Digits != "87" {
		t.Errorf("json 单元不对: %+v", v.Units)
	}
}
