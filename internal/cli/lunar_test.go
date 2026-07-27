package cli

import (
	"bytes"
	"encoding/json"
	"strings"
	"testing"
	"time"
)

func runLunar(t *testing.T, now time.Time, args ...string) (string, error) {
	t.Helper()
	var buf bytes.Buffer
	cmd := newLunarCommand(lunarCmdDeps{out: &buf, now: now})
	cmd.SetArgs(args)
	err := cmd.Execute()
	return buf.String(), err
}

var lunarNow = time.Date(2026, time.June, 26, 12, 0, 0, 0, time.UTC)

func TestLunarCmd_Inspect(t *testing.T) {
	out, err := runLunar(t, lunarNow, "2024-02-10")
	if err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(out, "甲辰年 正月初一") || !strings.Contains(out, "生肖 龙") {
		t.Errorf("2024 春节 output wrong:\n%s", out)
	}
}

func TestLunarCmd_LeapMonth(t *testing.T) {
	out, err := runLunar(t, lunarNow, "2025-07-25")
	if err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(out, "闰六月初一") {
		t.Errorf("2025 闰六月 output wrong:\n%s", out)
	}
}

func TestLunarCmd_ToSolar(t *testing.T) {
	out, err := runLunar(t, lunarNow, "--to-solar", "2026", "1", "1")
	if err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(out, "2026-02-17") {
		t.Errorf("2026 春节应为 2026-02-17:\n%s", out)
	}
}

func TestLunarCmd_ToSolarLeap(t *testing.T) {
	out, err := runLunar(t, lunarNow, "--to-solar", "2025", "6", "1", "--leap")
	if err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(out, "2025-07-25") {
		t.Errorf("2025 闰六月初一应为 2025-07-25:\n%s", out)
	}
}

func TestLunarCmd_ToSolarBadLeap(t *testing.T) {
	if _, err := runLunar(t, lunarNow, "--to-solar", "2026", "3", "1", "--leap"); err == nil {
		t.Error("2026 无闰三月，应报错")
	}
}

func TestLunarCmd_Festivals(t *testing.T) {
	out, err := runLunar(t, lunarNow, "2026", "--festivals")
	if err != nil {
		t.Fatal(err)
	}
	for _, want := range []string{"春节", "中秋", "除夕", "2026-02-17"} {
		if !strings.Contains(out, want) {
			t.Errorf("festivals output missing %q:\n%s", want, out)
		}
	}
}

func TestLunarCmd_FestivalsCurrentYear(t *testing.T) {
	out, err := runLunar(t, lunarNow, "--festivals")
	if err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(out, "2026 年农历节日") {
		t.Errorf("default festivals year should be now's year:\n%s", out)
	}
}

func TestLunarCmd_JSON(t *testing.T) {
	out, err := runLunar(t, lunarNow, "2024-02-10", "--json")
	if err != nil {
		t.Fatal(err)
	}
	var v struct {
		Lunar  string `json:"lunar"`
		Zodiac string `json:"zodiac"`
		IsLeap bool   `json:"is_leap"`
	}
	if err := json.Unmarshal([]byte(out), &v); err != nil {
		t.Fatalf("--json should emit valid JSON: %v\n%s", err, out)
	}
	if v.Zodiac != "龙" || !strings.Contains(v.Lunar, "正月初一") {
		t.Errorf("json wrong: %+v", v)
	}
}

func TestLunarCmd_BadDate(t *testing.T) {
	if _, err := runLunar(t, lunarNow, "not-a-date"); err == nil {
		t.Error("invalid date should error")
	}
}

func TestLunarCmd_OutOfRange(t *testing.T) {
	if _, err := runLunar(t, lunarNow, "1899-01-01"); err == nil {
		t.Error("out-of-range date should error")
	}
}

