package pinyinx

import (
	"strings"
	"testing"
)

func conv(text, style string) string {
	return Join(Convert(text, Options{Style: style}), " ")
}

func TestConvert_Styles(t *testing.T) {
	// tone/num/plain 用「中文」；initials/first 用「北京」避开零声母（文=w 无声母）
	cases := []struct{ style, in, want string }{
		{"tone", "中文", "zhōng wén"},
		{"num", "中文", "zhong1 wen2"},
		{"plain", "中文", "zhong wen"},
		{"initials", "北京", "b j"},
		{"first", "北京", "b j"},
	}
	for _, c := range cases {
		if got := conv(c.in, c.style); got != c.want {
			t.Errorf("[%s] %s → %q, want %q", c.style, c.in, got, c.want)
		}
	}
}

func TestConvert_InitialsZeroShengmu(t *testing.T) {
	// 文/王 等零声母字在 initials 样式下声母为空（go-pinyin 的真实数据）
	toks := Convert("文", Options{Style: "initials"})
	if len(toks) != 1 || len(toks[0].Pinyin) != 1 || toks[0].Pinyin[0] != "" {
		t.Errorf("文 声母应为空: %+v", toks)
	}
}

func TestConvert_NonHanPassthrough(t *testing.T) {
	// 非汉字原样穿插，句子结构保住
	got := conv("Hello 世界 2024", "tone")
	if got != "Hello shì jiè 2024" {
		t.Errorf("→ %q, want %q", got, "Hello shì jiè 2024")
	}
}

func TestConvert_NonHanTokens(t *testing.T) {
	toks := Convert("abc中", Options{Style: "plain"})
	if len(toks) != 2 || toks[0].Han || toks[0].Text != "abc" || !toks[1].Han {
		t.Fatalf("分词不对: %+v", toks)
	}
	if len(toks[1].Pinyin) != 1 || toks[1].Pinyin[0] != "zhong" {
		t.Errorf("汉字读音不对: %+v", toks[1])
	}
}

func TestConvert_Heteronym(t *testing.T) {
	// 行 有 xing/hang 多读音
	toks := Convert("行", Options{Style: "plain", Heteronym: true})
	if len(toks) != 1 || len(toks[0].Pinyin) < 2 {
		t.Fatalf("多音字应有多个读音: %+v", toks)
	}
	joined := strings.Join(toks[0].Pinyin, "/")
	if !strings.Contains(joined, "xing") || !strings.Contains(joined, "hang") {
		t.Errorf("行 读音应含 xing 和 hang: %q", joined)
	}
}

func TestJoin_Separator(t *testing.T) {
	toks := Convert("中文", Options{Style: "plain"})
	if got := Join(toks, "-"); got != "zhong-wen" {
		t.Errorf("sep=- → %q, want zhong-wen", got)
	}
	if got := Join(toks, ""); got != "zhongwen" {
		t.Errorf("sep='' → %q, want zhongwen", got)
	}
}

func TestPlain(t *testing.T) {
	if got := Plain('中'); got != "zhong" {
		t.Errorf("Plain(中) = %q, want zhong", got)
	}
	if got := Plain('A'); got != "" {
		t.Errorf("Plain(A) = %q, want empty", got)
	}
}

func TestStyleValid(t *testing.T) {
	for _, s := range StyleNames() {
		if !StyleValid(s) {
			t.Errorf("%q 应合法", s)
		}
	}
	if StyleValid("nope") {
		t.Error("nope 应非法")
	}
}
