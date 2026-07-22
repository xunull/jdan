package cli

import (
	"bytes"
	"encoding/json"
	"strings"
	"testing"
)

func runCangjie(t *testing.T, stdin string, args ...string) (string, error) {
	t.Helper()
	var out bytes.Buffer
	cmd := newCangjieCommand(cangjieDeps{out: &out, in: strings.NewReader(stdin)})
	cmd.SetArgs(args)
	cmd.SetOut(&out)
	cmd.SetErr(&out)
	err := cmd.Execute()
	return out.String(), err
}

func TestCangjieCommand_SingleWithRoots(t *testing.T) {
	out, err := runCangjie(t, "", "明")
	if err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(out, "AB") || !strings.Contains(out, "日月") {
		t.Errorf("明 应 AB（日月）：\n%s", out)
	}
}

func TestCangjieCommand_Roots(t *testing.T) {
	out, _ := runCangjie(t, "", "你")
	if !strings.Contains(out, "ONF") || !strings.Contains(out, "人弓火") {
		t.Errorf("你 应 ONF（人弓火）：\n%s", out)
	}
}

func TestCangjieCommand_Phrase(t *testing.T) {
	out, err := runCangjie(t, "", "明一")
	if err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(out, "明 AB（日月）") || !strings.Contains(out, "一 M（一）") {
		t.Errorf("逐字应带码和字根：\n%s", out)
	}
	if !strings.Contains(out, " / ") {
		t.Errorf("多字应用 ' / ' 分隔：\n%s", out)
	}
}

func TestCangjieCommand_Stdin(t *testing.T) {
	out, err := runCangjie(t, "明\n")
	if err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(out, "AB") {
		t.Errorf("stdin 明 应 AB：\n%s", out)
	}
}

func TestCangjieCommand_SkipsNonHan(t *testing.T) {
	out, _ := runCangjie(t, "", "Hi 明 2024")
	if !strings.Contains(out, "明") || !strings.Contains(out, "AB") {
		t.Errorf("应统计 明：\n%s", out)
	}
	if strings.Contains(out, "Hi") || strings.Contains(out, "2024") {
		t.Errorf("非汉字不该出现在统计里：\n%s", out)
	}
}

func TestCangjieCommand_OnlyNonHan(t *testing.T) {
	out, _ := runCangjie(t, "", "abc123")
	if !strings.Contains(out, "没有汉字") {
		t.Errorf("纯非汉字应提示没有汉字：\n%s", out)
	}
}

func TestCangjieCommand_JSON(t *testing.T) {
	out, err := runCangjie(t, "", "--json", "明你")
	if err != nil {
		t.Fatal(err)
	}
	var res cangjieJSONResult
	if err := json.Unmarshal([]byte(out), &res); err != nil {
		t.Fatalf("JSON 不可解析：%v\n%s", err, out)
	}
	if len(res.Chars) != 2 {
		t.Fatalf("应有 2 字，得 %d", len(res.Chars))
	}
	if res.Chars[0].Char != "明" || res.Chars[0].Code != "AB" || res.Chars[0].Roots != "日月" || !res.Chars[0].Known {
		t.Errorf("首字应 明/AB/日月/known，得 %+v", res.Chars[0])
	}
	if res.Chars[1].Char != "你" || res.Chars[1].Code != "ONF" {
		t.Errorf("次字应 你/ONF，得 %+v", res.Chars[1])
	}
	if res.Unknown != 0 {
		t.Errorf("unknown 应 0，得 %d", res.Unknown)
	}
}

// 非汉字不进 chars。
func TestCangjieCommand_JSONSkipsNonHan(t *testing.T) {
	out, _ := runCangjie(t, "", "--json", "A明")
	var res cangjieJSONResult
	if err := json.Unmarshal([]byte(out), &res); err != nil {
		t.Fatalf("JSON 不可解析：%v\n%s", err, out)
	}
	if len(res.Chars) != 1 || res.Chars[0].Char != "明" {
		t.Errorf("非汉字 A 不该进 chars，应只有 明：%+v", res.Chars)
	}
}

func TestCangjieCommand_Empty(t *testing.T) {
	if _, err := runCangjie(t, ""); err == nil {
		t.Error("无输入应报错")
	}
	if _, err := runCangjie(t, "   \n"); err == nil {
		t.Error("纯空白应报错")
	}
}
