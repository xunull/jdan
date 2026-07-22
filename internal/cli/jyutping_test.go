package cli

import (
	"bytes"
	"encoding/json"
	"strings"
	"testing"
)

func runJyutping(t *testing.T, stdin string, args ...string) (string, error) {
	t.Helper()
	var out bytes.Buffer
	cmd := newJyutpingCommand(jyutpingDeps{out: &out, in: strings.NewReader(stdin)})
	cmd.SetArgs(args)
	cmd.SetOut(&out)
	cmd.SetErr(&out)
	err := cmd.Execute()
	return out.String(), err
}

func TestJyutpingCommand_Single(t *testing.T) {
	out, err := runJyutping(t, "", "你")
	if err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(out, "nei5") {
		t.Errorf("你 应 nei5：\n%s", out)
	}
}

func TestJyutpingCommand_Phrase(t *testing.T) {
	out, err := runJyutping(t, "", "我爱广东")
	if err != nil {
		t.Fatal(err)
	}
	for _, want := range []string{"我 ngo5", "爱 oi3", "广 gwong2", "东 dung1"} {
		if !strings.Contains(out, want) {
			t.Errorf("输出缺 %q：\n%s", want, out)
		}
	}
	if !strings.Contains(out, " / ") {
		t.Errorf("多字应用 ' / ' 分隔：\n%s", out)
	}
}

func TestJyutpingCommand_Stdin(t *testing.T) {
	out, err := runJyutping(t, "你好\n")
	if err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(out, "你 nei5") || !strings.Contains(out, "好 hou2") {
		t.Errorf("stdin 你好 应 你 nei5 / 好 hou2：\n%s", out)
	}
}

func TestJyutpingCommand_SkipsNonHan(t *testing.T) {
	out, _ := runJyutping(t, "", "Hi 你 2024")
	if !strings.Contains(out, "你") || !strings.Contains(out, "nei5") {
		t.Errorf("应统计 你：\n%s", out)
	}
	if strings.Contains(out, "Hi") || strings.Contains(out, "2024") {
		t.Errorf("非汉字不该出现在统计里：\n%s", out)
	}
}

func TestJyutpingCommand_OnlyNonHan(t *testing.T) {
	out, _ := runJyutping(t, "", "abc123")
	if !strings.Contains(out, "没有汉字") {
		t.Errorf("纯非汉字应提示没有汉字：\n%s", out)
	}
}

func TestJyutpingCommand_JSON(t *testing.T) {
	out, err := runJyutping(t, "", "--json", "你好")
	if err != nil {
		t.Fatal(err)
	}
	var res jyutpingJSONResult
	if err := json.Unmarshal([]byte(out), &res); err != nil {
		t.Fatalf("JSON 不可解析：%v\n%s", err, out)
	}
	if len(res.Chars) != 2 {
		t.Fatalf("应有 2 字，得 %d", len(res.Chars))
	}
	if res.Chars[0].Char != "你" || res.Chars[0].Reading != "nei5" || !res.Chars[0].Known {
		t.Errorf("首字应 你/nei5/known，得 %+v", res.Chars[0])
	}
	if res.Chars[1].Char != "好" || res.Chars[1].Reading != "hou2" {
		t.Errorf("次字应 好/hou2，得 %+v", res.Chars[1])
	}
	if res.Unknown != 0 {
		t.Errorf("unknown 应 0，得 %d", res.Unknown)
	}
}

// 非汉字不进 chars。
func TestJyutpingCommand_JSONSkipsNonHan(t *testing.T) {
	out, _ := runJyutping(t, "", "--json", "A你")
	var res jyutpingJSONResult
	if err := json.Unmarshal([]byte(out), &res); err != nil {
		t.Fatalf("JSON 不可解析：%v\n%s", err, out)
	}
	if len(res.Chars) != 1 || res.Chars[0].Char != "你" {
		t.Errorf("非汉字 A 不该进 chars，应只有 你：%+v", res.Chars)
	}
}

func TestJyutpingCommand_Empty(t *testing.T) {
	if _, err := runJyutping(t, ""); err == nil {
		t.Error("无输入应报错")
	}
	if _, err := runJyutping(t, "   \n"); err == nil {
		t.Error("纯空白应报错")
	}
}
