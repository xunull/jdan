// Package lunarx 公历 ↔ 农历（中国阴历）转换 + 干支/生肖 + 农历节日。
// 用内嵌的 1900–2100 农历表（每年一个整数编码大小月 bitmask + 闰月号 + 闰月大小），
// 公开算法、0 新依赖。正确性靠一批真实锚点 + round-trip 测试守护。
package lunarx

import (
	"fmt"
	"sort"
	"time"

	"github.com/xunull/jdan/internal/ganzhix"
)

// lunarInfo[year-1900]：bits 4-15 = 1..12 月大小（1=30 天），bits 0-3 = 闰月号（0=无），
// bit 16 = 闰月是否 30 天。1900–2100 共 201 项。
var lunarInfo = []int{
	0x04bd8, 0x04ae0, 0x0a570, 0x054d5, 0x0d260, 0x0d950, 0x16554, 0x056a0, 0x09ad0, 0x055d2, // 1900-1909
	0x04ae0, 0x0a5b6, 0x0a4d0, 0x0d250, 0x1d255, 0x0b540, 0x0d6a0, 0x0ada2, 0x095b0, 0x14977, // 1910-1919
	0x04970, 0x0a4b0, 0x0b4b5, 0x06a50, 0x06d40, 0x1ab54, 0x02b60, 0x09570, 0x052f2, 0x04970, // 1920-1929
	0x06566, 0x0d4a0, 0x0ea50, 0x06e95, 0x05ad0, 0x02b60, 0x186e3, 0x092e0, 0x1c8d7, 0x0c950, // 1930-1939
	0x0d4a0, 0x1d8a6, 0x0b550, 0x056a0, 0x1a5b4, 0x025d0, 0x092d0, 0x0d2b2, 0x0a950, 0x0b557, // 1940-1949
	0x06ca0, 0x0b550, 0x15355, 0x04da0, 0x0a5b0, 0x14573, 0x052b0, 0x0a9a8, 0x0e950, 0x06aa0, // 1950-1959
	0x0aea6, 0x0ab50, 0x04b60, 0x0aae4, 0x0a570, 0x05260, 0x0f263, 0x0d950, 0x05b57, 0x056a0, // 1960-1969
	0x096d0, 0x04dd5, 0x04ad0, 0x0a4d0, 0x0d4d4, 0x0d250, 0x0d558, 0x0b540, 0x0b5a0, 0x195a6, // 1970-1979
	0x095b0, 0x049b0, 0x0a974, 0x0a4b0, 0x0b27a, 0x06a50, 0x06d40, 0x0af46, 0x0ab60, 0x09570, // 1980-1989
	0x04af5, 0x04970, 0x064b0, 0x074a3, 0x0ea50, 0x06b58, 0x05ac0, 0x0ab60, 0x096d5, 0x092e0, // 1990-1999
	0x0c960, 0x0d954, 0x0d4a0, 0x0da50, 0x07552, 0x056a0, 0x0abb7, 0x025d0, 0x092d0, 0x0cab5, // 2000-2009
	0x0a950, 0x0b4a0, 0x0baa4, 0x0ad50, 0x055d9, 0x04ba0, 0x0a5b0, 0x15176, 0x052b0, 0x0a930, // 2010-2019
	0x07954, 0x06aa0, 0x0ad50, 0x05b52, 0x04b60, 0x0a6e6, 0x0a4e0, 0x0d260, 0x0ea65, 0x0d530, // 2020-2029
	0x05aa0, 0x076a3, 0x096d0, 0x04afb, 0x04ad0, 0x0a4d0, 0x1d0b6, 0x0d250, 0x0d520, 0x0dd45, // 2030-2039
	0x0b5a0, 0x056d0, 0x055b2, 0x049b0, 0x0a577, 0x0a4b0, 0x0aa50, 0x1b255, 0x06d20, 0x0ada0, // 2040-2049
	0x14b63, 0x09370, 0x049f8, 0x04970, 0x064b0, 0x168a6, 0x0ea50, 0x06b20, 0x1a6c4, 0x0aae0, // 2050-2059
	0x0a2e0, 0x0d2e3, 0x0c960, 0x0d557, 0x0d4a0, 0x0da50, 0x05d55, 0x056a0, 0x0a6d0, 0x055d4, // 2060-2069
	0x052d0, 0x0a9b8, 0x0a950, 0x0b4a0, 0x0b6a6, 0x0ad50, 0x055a0, 0x0aba4, 0x0a5b0, 0x052b0, // 2070-2079
	0x0b273, 0x06930, 0x07337, 0x06aa0, 0x0ad50, 0x14b55, 0x04b60, 0x0a570, 0x054e4, 0x0d160, // 2080-2089
	0x0e968, 0x0d520, 0x0daa0, 0x16aa6, 0x056d0, 0x04ae0, 0x0a9d4, 0x0a2d0, 0x0d150, 0x0f252, // 2090-2099
	0x0d520, // 2100
}