// TestLunarCmd_JSONFieldsAreStable 守着 --json 的 9 个既有字段。
//
// 这是加 jdan ganzhi 时被强制补上的回归测试。原来的 TestLunarCmd_JSON
// 只断言 3 个字段（lunar / zodiac / is_leap），剩下 6 个没人看着——
// 新增 ganzhi_basis 的时候顺手把 ganzhi 的含义从生肖年改成节气年，
// 测试会全绿，而所有下游脚本静默拿到另一个意思的值。
//
// ganzhi 尤其要盯：它是、且必须继续是**生肖年**（正月初一为界）。
// 八字的年柱以立春为界，那是 jdan ganzhi 的事，不是这里的。
func TestLunarCmd_JSONFieldsAreStable(t *testing.T) {
	out, err := runLunar(t, lunarNow, "2024-02-10", "--json")
	if err != nil {
		t.Fatal(err)
	}
	var m map[string]any
	if err := json.Unmarshal([]byte(out), &m); err != nil {
		t.Fatalf("--json 应输出合法 JSON: %v\n%s", err, out)
	}

	// 字段名 -> 期望的 JSON 类型与值。9 个全部锁死。
	want := []struct {
		key   string
		kind  string // "string" | "number" | "bool"
		value any    // nil 表示只查类型不查值
	}{
		{"solar", "string", "2024-02-10"},
		{"weekday", "string", "周六"},
		{"lunar", "string", nil},
		{"year", "number", float64(2024)},
		{"month", "number", float64(1)},
		{"day", "number", float64(1)},
		{"is_leap", "bool", false},
		{"ganzhi", "string", "甲辰"}, // 生肖年，不是节气年
		{"zodiac", "string", "龙"},
	}
	for _, w := range want {
		v, ok := m[w.key]
		if !ok {
			t.Errorf("既有字段 %q 消失了——这是对已发布接口的破坏性变更", w.key)
			continue
		}
		switch w.kind {
		case "string":
			if _, ok := v.(string); !ok {
				t.Errorf("字段 %q 类型变了：want string, got %T", w.key, v)
			}
		case "number":
			if _, ok := v.(float64); !ok {
				t.Errorf("字段 %q 类型变了：want number, got %T", w.key, v)
			}
		case "bool":
			if _, ok := v.(bool); !ok {
				t.Errorf("字段 %q 类型变了：want bool, got %T", w.key, v)
			}
		}
		if w.value != nil && v != w.value {
			t.Errorf("字段 %q 的值变了：want %v, got %v", w.key, w.value, v)
		}
	}

	// 2024-02-10 是甲辰年正月初一（春节当天），此时生肖年与节气年一致
	// （2024 立春在 2/4，早于春节），所以不该有差异标记。
	if d, ok := m["term_year_differs"].(bool); ok && d {
		t.Errorf("2024-02-10 生肖年与节气年应一致，实得 differs=true")
	}
	if m["ganzhi_basis"] != "lunar-new-year" {
		t.Errorf("ganzhi_basis = %v, want lunar-new-year", m["ganzhi_basis"])
	}
}

// 差异期：立春已过、春节未到，两套口径给出不同干支，必须提示。
func TestLunarCmd_BasisDifferHint(t *testing.T) {
	// 2026 立春 02-04，春节 02-17
	out, err := runLunar(t, lunarNow, "2026-02-10")
	if err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(out, "口径") {
		t.Errorf("输出应标明口径:\n%s", out)
	}
	if !strings.Contains(out, "节气年") || !strings.Contains(out, "jdan ganzhi") {
		t.Errorf("差异期应提示节气年并指向 jdan ganzhi:\n%s", out)
	}

	jsonOut, err := runLunar(t, lunarNow, "2026-02-10", "--json")
	if err != nil {
		t.Fatal(err)
	}
	var m map[string]any
	if err := json.Unmarshal([]byte(jsonOut), &m); err != nil {
		t.Fatal(err)
	}
	if m["ganzhi"] != "乙巳" {
		t.Errorf("生肖年应为乙巳（春节未到），实得 %v", m["ganzhi"])
	}
	if m["solar_term_ganzhi"] != "丙午" {
		t.Errorf("节气年应为丙午（立春已过），实得 %v", m["solar_term_ganzhi"])
	}
	if m["term_year_differs"] != true {
		t.Errorf("term_year_differs 应为 true，实得 %v", m["term_year_differs"])
	}

	// 非差异期不该打提示行
	out, err = runLunar(t, lunarNow, "2026-06-26")
	if err != nil {
		t.Fatal(err)
	}
	if strings.Contains(out, "不同") {
		t.Errorf("非差异期不该打差异提示:\n%s", out)
	}
}
