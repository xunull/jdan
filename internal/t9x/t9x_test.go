package t9x

import (
	"encoding/json"
	"strings"
	"testing"
)

func TestLetterDigit(t *testing.T) {
	cases := map[byte]byte{
		'a': '2', 'c': '2', 'd': '3', 'g': '4', 'j': '5',
		'm': '6', 'p': '7', 's': '7', 't': '8', 'v': '8',
		'w': '9', 'z': '9', 'A': '2', 'Z': '9',
	}
	for in, want := range cases {
		if got, ok := LetterDigit(in); !ok || got != want {
			t.Errorf("LetterDigit(%c) = %c,%v, want %c", in, got, ok, want)
		}
	}
	for _, bad := range []byte{'1', '!', ' ', '0'} {
		if _, ok := LetterDigit(bad); ok {
			t.Errorf("LetterDigit(%q) 应为 false", string(bad))
		}
	}
}

func TestLettersToDigits(t *testing.T) {
	cases := map[string]string{
		"zhong": "94664", // 中
		"wen":   "936",   // 文
		"hello": "43556",
		"HELLO": "43556", // 大小写无关
		"":      "",
	}
	for in, want := range cases {
		if got := LettersToDigits(in); got != want {
			t.Errorf("LettersToDigits(%q) = %q, want %q", in, got, want)
		}
	}
}

func TestResult_DigitString(t *testing.T) {
	r := Result{Units: []Unit{
		{Text: "中", Pinyin: "zhong", Digits: "94664"},
		{Text: "文", Pinyin: "wen", Digits: "936"},
		{Text: "hi", Digits: "44"},
	}}
	if got := r.DigitString(); got != "94664 936 44" {
		t.Errorf("DigitString = %q", got)
	}
}

func TestResult_Render(t *testing.T) {
	r := Result{Units: []Unit{
		{Text: "中", Pinyin: "zhong", Digits: "94664"},
		{Text: "hi", Digits: "44"},
	}}
	s := r.Render()
	if !strings.Contains(s, "zhong") || !strings.Contains(s, "94664") {
		t.Errorf("缺汉字行:\n%s", s)
	}
	if !strings.Contains(s, "—") { // 英文单元拼音列显示 —
		t.Errorf("英文单元应显示 — 占位:\n%s", s)
	}
	if !strings.Contains(s, "94664 44") { // 底部整串
		t.Errorf("缺底部数字串:\n%s", s)
	}
}

func TestResult_Render_Empty(t *testing.T) {
	if s := (Result{}).Render(); s != "" {
		t.Errorf("空结果应渲染空串，得 %q", s)
	}
}

func TestFormatJSON(t *testing.T) {
	r := Result{Units: []Unit{{Text: "中", Pinyin: "zhong", Digits: "94664"}}, Skipped: 2}
	s, err := r.FormatJSON()
	if err != nil {
		t.Fatal(err)
	}
	var v map[string]any
	if err := json.Unmarshal([]byte(s), &v); err != nil {
		t.Fatalf("非法 json:\n%s", s)
	}
	if v["skipped"].(float64) != 2 {
		t.Errorf("skipped 应为 2: %v", v["skipped"])
	}
}

func TestFormatJSON_EmptyUnitsNotNull(t *testing.T) {
	s, _ := (Result{}).FormatJSON()
	if !strings.Contains(s, `"units": []`) {
		t.Errorf("空 units 应为 []，得:\n%s", s)
	}
}