const (
	minYear = 1900
	maxYear = 2100
)

func baseDate() time.Time { return time.Date(1900, 1, 31, 0, 0, 0, 0, time.UTC) }

func info(year int) int { return lunarInfo[year-minYear] }

func leapMonth(year int) int { return info(year) & 0xf } // 0 = 无闰月

func leapDays(year int) int {
	if leapMonth(year) == 0 {
		return 0
	}
	if info(year)&0x10000 != 0 {
		return 30
	}
	return 29
}

func monthDays(year, month int) int {
	if info(year)&(0x10000>>uint(month)) != 0 {
		return 30
	}
	return 29
}

func yearDays(year int) int {
	sum := leapDays(year)
	for m := 1; m <= 12; m++ {
		sum += monthDays(year, m)
	}
	return sum
}

// Lunar 是一个农历日期。
type Lunar struct {
	Year   int
	Month  int // 1-12
	Day    int // 1-30
	IsLeap bool
}

// SolarToLunar 公历 → 农历。
func SolarToLunar(t time.Time) (Lunar, error) {
	t = time.Date(t.Year(), t.Month(), t.Day(), 0, 0, 0, 0, time.UTC)
	offset := int(t.Sub(baseDate()) / (24 * time.Hour))
	if offset < 0 {
		return Lunar{}, fmt.Errorf("早于支持范围（1900-01-31）")
	}
	year := minYear
	for year <= maxYear {
		yd := yearDays(year)
		if offset < yd {
			break
		}
		offset -= yd
		year++
	}
	if year > maxYear {
		return Lunar{}, fmt.Errorf("晚于支持范围（%d）", maxYear)
	}

	leap := leapMonth(year)
	for m := 1; m <= 12; m++ {
		if d := monthDays(year, m); offset < d {
			return Lunar{year, m, offset + 1, false}, nil
		} else {
			offset -= d
		}
		if leap > 0 && m == leap {
			if ld := leapDays(year); offset < ld {
				return Lunar{year, m, offset + 1, true}, nil
			} else {
				offset -= ld
			}
		}
	}
	return Lunar{}, fmt.Errorf("内部错误：月份遍历越界")
}

// LunarToSolar 农历 → 公历。leap=true 表示闰月（该年须确有此闰月）。
func LunarToSolar(year, month, day int, leap bool) (time.Time, error) {
	if year < minYear || year > maxYear {
		return time.Time{}, fmt.Errorf("年份超出支持范围（%d–%d）", minYear, maxYear)
	}
	if month < 1 || month > 12 {
		return time.Time{}, fmt.Errorf("农历月份要 1–12")
	}
	lm := leapMonth(year)
	if leap && month != lm {
		return time.Time{}, fmt.Errorf("%d 年没有闰%d月", year, month)
	}

	offset := 0
	for y := minYear; y < year; y++ {
		offset += yearDays(y)
	}
	for m := 1; m <= 12; m++ {
		if m == month && !leap {
			break
		}
		offset += monthDays(year, m)
		if lm > 0 && m == lm {
			if leap && m == month {
				break
			}
			offset += leapDays(year)
		}
	}
	maxD := monthDays(year, month)
	if leap {
		maxD = leapDays(year)
	}
	if day < 1 || day > maxD {
		return time.Time{}, fmt.Errorf("该农历月只有 %d 天，无第 %d 天", maxD, day)
	}
	offset += day - 1
	return baseDate().AddDate(0, 0, offset), nil
}

