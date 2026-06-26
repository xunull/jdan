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
