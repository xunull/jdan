package jieqix

import (
	"fmt"
	"math"
	"time"
)

// Term 是一个节气。
type Term struct {
	Index     int       // 0..23，0 = 立春（与干支月序对齐，不是与公历对齐）
	Name      string    // 立春
	Longitude int       // 太阳视黄经，度
	Major     bool      // true = 节（月令起点），false = 气（中气）
	Time      time.Time // 精确时刻，UTC
}

// 二十四节气，按干支月序排列：寅月起于立春，所以 Index 0 是立春。
//
// Major（节）是黄经 315° + 30°k 那 12 个，它们才是月柱的分界；
// Minor（气／中气）是夹在中间的 12 个，跟月柱无关。
// 这两者搞混，月柱会整体错半个月，而且看起来完全合理。
var termDefs = [24]struct {
	name string
	lon  int
}{
	{"立春", 315}, {"雨水", 330}, {"惊蛰", 345}, {"春分", 0},
	{"清明", 15}, {"谷雨", 30}, {"立夏", 45}, {"小满", 60},
	{"芒种", 75}, {"夏至", 90}, {"小暑", 105}, {"大暑", 120},
	{"立秋", 135}, {"处暑", 150}, {"白露", 165}, {"秋分", 180},
	{"寒露", 195}, {"霜降", 210}, {"立冬", 225}, {"小雪", 240},
	{"大雪", 255}, {"冬至", 270}, {"小寒", 285}, {"大寒", 300},
}

// isMajor 判断第 i 个节气是不是「节」。
// termDefs 按立春起排，立春本身是节，之后节气交替，所以偶数下标即为节。
func isMajor(i int) bool { return i%2 == 0 }

// 已验证区间：testdata/solar_terms_ut.tsv 实际覆盖 1900–2100（6 个年份 × 24）。
//
// 区间外仍会算——本包是纯算法，没有表的硬边界——但精度未经锚点验证：
// 截断级数误差随离 J2000 增大，ΔT 在 1620 前靠史料推算、2100 后靠外推
// （见 deltat.go：2100 年本实现与 NAOJ 差 101 秒）。
const (
	verifiedFrom = 1900
	verifiedTo   = 2100
)

// Verified 报告 year 是否落在锚点实际覆盖的区间内。
// 上层 CLI 据此在 text 输出加警告行、在 --json 加 verified 字段。
func Verified(year int) bool { return year >= verifiedFrom && year <= verifiedTo }

// VerifiedRange 返回已验证区间，供 CLI 组装提示文案。
func VerifiedRange() (int, int) { return verifiedFrom, verifiedTo }

// 牛顿迭代参数。
const (
	maxIter   = 30
	tolRad    = 1e-11     // 收敛判据：黄经残差（弧度）。1e-11 rad ≈ 2e-6″ ≈ 1 微秒时间
	degPerDay = 0.9856473 // 太阳日均视运动，用作牛顿迭代的导数近似
)

// solveLongitude 用牛顿迭代求 lambda(jde) == target 的时刻。
//
// 把黄经函数作为参数传入，而不是直接调 apparentSolarLongitude：
// 用真实的太阳函数时，迭代从任意起点都在 3-4 步内收敛，那条不收敛的
// error 路径根本够不到，也就无法验证它是不是真的返回 error。
// 注入之后测试可以传一个病态函数把那条路径走出来。
//
// 迭代不收敛时返回 error，不返回最后一次迭代值——静默返回一个垃圾数字
// 会让上层拿到一个看起来完全合理的错误时刻，这正是本包最怕的失败模式。
func solveLongitude(target, guessJDE float64, lambda func(float64) float64) (float64, error) {
	rate := degPerDay * math.Pi / 180 // 弧度/天

	jde := guessJDE
	for range maxIter {
		diff := wrapDiff(target, lambda(jde))
		if math.Abs(diff) < tolRad {
			return jde, nil
		}
		jde += diff / rate
	}
	return 0, fmt.Errorf("求解黄经 %.4f rad 未收敛（起点 JDE %.5f，迭代 %d 次）",
		target, guessJDE, maxIter)
}

// solveTerm 求太阳视黄经等于 targetDeg 的时刻（TT 儒略日），
// 从 guessJDE 出发向后找最近的一次。
func solveTerm(targetDeg int, guessJDE float64) (float64, error) {
	jde, err := solveLongitude(float64(targetDeg)*math.Pi/180, guessJDE, apparentSolarLongitude)
	if err != nil {
		return 0, fmt.Errorf("黄经 %d°: %w", targetDeg, err)
	}
	return jde, nil
}

