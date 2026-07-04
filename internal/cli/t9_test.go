package cli

import (
	"bytes"
	"encoding/json"
	"strings"
	"testing"
)

var demoPinyin = map[rune]string{'中': "zhong", '文': "wen", '你': "ni", '好': "hao", '国': "guo"}

func fakePinyin(r rune) string { return demoPinyin[r] }

func runT9(t *testing.T, pinyinOf func(rune) string, stdin string, args ...string) (string, string, error) {
	t.Helper()
	var o, e bytes.Buffer
	deps := t9Deps{out: &o, errOut: &e, in: strings.NewReader(stdin), pinyinOf: pinyinOf}
	cmd := newT9Command(deps)
	cmd.SetArgs(args)
	err := cmd.Execute()
	return o.String(), e.String(), err
}

func TestT9_Chinese(t *testing.T) {
	out, _, err := runT9(t, fakePinyin, "", "中文")
	if err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(out, "zhong") || !strings.Contains(out, "94664") {
		t.Errorf("应有 中→zhong→94664:\n%s", out)
	}
	if !strings.Contains(out, "94664 936") {
		t.Errorf("底部整串应为 94664 936:\n%s", out)
	}
}

func TestT9_MixedCNEN(t *testing.T) {
	// 中英混排：你好 + hi
	out, _, err := runT9(t, fakePinyin, "", "你好 hi")
	if err != nil {
		t.Fatal(err)
	}
	// ni=64, hao=426, hi=44
	if !strings.Contains(out, "64 426 44") {
		t.Errorf("混排整串应为 64 426 44:\n%s", out)
	}
}

func TestT9_EnglishOnly(t *testing.T) {
	out, _, err := runT9(t, fakePinyin, "", "--digits", "hello")
	if err != nil {
		t.Fatal(err)
	}
	if strings.TrimSpace(out) != "43556" {
		t.Errorf("hello → %q, want 43556", strings.TrimSpace(out))
	}
}

func TestT9_DigitsPassthroughAndPunctSkipped(t *testing.T) {
	// 阿拉伯数字原样，标点静默跳过
	out, errOut, err := runT9(t, fakePinyin, "", "--digits", "中5!文")
	if err != nil {
		t.Fatal(err)
	}
	if strings.TrimSpace(out) != "94664 5 936" {
		t.Errorf("→ %q, want 94664 5 936", strings.TrimSpace(out))
	}
	if strings.Contains(errOut, "跳过") {
		t.Errorf("标点不应计入跳过: %q", errOut)
	}
}

func TestT9_UnknownCounted(t *testing.T) {
	// 天 不在 fake 字典 → 计为跳过并提示
	out, errOut, err := runT9(t, fakePinyin, "", "中天")
	if err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(out, "94664") {
		t.Errorf("中 仍应映射:\n%s", out)
	}
	if !strings.Contains(errOut, "跳过 1") {
		t.Errorf("天 应计 1 个跳过: %q", errOut)
	}
}

func TestT9_Stdin(t *testing.T) {
	out, _, err := runT9(t, fakePinyin, "中国\n", "--digits")
	if err != nil {
		t.Fatal(err)
	}
	if strings.TrimSpace(out) != "94664 486" { // 中=94664 国(guo=g·u·o)=486
		t.Errorf("stdin → %q, want 94664 486", strings.TrimSpace(out))
	}
}

func TestT9_JSON(t *testing.T) {
	out, _, err := runT9(t, fakePinyin, "", "--json", "中")
	if err != nil {
		t.Fatal(err)
	}
	var v struct {
		Units []struct {
			Text, Pinyin, Digits string
		}
		Skipped int
	}
	if err := json.Unmarshal([]byte(out), &v); err != nil {
		t.Fatalf("非法 json:\n%s", out)
	}
	if len(v.Units) != 1 || v.Units[0].Digits != "94664" || v.Units[0].Pinyin != "zhong" {
		t.Errorf("json 单元不对: %+v", v.Units)
	}
}

func TestT9_EmptyInput(t *testing.T) {
	if _, _, err := runT9(t, fakePinyin, "", "   "); err == nil {
		t.Error("空输入应报错")
	}
}

// 集成：用真实 go-pinyin（字典内嵌、离线可跑），证明汉字→拼音→数字这条链通。
func TestT9_RealPinyin(t *testing.T) {
	out, _, err := runT9(t, nil, "", "--digits", "中文") // nil → 默认 realPinyin
	if err != nil {
		t.Fatal(err)
	}
	if strings.TrimSpace(out) != "94664 936" {
		t.Errorf("真实拼音 中文 → %q, want 94664 936", strings.TrimSpace(out))
	}
}
