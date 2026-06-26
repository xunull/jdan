package calx

import (
	"strings"
	"testing"
	"time"
)

// ---- MonthGrid ----

func TestMonthGrid_MondayStart(t *testing.T) {
	// 2026-06-01 是周一 → 第一周满格 1..7
	g := MonthGrid(2026, time.June, time.Monday)
	want := []int{1, 2, 3, 4, 5, 6, 7}
	for i, v := range want {
		if g[0][i] != v {
			t.Fatalf("week0 = %v, want %v", g[0], want)
		}
	}
	last := g[len(g)-1]
	if last[0] != 29 || last[1] != 30 || last[2] != 0 {
		t.Errorf("last week = %v, want [29 30 0 ...]", last)
	}
}

func TestMonthGrid_LeadingBlanks(t *testing.T) {
	// 2026-02-01 是周日 → 周一起始时 1 落在最后一列
	g := MonthGrid(2026, time.February, time.Monday)
	if g[0][6] != 1 {
		t.Errorf("2026-02 week0 = %v, want 1 in last column", g[0])
	}
	for i := range 6 {
		if g[0][i] != 0 {
			t.Errorf("leading cells should be blank: %v", g[0])
		}
	}
}

func TestMonthGrid_LeapYear(t *testing.T) {
	g := MonthGrid(2024, time.February, time.Monday)
	maxDay := 0
	for _, wk := range g {
		for _, d := range wk {
			if d > maxDay {
				maxDay = d
			}
		}
	}
	if maxDay != 29 {
		t.Errorf("2024-02 should have 29 days, got %d", maxDay)
	}
}

func TestMonthGrid_SundayStart(t *testing.T) {
	// 周日起始：2026-06-01(周一) → 1 落在第 2 列（col 1）
	g := MonthGrid(2026, time.June, time.Sunday)
	if g[0][1] != 1 || g[0][0] != 0 {
		t.Errorf("sunday start 2026-06 week0 = %v, want 1 at col 1", g[0])
	}
}

// ---- MonthLines / 高亮 ----

func TestMonthLines_Headers(t *testing.T) {
	lines := MonthLines(2026, time.June, Options{WeekStart: time.Monday}, -1)
	if !strings.Contains(lines[0], "2026 年 6 月") {
		t.Errorf("title wrong: %q", lines[0])
	}
	if !strings.HasPrefix(strings.TrimSpace(lines[1]), "一 二 三") {
		t.Errorf("weekday header wrong: %q", lines[1])
	}
}

func TestMonthLines_TodayHighlight(t *testing.T) {
	lines := MonthLines(2026, time.June, Options{WeekStart: time.Monday, Color: true}, 15)
	joined := strings.Join(lines, "\n")
	if !strings.Contains(joined, "\x1b[7m15\x1b[0m") {
		t.Errorf("today 15 should be reverse-highlighted:\n%s", joined)
	}
}

func TestMonthLines_NoColorNoANSI(t *testing.T) {
	lines := MonthLines(2026, time.June, Options{WeekStart: time.Monday, Color: false}, 15)
	if strings.Contains(strings.Join(lines, ""), "\x1b") {
		t.Error("Color=false must not emit ANSI")
	}
}

func TestMonthLines_WeekNumbers(t *testing.T) {
	lines := MonthLines(2026, time.June, Options{WeekStart: time.Monday, WeekNum: true}, -1)
	// 2026-06-01 那周是 ISO 第 23 周
	if !strings.HasPrefix(lines[2], "23 ") {
		t.Errorf("first week row should start with ISO week 23: %q", lines[2])
	}
}

func TestMonthLines_FixedHeight(t *testing.T) {
	lines := MonthLines(2026, time.June, Options{WeekStart: time.Monday}, -1)
	if len(lines) != 8 { // 标题 + 表头 + 6 周
		t.Errorf("month block should be 8 lines, got %d", len(lines))
	}
}

// ---- Render ----

func TestRender_TrimsTrailingBlank(t *testing.T) {
	out := Render(MonthLines(2026, time.June, Options{WeekStart: time.Monday}, -1))
	if strings.HasSuffix(out, "  \n\n") {
		t.Error("Render should trim trailing blank week lines")
	}
}

func TestRenderBlocks_SideBySide(t *testing.T) {
	a := MonthLines(2026, time.January, Options{WeekStart: time.Monday}, -1)
	b := MonthLines(2026, time.February, Options{WeekStart: time.Monday}, -1)
	out := RenderBlocks([][]string{a, b}, 2)
	// 标题行应同时含两个月
	first := strings.SplitN(out, "\n", 2)[0]
	if !strings.Contains(first, "1 月") || !strings.Contains(first, "2 月") {
		t.Errorf("two months should be side by side on the title line: %q", first)
	}
}

func TestVisualWidth_IgnoresANSI(t *testing.T) {
	if w := visualWidth("\x1b[7m15\x1b[0m"); w != 2 {
		t.Errorf("ANSI-wrapped '15' visual width = %d, want 2", w)
	}
}
