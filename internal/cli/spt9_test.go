package cli

import (
	"bytes"
	"encoding/json"
	"strings"
	"testing"
)

func runSPT9(t *testing.T, pinyinOf func(rune) string, stdin string, args ...string) (string, string, error) {
	t.Helper()
	var o, e bytes.Buffer
	deps := spt9Deps{out: &o, errOut: &e, in: strings.NewReader(stdin), pinyinOf: pinyinOf}
	cmd := newSPT9Command(deps)
	cmd.SetArgs(args)
	err := cmd.Execute()
	return o.String(), e.String(), err
}

// 复用 t9_test.go 的 demoPinyin/fakePinyin。

func TestSPT9_Chinese(t *testing.T) {
	// 中=zhong→vs→87, 文=wen→wf→93
	out, _, err := runSPT9(t, fakePinyin, "", "--digits", "中文")
	if err != nil {
		t.Fatal(err)
	}
	if strings.TrimSpace(out) != "87 93" {
		t.Errorf("中文 → %q, want 87 93", strings.TrimSpace(out))
	}
}

func TestSPT9_Table(t *testing.T) {
	out, _, err := runSPT9(t, fakePinyin, "", "中")
	if err != nil {
		t.Fatal(err)
	}
	// 逐字对照应含拼音 + 双拼两码 + 数字
	if !strings.Contains(out, "zhong") || !strings.Contains(out, "vs") || !strings.Contains(out, "87") {
		t.Errorf("对照表缺列:\n%s", out)
	}
}

func TestSPT9_MixedEnglish(t *testing.T) {
	// 英文按普通 T9：hi → 44
	out, _, err := runSPT9(t, fakePinyin, "", "--digits", "中 hi")
	if err != nil {
		t.Fatal(err)
	}
	if strings.TrimSpace(out) != "87 44" {
		t.Errorf("中 hi → %q, want 87 44", strings.TrimSpace(out))
	}
}

func TestSPT9_JSON(t *testing.T) {
	out, _, err := runSPT9(t, fakePinyin, "", "--json", "中")
	if err != nil {
		t.Fatal(err)
	}
	var v struct {
		Units []struct{ Text, Pinyin, Code, Digits string }
	}
	if err := json.Unmarshal([]byte(out), &v); err != nil {
		t.Fatalf("非法 json:\n%s", out)
	}
	if len(v.Units) != 1 || v.Units[0].Code != "vs" || v.Units[0].Digits != "87" {
		t.Errorf("json 不对: %+v", v.Units)
	}
}

// 集成：真实 go-pinyin，证明汉字→拼音→小鹤→数字这条链通（离线可跑）。
func TestSPT9_RealPinyin(t *testing.T) {
	out, _, err := runSPT9(t, nil, "", "--digits", "你好世界")
	if err != nil {
		t.Fatal(err)
	}
	// 你 ni→ni→64, 好 hao→hc→42, 世 shi→ui→84, 界 jie→jp→57
	if strings.TrimSpace(out) != "64 42 84 57" {
		t.Errorf("你好世界 → %q, want 64 42 84 57", strings.TrimSpace(out))
	}
}
