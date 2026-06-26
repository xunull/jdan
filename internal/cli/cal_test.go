package cli

import (
	"bytes"
	"encoding/json"
	"strings"
	"testing"
	"time"
)

func runCal(t *testing.T, now time.Time, args ...string) (string, error) {
	t.Helper()
	var buf bytes.Buffer
	cmd := newCalCommand(calCmdDeps{out: &buf, now: now})
	cmd.SetArgs(args)
	err := cmd.Execute()
	return buf.String(), err
}

var calNow = time.Date(2026, time.June, 15, 12, 0, 0, 0, time.UTC)

func TestCalCmd_CurrentMonth(t *testing.T) {
	out, err := runCal(t, calNow)
	if err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(out, "2026 年 6 月") || !strings.Contains(out, "一 二 三") {
		t.Errorf("current month output wrong:\n%s", out)
	}
}

func TestCalCmd_SpecificMonthYear(t *testing.T) {
	out, err := runCal(t, calNow, "2", "2026")
	if err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(out, "2026 年 2 月") {
		t.Errorf("got:\n%s", out)
	}
}

func TestCalCmd_MonthName(t *testing.T) {
	out, err := runCal(t, calNow, "December")
	if err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(out, "年 12 月") {
		t.Errorf("month name parse wrong:\n%s", out)
	}
}

func TestCalCmd_Year(t *testing.T) {
	out, err := runCal(t, calNow, "-y", "2026")
	if err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(out, "2026 年") || !strings.Contains(out, "1 月") || !strings.Contains(out, "12 月") {
		t.Errorf("year output should contain all months:\n%s", out)
	}
}

func TestCalCmd_Three(t *testing.T) {
	out, err := runCal(t, calNow, "-3")
	if err != nil {
		t.Fatal(err)
	}
	// 5月/6月/7月
	for _, m := range []string{"5 月", "6 月", "7 月"} {
		if !strings.Contains(out, m) {
			t.Errorf("three-month should contain %q:\n%s", m, out)
		}
	}
}

func TestCalCmd_InvalidMonth(t *testing.T) {
	if _, err := runCal(t, calNow, "13"); err == nil {
		t.Error("month 13 should error")
	}
}

func TestCalCmd_NoANSIWhenPiped(t *testing.T) {
	// 注入 buffer（非 TTY）+ now 落在所渲染月份 → 仍不应有 ANSI（今天不高亮）
	out, err := runCal(t, calNow)
	if err != nil {
		t.Fatal(err)
	}
	if strings.Contains(out, "\x1b") {
		t.Error("piped (non-TTY) output must not contain ANSI escapes")
	}
}

func TestCalCmd_JSON(t *testing.T) {
	out, err := runCal(t, calNow, "6", "2026", "--json")
	if err != nil {
		t.Fatal(err)
	}
	var v struct {
		Year      int     `json:"year"`
		Month     int     `json:"month"`
		WeekStart string  `json:"week_start"`
		Weeks     [][]int `json:"weeks"`
	}
	if err := json.Unmarshal([]byte(out), &v); err != nil {
		t.Fatalf("--json should emit valid JSON: %v\n%s", err, out)
	}
	if v.Year != 2026 || v.Month != 6 || v.WeekStart != "Monday" || len(v.Weeks) == 0 {
		t.Errorf("json wrong: %+v", v)
	}
}

func TestParseMonth(t *testing.T) {
	if m, _ := parseMonth("6"); m != time.June {
		t.Error("numeric month")
	}
	if m, _ := parseMonth("Jun"); m != time.June {
		t.Error("abbrev month")
	}
	if m, _ := parseMonth("january"); m != time.January {
		t.Error("full month case-insensitive")
	}
	if _, err := parseMonth("13"); err == nil {
		t.Error("13 should error")
	}
	if _, err := parseMonth("foo"); err == nil {
		t.Error("garbage should error")
	}
}
