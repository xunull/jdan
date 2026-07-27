// Package jieqix 算二十四节气的精确时刻。
//
// 输入公历年，输出 24 个节气各自的瞬间——「太阳视黄经走到 15° 整倍数」的那一刻。
// 纯算法、0 数据表、不限年份，这一点和同仓库的 lunarx 不同（lunarx 内嵌
// 1900–2100 农历表，出界直接报错）。
//
// 算法链，顺序不能乱：
//
//	公历 ─▶ 儒略日 ─▶ 地球日心黄经 L (截断 VSOP87)
//	                    │
//	                    ├─ +180°  ──▶ 太阳地心几何黄经
//	                    ├─ 章动 Δψ
//	                    └─ 光行差 −20.4898″/R
//	                              │
//	                              ▼
//	                    太阳视黄经 λ
//	                              │
//	                    解 λ ≡ 15°k  (牛顿迭代)
//	                              │
//	                              ▼
//	                          时刻 (TT)
//	                              │
//	                          − ΔT      ← 漏这步是分钟级静默偏移
//	                              ▼
//	                          时刻 (UT)
//
// 本包一律以 UT 为输出基准。北京时（1929 前 UTC+8:05:43、之后 UTC+8）是
// 上层 CLI 的事——数值内核和时区口径分开测，两个关注点不搅在一起。
package jieqix

import "time"

// 格里高利历改历日：1582-10-15 之前按儒略历规则，之后按格里高利历。
// 与 NAOJ 暦計算室的默认口径一致（jg=2「デフォルト」），锚点表就是这么抓的。
const gregorianStartJDN = 2299161 // 1582-10-15

// floorDiv 是向下取整的整除。Go 的 / 对负数是向零截断，
// 而儒略日公式要求向下取整，公元前的年份会因此差 1 天。
// 本包声称不限年份，所以这里不能图省事直接用 /。
func floorDiv(a, b int) int {
	q := a / b
	if (a%b != 0) && ((a < 0) != (b < 0)) {
		q--
	}
	return q
}

// jdnGregorian 是 Fliegel–Van Flandern 公式（格里高利历）。
func jdnGregorian(y, m, d int) int {
	a := floorDiv(14-m, 12)
	yy := y + 4800 - a
	mm := m + 12*a - 3
	return d + floorDiv(153*mm+2, 5) + 365*yy +
		floorDiv(yy, 4) - floorDiv(yy, 100) + floorDiv(yy, 400) - 32045
}

// jdnJulian 是同一族公式的儒略历分支（无百年/四百年例外）。
func jdnJulian(y, m, d int) int {
	a := floorDiv(14-m, 12)
	yy := y + 4800 - a
	mm := m + 12*a - 3
	return d + floorDiv(153*mm+2, 5) + 365*yy + floorDiv(yy, 4) - 32083
}

// JDN 返回该历日的儒略日数（整数，以当日 12:00 UT 为界）。
//
// 1582-10-15 起用格里高利历规则，之前用儒略历。判断方式是先按格里高利算，
// 落在改历日之前就改用儒略历——不能先比年月日，因为 1582 年 10 月 5–14 日
// 这十天在历史上不存在，按日期比较会得到一个没有意义的答案。
func JDN(y int, m time.Month, d int) int {
	g := jdnGregorian(y, int(m), d)
	if g < gregorianStartJDN {
		return jdnJulian(y, int(m), d)
	}
	return g
}

// julianDay 返回 t 对应的儒略日（含小数部分）。t 按 UTC 解释。
//
// 儒略日的零点是中午 12:00 UT，不是午夜——所以要减 0.5 天。
// 这个半天的偏移是儒略日最常见的错源。
func julianDay(t time.Time) float64 {
	t = t.UTC()
	n := JDN(t.Year(), t.Month(), t.Day())
	frac := (float64(t.Hour()) +
		float64(t.Minute())/60 +
		float64(t.Second())/3600 +
		float64(t.Nanosecond())/3600e9) / 24
	return float64(n) - 0.5 + frac
}

// fromJulianDay 是 julianDay 的逆运算，返回 UTC 时刻。
//
// 结果四舍五入到秒。节气时刻的最终精度受 ΔT 与截断级数限制（分钟级），
// 保留纳秒只会给出一个假的精确感。
func fromJulianDay(jd float64) time.Time {
	z := int(floorF(jd + 0.5))
	frac := jd + 0.5 - float64(z)

	var a int
	if z < gregorianStartJDN {
		a = z
	} else {
		alpha := floorDiv(z*4-7468865, 146097)
		a = z + 1 + alpha - floorDiv(alpha, 4)
	}
	b := a + 1524
	c := floorDiv(b*100-12210, 36525)
	d := 365*c + floorDiv(c, 4)
	e := floorDiv((b-d)*10000, 306001)

	day := b - d - floorDiv(306001*e, 10000)
	var month int
	if e < 14 {
		month = e - 1
	} else {
		month = e - 13
	}
	var year int
	if month > 2 {
		year = c - 4716
	} else {
		year = c - 4715
	}

	secs := frac * 86400
	return time.Date(year, time.Month(month), day, 0, 0, 0, 0, time.UTC).
		Add(time.Duration(roundF(secs)) * time.Second)
}

func floorF(x float64) float64 {
	i := float64(int64(x))
	if x < 0 && i != x {
		i--
	}
	return i
}

func roundF(x float64) int64 {
	if x < 0 {
		return int64(x - 0.5)
	}
	return int64(x + 0.5)
}
