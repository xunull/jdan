package jieqix

import "math"

const (
	// j2000 是 J2000.0 历元的儒略日：2000-01-01 12:00 TT。
	j2000 = 2451545.0
	// 儒略千年（VSOP87 的时间单位）与儒略世纪（章动公式的时间单位）。
	julianMillennium = 365250.0
	julianCentury    = 36525.0

	arcsec = math.Pi / (180 * 3600) // 1 角秒，弧度
	twoPi  = 2 * math.Pi
)

// vsopSeries 求一条 VSOP87 级数：Σ_p (Σ_i A·cos(B + C·τ)) · τ^p。
//
// 按幂次从高到低用 Horner 累加，避免先算出 τ^5 再乘上一个小系数——
// 本包声称不限年份，τ 在公元前会取到 −2 量级，直接展开会丢有效位。
func vsopSeries(series [][][3]float64, tau float64) float64 {
	sum := 0.0
	for p := len(series) - 1; p >= 0; p-- {
		s := 0.0
		for _, t := range series[p] {
			s += t[0] * math.Cos(t[1]+t[2]*tau)
		}
		sum = sum*tau + s
	}
	return sum
}

// earthLongitude 返回地球的日心黄经 L（弧度，归一到 [0, 2π)），
// jde 为力学时下的儒略日。
func earthLongitude(jde float64) float64 {
	return normRad(vsopSeries(vsopL, (jde-j2000)/julianMillennium))
}

// earthRadius 返回日地距离 R（天文单位）。光行差修正要用它。
func earthRadius(jde float64) float64 {
	return vsopSeries(vsopR, (jde-j2000)/julianMillennium)
}

// nutationLongitude 返回黄经章动 Δψ（弧度）。
//
// 地球自转轴在进动的基础上还有周期性摆动，主项周期 18.6 年、幅度 17.2″。
// 用 Meeus《Astronomical Algorithms》22 章的简化式，精度约 0.5″。
//
// 为什么 0.5″ 够：太阳每分钟走 2.46″，而验收标准是 1 分钟。0.5″ 折合
// 12 秒时间，占不到容差的 1/5。上完整的 IAU1980 63 项级数是过度工程。
func nutationLongitude(jde float64) float64 {
	t := (jde - j2000) / julianCentury
	deg := math.Pi / 180

	// 月球升交点黄经
	omega := (125.04452 - 1934.136261*t) * deg
	// 太阳平黄经
	ls := (280.4665 + 36000.7698*t) * deg
	// 月球平黄经
	lm := (218.3165 + 481267.8813*t) * deg

	return (-17.20*math.Sin(omega) -
		1.32*math.Sin(2*ls) -
		0.23*math.Sin(2*lm) +
		0.21*math.Sin(2*omega)) * arcsec
}

// apparentSolarLongitude 返回太阳的视黄经（弧度，归一到 [0, 2π)）。
// jde 为力学时（TT）下的儒略日。
//
// 「视」黄经是三步修正之后的结果，一步都不能少：
//
//	几何黄经  = 地球日心黄经 + 180°     ← 地心看太阳，与日心看地球差半圈
//	  ↓ −0.09033″                      VSOP87 动力学黄道 → FK5 参考系
//	  ↓ + Δψ                            章动（黄道本身在摆）
//	  ↓ −20.4898″/R                     光行差（光走过来要 8 分钟，
//	                                     这段时间地球动了）
//	视黄经
//
// 三项合计约 20.6″，折合时间 8.4 分钟。全漏掉的话交节时刻会整体偏
// 八分多钟——比 ΔT 那一步影响还大。
func apparentSolarLongitude(jde float64) float64 {
	theta := earthLongitude(jde) + math.Pi
	theta -= 0.09033 * arcsec
	theta += nutationLongitude(jde)
	theta -= 20.4898 * arcsec / earthRadius(jde)
	return normRad(theta)
}

// normRad 把弧度归一到 [0, 2π)。
func normRad(x float64) float64 {
	x = math.Mod(x, twoPi)
	if x < 0 {
		x += twoPi
	}
	return x
}

// wrapDiff 返回 a − b 折算到 (−π, π] 的差值。
//
// 求解节气时目标黄经可能是 0°（春分），而当前黄经可能是 359.9°。
// 直接相减得到 −359.9° 会把牛顿迭代甩出去一整年，所以必须走这里。
// 这就是「黄经跨 0°/360° 回绕」那条失败模式的防线。
func wrapDiff(a, b float64) float64 {
	d := math.Mod(a-b, twoPi)
	if d > math.Pi {
		d -= twoPi
	}
	if d <= -math.Pi {
		d += twoPi
	}
	return d
}
