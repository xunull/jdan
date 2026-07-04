package cli

import (
	"bytes"
	"encoding/json"
	"strings"
	"testing"
)

func runSP(t *testing.T, pinyinOf func(rune) string, stdin string, args ...string) (string, string, error) {
	t.Helper()
	var o, e bytes.Buffer
	cmd := newSPCommand(spDeps{out: &o, errOut: &e, in: strings.NewReader(stdin), pinyinOf: pinyinOf})
	cmd.SetArgs(args)
	err := cmd.Execute()
	return o.String(), e.String(), err
}

func TestSP_DefaultFlypy(t *testing.T) {
	// 中=zhong→vs, 文=wen→wf（小鹤默认）
	out, _, err := runSP(t, fakePinyin, "", "--codes", "中文")
	if err != nil {
		t.Fatal(err)
	}
	if strings.TrimSpace(out) != "vs wf" {
		t.Errorf("中文(小鹤) → %q, want vs wf", strings.TrimSpace(out))
	}
}

func TestSP_SchemeByName(t *testing.T) {
	// 拼音加加：中=vy, 文=wr
	out, _, err := runSP(t, fakePinyin, "", "--codes", "--scheme", "拼音加加", "中文")
	if err != nil {
		t.Fatal(err)
	}
	if strings.TrimSpace(out) != "vy wr" {
		t.Errorf("中文(拼音加加) → %q, want vy wr", strings.TrimSpace(out))
	}
}

func TestSP_SchemeAlias(t *testing.T) {
	// 搜狗别名 → 微软：中=vs 文=wf
	out, _, err := runSP(t, fakePinyin, "", "--codes", "-s", "sogou", "中文")
	if err != nil {
		t.Fatal(err)
	}
	if strings.TrimSpace(out) != "vs wf" {
		t.Errorf("中文(搜狗=微软) → %q, want vs wf", strings.TrimSpace(out))
	}
}

func TestSP_UnknownScheme(t *testing.T) {
	_, _, err := runSP(t, fakePinyin, "", "-s", "nope", "中")
	if err == nil {
		t.Error("未知方案应报错")
	}
}

func TestSP_All(t *testing.T) {
	out, _, err := runSP(t, fakePinyin, "", "--all", "中文")
	if err != nil {
		t.Fatal(err)
	}
	// 应列出各方案，且小鹤=vs wf、智能ABC=as wf 各不同
	for _, name := range []string{"小鹤", "自然码", "微软", "智能ABC", "拼音加加"} {
		if !strings.Contains(out, name) {
			t.Errorf("--all 缺方案 %s:\n%s", name, out)
		}
	}
	if !strings.Contains(out, "vs wf") || !strings.Contains(out, "as wf") {
		t.Errorf("--all 码串不对:\n%s", out)
	}
}

func TestSP_MixedEnglish(t *testing.T) {
	// 英文按字母本身：中 hi → "vs hi"
	out, _, err := runSP(t, fakePinyin, "", "--codes", "中 hi")
	if err != nil {
		t.Fatal(err)
	}
	if strings.TrimSpace(out) != "vs hi" {
		t.Errorf("中 hi → %q, want vs hi", strings.TrimSpace(out))
	}
}

func TestSP_JSONOne(t *testing.T) {
	out, _, err := runSP(t, fakePinyin, "", "--json", "中")
	if err != nil {
		t.Fatal(err)
	}
	var v struct {
		Scheme string
		Units  []struct{ Text, Pinyin, Code string }
	}
	if err := json.Unmarshal([]byte(out), &v); err != nil {
		t.Fatalf("非法 json:\n%s", out)
	}
	if v.Scheme != "flypy" || len(v.Units) != 1 || v.Units[0].Code != "vs" {
		t.Errorf("json 不对: %+v", v)
	}
}

func TestSP_JSONAll(t *testing.T) {
	out, _, err := runSP(t, fakePinyin, "", "--all", "--json", "中")
	if err != nil {
		t.Fatal(err)
	}
	var m map[string]string
	if err := json.Unmarshal([]byte(out), &m); err != nil {
		t.Fatalf("非法 json:\n%s", out)
	}
	if m["flypy"] != "vs" || m["abc"] != "as" {
		t.Errorf("--all json 不对: %+v", m)
	}
}

// 集成：真实 go-pinyin，证明整条链通。
func TestSP_RealPinyin(t *testing.T) {
	// 你 ni→ni, 好 hao→hc（小鹤）
	out, _, err := runSP(t, nil, "", "--codes", "你好")
	if err != nil {
		t.Fatal(err)
	}
	if strings.TrimSpace(out) != "ni hc" {
		t.Errorf("你好(小鹤) → %q, want ni hc", strings.TrimSpace(out))
	}
}
