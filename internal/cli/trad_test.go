package cli

import (
	"bytes"
	"encoding/json"
	"strings"
	"testing"
)

func runTrad(t *testing.T, stdin string, args ...string) (string, error) {
	t.Helper()
	var out bytes.Buffer
	cmd := newTradCommand(tradDeps{out: &out, in: strings.NewReader(stdin)})
	cmd.SetArgs(args)
	cmd.SetOut(&out)
	cmd.SetErr(&out)
	err := cmd.Execute()
	return out.String(), err
}

func TestTradCommand_S2T_Default(t *testing.T) {
	out, err := runTrad(t, "", "头发和发展")
	if err != nil {
		t.Fatal(err)
	}
	if strings.TrimSpace(out) != "頭髮和發展" {
		t.Errorf("头发和发展 应为 頭髮和發展：%q", out)
	}
}

func TestTradCommand_S2TWP_Vocabulary(t *testing.T) {
	out, err := runTrad(t, "", "--to", "twp", "软件网络")
	if err != nil {
		t.Fatal(err)
	}
	if strings.TrimSpace(out) != "軟體網路" {
		t.Errorf("--to twp 软件网络 应为 軟體網路：%q", out)
	}
}

func TestTradCommand_T2S(t *testing.T) {
	out, err := runTrad(t, "", "--to", "s", "軟體")
	if err != nil {
		t.Fatal(err)
	}
	if strings.TrimSpace(out) != "软体" {
		t.Errorf("--to s 軟體 应为 软体：%q", out)
	}
}

// s2t 与 s2twp 在"软件"上必须不同（字形 vs 词汇）。
func TestTradCommand_S2TvsTWP(t *testing.T) {
	st, _ := runTrad(t, "", "软件")
	twp, _ := runTrad(t, "", "--to", "twp", "软件")
	if strings.TrimSpace(st) != "軟件" {
		t.Errorf("s2t 软件 应为 軟件：%q", st)
	}
	if strings.TrimSpace(twp) != "軟體" {
		t.Errorf("s2twp 软件 应为 軟體：%q", twp)
	}
}

func TestTradCommand_Stdin(t *testing.T) {
	out, err := runTrad(t, "头发\n")
	if err != nil {
		t.Fatal(err)
	}
	if strings.TrimSpace(out) != "頭髮" {
		t.Errorf("stdin 头发 应为 頭髮：%q", out)
	}
}

// 多行 stdin 逐行处理。
func TestTradCommand_StdinMultiline(t *testing.T) {
	out, err := runTrad(t, "头发\n发展\n")
	if err != nil {
		t.Fatal(err)
	}
	got := strings.TrimSpace(out)
	if got != "頭髮\n發展" {
		t.Errorf("多行 stdin 应逐行转换，得 %q", got)
	}
}

func TestTradCommand_Passthrough(t *testing.T) {
	out, _ := runTrad(t, "", "Hi 发 2024!")
	if strings.TrimSpace(out) != "Hi 發 2024!" {
		t.Errorf("非汉字应透传：%q", out)
	}
}

func TestTradCommand_Diff(t *testing.T) {
	// 用带公共锚点（和）的例子，改动段才会按词分开；全变无锚点会合成一段（rune-diff 固有行为）。
	out, err := runTrad(t, "", "--diff", "--to", "twp", "软件和网络")
	if err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(out, "「軟體」") || !strings.Contains(out, "「網路」") {
		t.Errorf("--diff 应用「」括出改动段：%q", out)
	}
	if !strings.Contains(out, "软件 → 軟體") || !strings.Contains(out, "网络 → 網路") {
		t.Errorf("--diff 应列出 原→新：%q", out)
	}
}

func TestTradCommand_DiffNoChange(t *testing.T) {
	// 已是繁体，s2t 基本不动。
	out, _ := runTrad(t, "", "--diff", "ABC123")
	if !strings.Contains(out, "无改动") {
		t.Errorf("无改动时应提示：%q", out)
	}
}

func TestTradCommand_JSON(t *testing.T) {
	out, err := runTrad(t, "", "--json", "--to", "twp", "软件")
	if err != nil {
		t.Fatal(err)
	}
	var res tradJSONResult
	if err := json.Unmarshal([]byte(out), &res); err != nil {
		t.Fatalf("JSON 不可解析：%v\n%s", err, out)
	}
	if res.Config != "twp" || res.From != "软件" || res.To != "軟體" {
		t.Errorf("JSON 头字段错：%+v", res)
	}
	if len(res.Changed) != 1 || res.Changed[0].Pos != 0 || res.Changed[0].Orig != "软件" || res.Changed[0].Conv != "軟體" {
		t.Errorf("changed 应为 [{0 软件 軟體}]：%+v", res.Changed)
	}
}

func TestTradCommand_BadTarget(t *testing.T) {
	_, err := runTrad(t, "", "--to", "zzz", "头发")
	if err == nil {
		t.Error("未知 --to 应报错")
	}
}

func TestTradCommand_EmptyInput(t *testing.T) {
	// 真正空输入报错。
	if _, err := runTrad(t, ""); err == nil {
		t.Error("无输入应报错")
	}
	// 纯空白按 filter 语义透传（像 tr/sed），不报错。
	out, err := runTrad(t, "   \n")
	if err != nil {
		t.Errorf("纯空白应透传不报错：%v", err)
	}
	if !strings.Contains(out, "   ") {
		t.Errorf("纯空白应原样透传：%q", out)
	}
}
