package cli

import (
	"bytes"
	"encoding/json"
	"strings"
	"testing"
)

func runPinyin(t *testing.T, stdin string, args ...string) (string, error) {
	t.Helper()
	var out bytes.Buffer
	cmd := newPinyinCommand(pinyinDeps{out: &out, in: strings.NewReader(stdin)})
	cmd.SetArgs(args)
	err := cmd.Execute()
	return out.String(), err
}

func TestPinyin_DefaultTone(t *testing.T) {
	out, err := runPinyin(t, "", "中文")
	if err != nil {
		t.Fatal(err)
	}
	if strings.TrimSpace(out) != "zhōng wén" {
		t.Errorf("中文 → %q, want zhōng wén", strings.TrimSpace(out))
	}
}

func TestPinyin_Styles(t *testing.T) {
	cases := map[string]string{"plain": "zhong wen", "num": "zhong1 wen2"}
	for style, want := range cases {
		out, err := runPinyin(t, "", "--style", style, "中文")
		if err != nil {
			t.Fatal(err)
		}
		if strings.TrimSpace(out) != want {
			t.Errorf("[%s] → %q, want %q", style, strings.TrimSpace(out), want)
		}
	}
}

func TestPinyin_NonHanPassthrough(t *testing.T) {
	out, err := runPinyin(t, "", "Hello 世界 2024")
	if err != nil {
		t.Fatal(err)
	}
	if strings.TrimSpace(out) != "Hello shì jiè 2024" {
		t.Errorf("→ %q, want Hello shì jiè 2024", strings.TrimSpace(out))
	}
}

func TestPinyin_Sep(t *testing.T) {
	out, err := runPinyin(t, "", "--style", "plain", "--sep", "-", "中文")
	if err != nil {
		t.Fatal(err)
	}
	if strings.TrimSpace(out) != "zhong-wen" {
		t.Errorf("→ %q, want zhong-wen", strings.TrimSpace(out))
	}
}

func TestPinyin_Heteronym(t *testing.T) {
	out, err := runPinyin(t, "", "--style", "plain", "--heteronym", "行")
	if err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(out, "xing") || !strings.Contains(out, "hang") {
		t.Errorf("行 --heteronym 应含 xing 和 hang: %q", out)
	}
}

func TestPinyin_Stdin(t *testing.T) {
	out, err := runPinyin(t, "你好\n")
	if err != nil {
		t.Fatal(err)
	}
	if strings.TrimSpace(out) != "nǐ hǎo" {
		t.Errorf("你好 → %q, want nǐ hǎo", strings.TrimSpace(out))
	}
}

func TestPinyin_BadStyle(t *testing.T) {
	if _, err := runPinyin(t, "", "--style", "nope", "中"); err == nil {
		t.Error("非法样式应报错")
	}
}

func TestPinyin_JSON(t *testing.T) {
	out, err := runPinyin(t, "", "--json", "中A")
	if err != nil {
		t.Fatal(err)
	}
	var v struct {
		Input, Style, Result string
		Tokens               []struct {
			Text   string
			Han    bool
			Pinyin []string
		}
	}
	if err := json.Unmarshal([]byte(out), &v); err != nil {
		t.Fatalf("非法 json:\n%s", out)
	}
	if v.Style != "tone" || len(v.Tokens) != 2 || !v.Tokens[0].Han || v.Tokens[1].Han {
		t.Errorf("json 结构不对: %+v", v)
	}
}