// ---- 干支 / 生肖 ----

// 干支表与索引算法住在 internal/ganzhix —— 那里是它们的语义归属，
// 而且 ganzhix/jieqix 不受本包 1900–2100 的硬边界限制。
// 这里只做转发，保持本包既有 API 不变。
//
// ⚠️ 口径提醒：本包传的是**农历年**（正月初一为界的生肖年）。
// 八字四柱的年柱以**立春**为界，两者在春节到立春之间会给出不同干支，
// 最长约 30 天。要四柱请用 ganzhix.Of，不要拿 GanzhiYear 去凑。

// GanzhiYear 返回农历年的干支（如 1984 → 甲子）。以正月初一为界。
func GanzhiYear(lunarYear int) string { return ganzhix.FromYear(lunarYear).String() }

// Zodiac 返回农历年的生肖（如 2024 → 龙）。以正月初一为界。
func Zodiac(lunarYear int) string { return ganzhix.ZodiacOfYear(lunarYear) }

// ---- 名称 ----

var cnMonth = []string{"", "正", "二", "三", "四", "五", "六", "七", "八", "九", "十", "冬", "腊"}
var cnNum = []rune("日一二三四五六七八九十")

// MonthName 返回农历月名（如 5 → 五月；闰 6 → 闰六月）。
func MonthName(month int, leap bool) string {
	name := cnMonth[month] + "月"
	if leap {
		return "闰" + name
	}
	return name
}

// DayName 返回农历日名（初一 / 十五 / 廿三 / 三十）。
func DayName(day int) string {
	switch {
	case day <= 10:
		return "初" + string(cnNum[day])
	case day < 20:
		return "十" + string(cnNum[day-10])
	case day == 20:
		return "二十"
	case day < 30:
		return "廿" + string(cnNum[day-20])
	default:
		return "三十"
	}
}

// String 返回农历日期的中文（丙午年 五月初二）。
func (l Lunar) String() string {
	return GanzhiYear(l.Year) + "年 " + MonthName(l.Month, l.IsLeap) + DayName(l.Day)
}

// ---- 节日 ----

// Festival 是一个农历节日及其公历日期。
type Festival struct {
	Name string
	Date time.Time
}

// Festivals 返回某公历年里的农历节日（按公历日期排序）。
// 都是纯农历日期推出来的；清明/冬至属节气不在此列。
func Festivals(solarYear int) ([]Festival, error) {
	defs := []struct {
		name string
		m, d int
	}{
		{"春节", 1, 1}, {"元宵", 1, 15}, {"端午", 5, 5},
		{"七夕", 7, 7}, {"中秋", 8, 15}, {"重阳", 9, 9},
	}
	var fs []Festival
	for _, df := range defs {
		dt, err := LunarToSolar(solarYear, df.m, df.d, false)
		if err != nil {
			return nil, err
		}
		fs = append(fs, Festival{df.name, dt})
	}
	// 除夕 = 次年春节前一天
	if ny, err := LunarToSolar(solarYear+1, 1, 1, false); err == nil {
		fs = append(fs, Festival{"除夕", ny.AddDate(0, 0, -1)})
	}
	sort.Slice(fs, func(i, j int) bool { return fs[i].Date.Before(fs[j].Date) })
	return fs, nil
}