// ttToUTC 把力学时儒略日换成 UTC 时刻：UT = TT − ΔT。
//
// 漏这一步是分钟级的静默系统性偏移。见 deltat.go。
func ttToUTC(jde float64) time.Time {
	// ΔT 要按所在年月求值，而年月又要先知道时刻——先用不含修正的时刻
	// 定出年月，再求 ΔT。ΔT 本身以秒计，一次就够，不需要迭代。
	approx := fromJulianDay(jde)
	dt := deltaTAt(approx.Year(), int(approx.Month()))
	return fromJulianDay(jde - dt/86400)
}

// termsFrom 返回从 fromJDE 起算、每个目标黄经的下一次到达时刻。
func termsFrom(fromJDE float64) ([24]Term, error) {
	var out [24]Term
	lon0 := apparentSolarLongitude(fromJDE) * 180 / math.Pi

	for i, d := range termDefs {
		// 从起点黄经到目标黄经还要走多少度（沿运动方向，取正）
		ahead := math.Mod(float64(d.lon)-lon0, 360)
		if ahead < 0 {
			ahead += 360
		}
		guess := fromJDE + ahead/degPerDay

		jde, err := solveTerm(d.lon, guess)
		if err != nil {
			return out, fmt.Errorf("%s: %w", d.name, err)
		}
		out[i] = Term{
			Index:     i,
			Name:      d.name,
			Longitude: d.lon,
			Major:     isMajor(i),
			Time:      ttToUTC(jde),
		}
	}
	return out, nil
}

// Terms 返回落在公历 year 年内的 24 个节气，按时间先后排序。
//
// 注意语义：这是「今年的节气表」，给 CLI 用的。
// 月柱推算不要用它——那要的是「立春到立春」那一轮，用 PrevMajor。
// 两个语义分成两个函数，是因为掩盖其中一个只会把矛盾推到调用方：
// 1 月 1 日到小寒之间的日期，其前一个节在上一年，每年有 5 天会静默算错。
func Terms(year int) ([]Term, error) {
	start := float64(JDN(year, time.January, 1)) - 0.5
	ts, err := termsFrom(start)
	if err != nil {
		return nil, err
	}
	out := make([]Term, 0, 24)
	out = append(out, ts[:]...)
	sortByTime(out)
	return out, nil
}

// PrevMajor 返回 t 之前（含 t）最近的一个「节」。
//
// 内部自行跨年：1 月上旬的日期，其前一个节是上一年的大雪，
// 此时会同时算 year−1 和 year 两轮。调用方不需要知道这件事。
func PrevMajor(t time.Time) (Term, error) {
	return prevMatching(t, func(x Term) bool { return x.Major })
}

// TermAt 返回 t 所处节气区间的起点（含 t 当刻）。
func TermAt(t time.Time) (Term, error) {
	return prevMatching(t, func(Term) bool { return true })
}

func prevMatching(t time.Time, keep func(Term) bool) (Term, error) {
	t = t.UTC()
	// 往前多取一年：1 月上旬要回溯到上一年年末的节。
	var cands []Term
	for _, y := range []int{t.Year() - 1, t.Year()} {
		ts, err := Terms(y)
		if err != nil {
			return Term{}, err
		}
		for _, x := range ts {
			if keep(x) && !x.Time.After(t) {
				cands = append(cands, x)
			}
		}
	}
	if len(cands) == 0 {
		return Term{}, fmt.Errorf("%s 之前找不到节气（超出可算范围？）", t.Format(time.RFC3339))
	}
	best := cands[0]
	for _, x := range cands[1:] {
		if x.Time.After(best.Time) {
			best = x
		}
	}
	return best, nil
}

// Next 返回 t 之后最近的一个节气。
func Next(t time.Time) (Term, error) {
	t = t.UTC()
	for _, y := range []int{t.Year(), t.Year() + 1} {
		ts, err := Terms(y)
		if err != nil {
			return Term{}, err
		}
		for _, x := range ts {
			if x.Time.After(t) {
				return x, nil
			}
		}
	}
	return Term{}, fmt.Errorf("%s 之后找不到节气", t.Format(time.RFC3339))
}

func sortByTime(ts []Term) {
	for i := 1; i < len(ts); i++ {
		for j := i; j > 0 && ts[j].Time.Before(ts[j-1].Time); j-- {
			ts[j], ts[j-1] = ts[j-1], ts[j]
		}
	}
}
