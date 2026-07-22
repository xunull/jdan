package cli

import (
	"bytes"
	"encoding/json"
	"strings"
	"testing"
)

func runSijiao(t *testing.T, stdin string, args ...string) (string, error) {
	t.Helper()
	var out bytes.Buffer
	cmd := newSijiaoCommand(sijiaoDeps{out: &out, in: strings.NewReader(stdin)})
	cmd.SetArgs(args)
	cmd.SetOut(&out)
	cmd.SetErr(&out)
	err := cmd.Execute()
	return out.String(), err
}

func TestSijiaoCommand_Single(t *testing.T) {
	out, err := runSijiao(t, "", "王")
	if err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(out, "1010.4") {
		t.Errorf("王 应 1010.4：\n%s", out)
	}
}

func TestSijiaoCommand_MultiValue(t *testing.T) {
	// 你 有两个码，都要出，用逗号分隔。
	out, err := runSijiao(t, "", "你")
	if err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(out, "2729.0, 2729.2") {
		t.Errorf("你 应列出两个码 2729.0, 2729.2：\n%s", out)
	}
}

func TestSijiaoCommand_Phrase(t *testing.T) {
	out, err := runSijiao(t, "", "口业专")
	if err != nil {
		t.Fatal(err)
	}
	for _, want := range []string{"口 6000.0", "业 3210", "专 5030"} {
		if !strings.Contains(out, want) {
			t.Errorf("输出缺 %q：\n%s", want, out)
		}
	}
	// 字与字之间用 " / "
	if !strings.Contains(out, " / ") {
		t.Errorf("多字应用 ' / ' 分隔：\n%s", out)
	}
}

func TestSijiaoCommand_Stdin(t *testing.T) {
	out, err := runSijiao(t, "王\n")
	if err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(out, "1010.4") {
		t.Errorf("stdin 王 应 1010.4：\n%s", out)
	}
}

func TestSijiaoCommand_SkipsNonHan(t *testing.T) {
	out, err := runSijiao(t, "", "Hi 口 2024")
	if err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(out, "口") || !strings.Contains(out, "6000.0") {
		t.Errorf("应统计 口：\n%s", out)
	}
	if strings.Contains(out, "Hi") || strings.Contains(out, "2024") {
		t.Errorf("非汉字不该出现在统计里：\n%s", out)
	}
}

func TestSijiaoCommand_OnlyNonHan(t *testing.T) {
	out, err := runSijiao(t, "", "abc123")
	if err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(out, "没有汉字") {
		t.Errorf("纯非汉字应提示没有汉字：\n%s", out)
	}
}

func TestSijiaoCommand_JSON(t *testing.T) {
	out, err := runSijiao(t, "", "--json", "你口")
	if err != nil {
		t.Fatal(err)
	}
	var res sijiaoJSONResult
	if err := json.Unmarshal([]byte(out), &res); err != nil {
		t.Fatalf("JSON 不可解析：%v\n%s", err, out)
	}
	if len(res.Chars) != 2 {
		t.Fatalf("应有 2 字，得 %d", len(res.Chars))
	}
	// 你 是多值：codes 数组长度 2
	if res.Chars[0].Char != "你" || len(res.Chars[0].Codes) != 2 {
		t.Errorf("首字应为 你、两个码，得 %+v", res.Chars[0])
	}
	if res.Chars[1].Char != "口" || len(res.Chars[1].Codes) != 1 || res.Chars[1].Codes[0] != "6000.0" {
		t.Errorf("次字应为 口/6000.0，得 %+v", res.Chars[1])
	}
	if res.Unknown != 0 {
		t.Errorf("unknown 应为 0，得 %d", res.Unknown)
	}
}

func TestSijiaoCommand_Empty(t *testing.T) {
	if _, err := runSijiao(t, ""); err == nil {
		t.Error("无输入应报错")
	}
	if _, err := runSijiao(t, "   \n"); err == nil {
		t.Error("纯空白应报错")
	}
}
