package cli

import (
	"bytes"
	"strings"
	"testing"
)

func runAlpha(t *testing.T, args ...string) (string, error) {
	t.Helper()
	var o bytes.Buffer
	cmd := newAlphaCommand(alphaDeps{out: &o})
	cmd.SetArgs(args)
	err := cmd.Execute()
	return o.String(), err
}

func TestRenderAlpha_Lowercase(t *testing.T) {
	s := renderAlpha(false)
	lines := strings.Split(strings.TrimRight(s, "\n"), "\n")
	if len(lines) != 2 {
		t.Fatalf("应为 2 行，得 %d:\n%s", len(lines), s)
	}
	// 第一行 26 个字母，第二行 1-26
	if fields := strings.Fields(lines[0]); len(fields) != 26 || fields[0] != "a" || fields[25] != "z" {
		t.Errorf("字母行不对: %q", lines[0])
	}
	if fields := strings.Fields(lines[1]); len(fields) != 26 || fields[0] != "1" || fields[25] != "26" {
		t.Errorf("序号行不对: %q", lines[1])
	}
	// 对齐：字母 j 的起始列 == 序号 10 的起始列
	if strings.Index(lines[0], "j") != strings.Index(lines[1], "10") {
		t.Errorf("j 应对齐在 10 正上方:\n%s", s)
	}
}

func TestRenderAlpha_Upper(t *testing.T) {
	s := renderAlpha(true)
	if !strings.HasPrefix(s, "A ") || !strings.Contains(s, " Z\n") {
		t.Errorf("大写模式不对:\n%s", s)
	}
}

func TestLetterToNum(t *testing.T) {
	cases := map[string]int{"a": 1, "z": 26, "A": 1, "Z": 26, "k": 11}
	for in, want := range cases {
		if got, ok := letterToNum(in); !ok || got != want {
			t.Errorf("letterToNum(%q) = %d,%v, want %d", in, got, ok, want)
		}
	}
	for _, bad := range []string{"", "ab", "1", "!", " "} {
		if _, ok := letterToNum(bad); ok {
			t.Errorf("letterToNum(%q) 应为 false", bad)
		}
	}
}

func TestNumToLetter(t *testing.T) {
	if l, ok := numToLetter(1, false); !ok || l != "a" {
		t.Errorf("1 → %q,%v want a", l, ok)
	}
	if l, ok := numToLetter(26, false); !ok || l != "z" {
		t.Errorf("26 → %q,%v want z", l, ok)
	}
	if l, ok := numToLetter(3, true); !ok || l != "C" {
		t.Errorf("3 upper → %q,%v want C", l, ok)
	}
	for _, bad := range []int{0, 27, -1} {
		if _, ok := numToLetter(bad, false); ok {
			t.Errorf("numToLetter(%d) 应为 false", bad)
		}
	}
}

func TestAlphaCmd_Table(t *testing.T) {
	out, err := runAlpha(t)
	if err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(out, "a b c") || !strings.Contains(out, "26") {
		t.Errorf("无参数应打印对照表:\n%s", out)
	}
}

func TestAlphaCmd_Lookup(t *testing.T) {
	if out, err := runAlpha(t, "k"); err != nil || strings.TrimSpace(out) != "11" {
		t.Errorf("alpha k → %q,%v want 11", out, err)
	}
	if out, err := runAlpha(t, "11"); err != nil || strings.TrimSpace(out) != "k" {
		t.Errorf("alpha 11 → %q,%v want k", out, err)
	}
	if out, err := runAlpha(t, "-u", "3"); err != nil || strings.TrimSpace(out) != "C" {
		t.Errorf("alpha -u 3 → %q,%v want C", out, err)
	}
}

func TestAlphaCmd_Errors(t *testing.T) {
	for _, bad := range []string{"0", "27", "!", "ab"} {
		if _, err := runAlpha(t, bad); err == nil {
			t.Errorf("alpha %q 应报错", bad)
		}
	}
}
