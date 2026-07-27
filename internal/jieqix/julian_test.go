package jieqix

import (
	"math"
	"testing"
	"time"
)

// 这些 JDN 是天文学里被反复引用的固定点，不是从本包算出来的。
// J2000.0 那条尤其关键：2451545.0 是 2000-01-01 12:00 TT 的儒略日，
// 几乎所有星历公式都以它为原点，算错了后面全盘皆错。
func TestJDN_KnownEpochs(t *testing.T) {
	cases := []struct {
		y    int
		m    time.Month
		d    int
		want int
		why  string
	}{
		{2000, time.January, 1, 2451545, "J2000.0 历元（JD 2451545.0 = 当日 12:00）"},
		{1970, time.January, 1, 2440588, "Unix 纪元"},
		{1900, time.January, 1, 2415021, "20 世纪起点"},
		{1912, time.February, 18, 2419451, "日干支权威锚点（甲子日），见 ganzhix/testdata"},
		{1582, time.October, 15, 2299161, "格里高利改历首日"},
		{1582, time.October, 4, 2299160, "改历前最后一天（儒略历）；与上一条相邻，中间十天不存在"},
	}
	for _, c := range cases {
		if got := JDN(c.y, c.m, c.d); got != c.want {
			t.Errorf("JDN(%d-%02d-%02d) = %d, want %d  (%s)",
				c.y, c.m, c.d, got, c.want, c.why)
		}
	}
}

// 改历那十天（1582-10-05..14）在历史上不存在。这条测的是：紧邻改历日的
// 两天在儒略日上必须只差 1——如果实现是按「年月日先比较再选历法」写的，
// 这里会差出 10 天来。
func TestJDN_GregorianSwitchIsContiguous(t *testing.T) {
	before := JDN(1582, time.October, 4)
	after := JDN(1582, time.October, 15)
	if after-before != 1 {
		t.Errorf("改历日前后应相差 1 天，实得 %d 天（before=%d after=%d）",
			after-before, before, after)
	}
}

// 日干支锚点的自洽性：1912-02-18 与 1949-10-01 都是甲子日，
// 两者 JDN 之差必须是 60 的整数倍。这条把 testdata/day_pillar.tsv
// 里那句「13740 天 = 229 × 60」变成机器可验证的。
func TestJDN_GanzhiAnchorsAreConsistent(t *testing.T) {
	a := JDN(1912, time.February, 18)
	b := JDN(1949, time.October, 1)
	if d := b - a; d != 13740 || d%60 != 0 {
		t.Errorf("两个甲子日锚点间隔 = %d 天，want 13740 且能被 60 整除", d)
	}
}

func TestJulianDay_MidnightHalfDayOffset(t *testing.T) {
	// 儒略日的零点在中午。2000-01-01 00:00 UT 应当是 2451544.5，
	// 不是 2451545.0——差的这半天是儒略日最常见的错源。
	got := julianDay(time.Date(2000, time.January, 1, 0, 0, 0, 0, time.UTC))
	if math.Abs(got-2451544.5) > 1e-9 {
		t.Errorf("julianDay(2000-01-01 00:00 UT) = %.9f, want 2451544.5", got)
	}
	got = julianDay(time.Date(2000, time.January, 1, 12, 0, 0, 0, time.UTC))
	if math.Abs(got-2451545.0) > 1e-9 {
		t.Errorf("julianDay(2000-01-01 12:00 UT) = %.9f, want 2451545.0", got)
	}
}

// round-trip：任意时刻转成儒略日再转回来，必须回到秒级同一时刻。
// 覆盖改历前后、跨年、闰日、以及一天里的各个时段。
func TestJulianDay_RoundTrip(t *testing.T) {
	cases := []time.Time{
		time.Date(1582, time.October, 15, 0, 0, 0, 0, time.UTC),
		time.Date(1582, time.October, 4, 23, 59, 59, 0, time.UTC),
		time.Date(1900, time.February, 4, 5, 51, 0, 0, time.UTC),
		time.Date(1929, time.January, 1, 0, 0, 0, 0, time.UTC),
		time.Date(2000, time.February, 29, 12, 34, 56, 0, time.UTC),
		time.Date(2026, time.February, 3, 20, 2, 0, 0, time.UTC),
		time.Date(2026, time.December, 31, 23, 59, 59, 0, time.UTC),
		time.Date(2100, time.December, 21, 19, 52, 0, 0, time.UTC),
	}
	for _, want := range cases {
		got := fromJulianDay(julianDay(want))
		if !got.Equal(want) {
			t.Errorf("round-trip %s -> %s", want.Format(time.RFC3339), got.Format(time.RFC3339))
		}
	}
}

// 连续 4000 天逐日 round-trip，跨越 2000 年前后与闰年。
// 单点用例挑不出「某些月份差一天」这类系统性错误。
func TestJulianDay_RoundTripSweep(t *testing.T) {
	start := time.Date(1995, time.January, 1, 6, 30, 0, 0, time.UTC)
	for i := 0; i < 4000; i++ {
		want := start.AddDate(0, 0, i)
		if got := fromJulianDay(julianDay(want)); !got.Equal(want) {
			t.Fatalf("第 %d 天 round-trip 失败: %s -> %s", i,
				want.Format(time.RFC3339), got.Format(time.RFC3339))
		}
	}
}

func TestFloorDiv(t *testing.T) {
	// Go 的 / 对负数向零截断，儒略日公式要的是向下取整。
	cases := []struct{ a, b, want int }{
		{7, 2, 3}, {-7, 2, -4}, {7, -2, -4}, {-7, -2, 3}, {-6, 2, -3}, {0, 5, 0},
	}
	for _, c := range cases {
		if got := floorDiv(c.a, c.b); got != c.want {
			t.Errorf("floorDiv(%d, %d) = %d, want %d", c.a, c.b, got, c.want)
		}
	}
}
