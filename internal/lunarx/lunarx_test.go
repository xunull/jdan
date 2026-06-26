package lunarx

import (
	"testing"
	"time"
)

func d(y, m, day int) time.Time { return time.Date(y, time.Month(m), day, 0, 0, 0, 0, time.UTC) }

// ---- 真实锚点（公历 → 农历）----

func TestSolarToLunar_Anchors(t *testing.T) {
	cases := []struct {
		in   time.Time
		want Lunar
		desc string
	}{
		{d(2024, 2, 10), Lunar{2024, 1, 1, false}, "2024 龙年春节"},
		{d(2025, 1, 29), Lunar{2025, 1, 1, false}, "2025 蛇年春节"},
		{d(2026, 2, 17), Lunar{2026, 1, 1, false}, "2026 马年春节"},
		{d(2024, 9, 17), Lunar{2024, 8, 15, false}, "2024 中秋"},
		{d(2024, 6, 10), Lunar{2024, 5, 5, false}, "2024 端午"},
		{d(1900, 1, 31), Lunar{1900, 1, 1, false}, "表起点"},
	}
	for _, c := range cases {
		got, err := SolarToLunar(c.in)
		if err != nil {
			t.Errorf("%s: %v", c.desc, err)
			continue
		}
		if got != c.want {
			t.Errorf("%s: SolarToLunar(%s) = %+v, want %+v", c.desc, c.in.Format("2006-01-02"), got, c.want)
		}
	}
}

// ---- 闰月 ----

func TestLeapMonths(t *testing.T) {
	// 2025 闰六月、2023 闰二月、2020 闰四月
	if leapMonth(2025) != 6 {
		t.Errorf("2025 应闰六月，得到闰%d月", leapMonth(2025))
	}
	if leapMonth(2023) != 2 {
		t.Errorf("2023 应闰二月，得到闰%d月", leapMonth(2023))
	}
	if leapMonth(2020) != 4 {
		t.Errorf("2020 应闰四月，得到闰%d月", leapMonth(2020))
	}
	// 闰六月初一往返
	dt, err := LunarToSolar(2025, 6, 1, true)
	if err != nil {
		t.Fatal(err)
	}
	back, _ := SolarToLunar(dt)
	if back != (Lunar{2025, 6, 1, true}) {
		t.Errorf("2025 闰六月初一往返失败：%+v", back)
	}
}

// ---- 农历 → 公历 ----

func TestLunarToSolar_Anchors(t *testing.T) {
	cases := []struct {
		y, m, day int
		leap      bool
		want      time.Time
	}{
		{2024, 1, 1, false, d(2024, 2, 10)},
		{2025, 1, 1, false, d(2025, 1, 29)},
		{2026, 1, 1, false, d(2026, 2, 17)},
		{2024, 8, 15, false, d(2024, 9, 17)},
	}
	for _, c := range cases {
		got, err := LunarToSolar(c.y, c.m, c.day, c.leap)
		if err != nil {
			t.Errorf("LunarToSolar(%d,%d,%d): %v", c.y, c.m, c.day, err)
			continue
		}
		if !got.Equal(c.want) {
			t.Errorf("LunarToSolar(%d,%d,%d) = %s, want %s", c.y, c.m, c.day, got.Format("2006-01-02"), c.want.Format("2006-01-02"))
		}
	}
}

// ---- round-trip（最强守护）----

func TestRoundTrip(t *testing.T) {
	cur := d(1900, 1, 31)
	end := d(2100, 12, 1)
	for cur.Before(end) {
		l, err := SolarToLunar(cur)
		if err != nil {
			t.Fatalf("%s: SolarToLunar err %v", cur.Format("2006-01-02"), err)
		}
		back, err := LunarToSolar(l.Year, l.Month, l.Day, l.IsLeap)
		if err != nil {
			t.Fatalf("%s → %+v: LunarToSolar err %v", cur.Format("2006-01-02"), l, err)
		}
		if !back.Equal(cur) {
			t.Fatalf("round-trip 失败：%s → %+v → %s", cur.Format("2006-01-02"), l, back.Format("2006-01-02"))
		}
		cur = cur.AddDate(0, 0, 29) // 每 29 天采样
	}
}

// ---- 干支 / 生肖 ----

func TestGanzhiZodiac(t *testing.T) {
	cases := []struct {
		year int
		gz   string
		zo   string
	}{
		{1984, "甲子", "鼠"},
		{2024, "甲辰", "龙"},
		{2025, "乙巳", "蛇"},
		{2026, "丙午", "马"},
	}
	for _, c := range cases {
		if gz := GanzhiYear(c.year); gz != c.gz {
			t.Errorf("GanzhiYear(%d) = %s, want %s", c.year, gz, c.gz)
		}
		if zo := Zodiac(c.year); zo != c.zo {
			t.Errorf("Zodiac(%d) = %s, want %s", c.year, zo, c.zo)
		}
	}
}

// ---- 名称 ----

func TestNames(t *testing.T) {
	if MonthName(5, false) != "五月" || MonthName(6, true) != "闰六月" || MonthName(12, false) != "腊月" || MonthName(1, false) != "正月" {
		t.Error("月名错误")
	}
	cases := map[int]string{1: "初一", 10: "初十", 11: "十一", 15: "十五", 20: "二十", 21: "廿一", 29: "廿九", 30: "三十"}
	for day, want := range cases {
		if DayName(day) != want {
			t.Errorf("DayName(%d) = %s, want %s", day, DayName(day), want)
		}
	}
}

// ---- 节日 ----

func TestFestivals(t *testing.T) {
	fs, err := Festivals(2024)
	if err != nil {
		t.Fatal(err)
	}
	want := map[string]time.Time{
		"春节": d(2024, 2, 10),
		"端午": d(2024, 6, 10),
		"中秋": d(2024, 9, 17),
	}
	got := map[string]time.Time{}
	for _, f := range fs {
		got[f.Name] = f.Date
	}
	for name, dt := range want {
		if !got[name].Equal(dt) {
			t.Errorf("%s = %s, want %s", name, got[name].Format("2006-01-02"), dt.Format("2006-01-02"))
		}
	}
	// 按公历日期升序
	for i := 1; i < len(fs); i++ {
		if fs[i].Date.Before(fs[i-1].Date) {
			t.Error("节日未按日期排序")
		}
	}
}

// ---- 范围 ----

func TestOutOfRange(t *testing.T) {
	if _, err := SolarToLunar(d(1900, 1, 1)); err == nil {
		t.Error("1900-01-01 早于表起点，应报错")
	}
	// 2101-01-01 公历仍在农历 2100 年（2101 春节在 2 月），不越界；用更晚的日期测
	if _, err := SolarToLunar(d(2102, 1, 1)); err == nil {
		t.Error("2102 晚于范围，应报错")
	}
}
