package jieqix

import (
	"math"
	"strconv"
	"strings"
	"testing"
	"time"
)

// 时刻容差。不是「实现允许错多少」，是两个权威源之间本来差多少：
// NAOJ 与 AstroPixels 在 2026 年 4 个点里 3 个逐分一致、1 个差 1 分钟；
// 到 2100 年两者差 1–2 分钟（ΔT 外推模型不同）。
// 详见 testdata/solar_terms_ut.tsv 表头与 deltat.go 的对照表。
func toleranceMinutes(year int) float64 {
	if year <= 2050 {
		return 1
	}
	return 3
}

func TestTerms_AgainstNAOJAnchors(t *testing.T) {
	rows := readAnchorTSV(t, "solar_terms_ut.tsv")
	if len(rows) != 144 {
		t.Fatalf("锚点表应有 144 行（6 年 × 24），实得 %d 行", len(rows))
	}

	// 按年份缓存，避免每条锚点都重算一整年
	cache := map[int]map[string]Term{}
	getYear := func(y int) map[string]Term {
		if m, ok := cache[y]; ok {
			return m
		}
		ts, err := Terms(y)
		if err != nil {
			t.Fatalf("Terms(%d) 失败: %v", y, err)
		}
		m := make(map[string]Term, 24)
		for _, x := range ts {
			m[x.Name] = x
		}
		cache[y] = m
		return m
	}

	var worst float64
	var worstDesc string
	byYear := map[int]float64{}

	for _, r := range rows {
		year, err := strconv.Atoi(r[0])
		if err != nil {
			t.Fatalf("年份列 %q: %v", r[0], err)
		}
		name := r[1]
		wantLon, err := strconv.Atoi(r[3])
		if err != nil {
			t.Fatalf("黄经列 %q: %v", r[3], err)
		}
		want, err := time.Parse("2006-01-02T15:04Z", r[4])
		if err != nil {
			t.Fatalf("时刻列 %q: %v", r[4], err)
		}

		got, ok := getYear(year)[name]
		if !ok {
			t.Errorf("%d 年没算出「%s」", year, name)
			continue
		}
		if got.Longitude != wantLon {
			t.Errorf("%d %s 黄经 = %d°, 锚点 = %d°", year, name, got.Longitude, wantLon)
		}

		// 锚点只精确到分钟，本实现算到秒；比较时把实现值也取整到分钟。
		gotMin := got.Time.Truncate(time.Minute)
		diff := gotMin.Sub(want).Minutes()
		if a := math.Abs(diff); a > byYear[year] {
			byYear[year] = a
		}
		if a := math.Abs(diff); a > worst {
			worst, worstDesc = a, year1(year, name, gotMin, want)
		}

		if lim := toleranceMinutes(year); math.Abs(diff) > lim {
			t.Errorf("%d %s: 本实现 %s, NAOJ %s, 差 %+.0f 分钟（容差 %.0f）",
				year, name, gotMin.Format("01-02 15:04"), want.Format("01-02 15:04"), diff, lim)
		}
	}

	t.Logf("逐年最大偏差（分钟）:")
	for _, y := range []int{1900, 1929, 1950, 2000, 2026, 2100} {
		t.Logf("  %d: %.0f  (容差 %.0f)", y, byYear[y], toleranceMinutes(y))
	}
	t.Logf("全局最大偏差: %.0f 分钟  %s", worst, worstDesc)
}

func year1(y int, name string, got, want time.Time) string {
	return strconv.Itoa(y) + " " + name + " got=" + got.Format("01-02 15:04") +
		" want=" + want.Format("01-02 15:04")
}

// Terms(year) 必须只返回落在该公历年内的 24 个，且按时间升序。
func TestTerms_YearBoundaryAndOrdering(t *testing.T) {
	for _, y := range []int{1900, 2000, 2026, 2100} {
		ts, err := Terms(y)
		if err != nil {
			t.Fatalf("Terms(%d): %v", y, err)
		}
		if len(ts) != 24 {
			t.Errorf("Terms(%d) 返回 %d 个，want 24", y, len(ts))
		}
		for i, x := range ts {
			if x.Time.Year() != y {
				t.Errorf("Terms(%d)[%d] = %s 落在 %d 年，越界了",
					y, i, x.Name, x.Time.Year())
			}
			if i > 0 && !x.Time.After(ts[i-1].Time) {
				t.Errorf("Terms(%d) 未按时间升序：%s 不晚于 %s", y, x.Name, ts[i-1].Name)
			}
		}
	}
}

// 12 个「节」必须是黄经 315+30k，12 个「气」是 330+30k。
// 搞混会让月柱整体错半个月，而且看起来完全合理。
func TestTerms_MajorMinorSplit(t *testing.T) {
	ts, err := Terms(2026)
	if err != nil {
		t.Fatal(err)
	}
	majors, minors := 0, 0
	for _, x := range ts {
		if x.Major {
			majors++
			if (x.Longitude-315+360)%30 != 0 {
				t.Errorf("「%s」标为节，但黄经 %d° 不是 315+30k", x.Name, x.Longitude)
			}
		} else {
			minors++
			if (x.Longitude-330+360)%30 != 0 {
				t.Errorf("「%s」标为气，但黄经 %d° 不是 330+30k", x.Name, x.Longitude)
			}
		}
	}
	if majors != 12 || minors != 12 {
		t.Errorf("节/气数量 = %d/%d, want 12/12", majors, minors)
	}
	// 立春必须是节，且是 Index 0（干支月序的起点）
	for _, x := range ts {
		if x.Name == "立春" && (!x.Major || x.Index != 0) {
			t.Errorf("立春 Major=%v Index=%d, want true/0", x.Major, x.Index)
		}
	}
}

// 这是 D3 那条 P1 发现的回归测试：1 月上旬的日期，其前一个「节」在上一年。
// 实现如果只查当年，这里会拿到当年的小寒（还没到）或者直接找不到。
func TestPrevMajor_CrossYearLookback(t *testing.T) {
	cases := []struct {
		when time.Time
		want string
		year int // 期望这个节属于哪一年
	}{
		// 锚点：2026 小寒 = 01-05 08:23 UT。1/3 还没到，前一个节在 2025 年。
		{time.Date(2026, time.January, 3, 8, 0, 0, 0, time.UTC), "大雪", 2025},
		// 锚点：1950 小寒 = 01-05 21:39 UT。1/3 同样还没到。
		//
		// 这条最初写的是 1/6，预期大雪(1949)——错的：1/6 00:00 时小寒已过了
		// 2 小时 21 分。挑日期时没查锚点就凭「小寒大概在 1 月 5、6 号」写的。
		// 锚点表抓住了它。留着这段注释，因为下次还会有人这么写。
		{time.Date(1950, time.January, 3, 0, 0, 0, 0, time.UTC), "大雪", 1949},
		// 立春前：前一个节不是大寒（大寒是「气」，不分月），是小寒
		{time.Date(2026, time.February, 2, 0, 0, 0, 0, time.UTC), "小寒", 2026},
		// 立春之后：前一个节就是立春本身
		{time.Date(2026, time.February, 10, 0, 0, 0, 0, time.UTC), "立春", 2026},
	}
	for _, c := range cases {
		got, err := PrevMajor(c.when)
		if err != nil {
			t.Errorf("PrevMajor(%s): %v", c.when.Format("2006-01-02"), err)
			continue
		}
		if got.Name != c.want || got.Time.Year() != c.year {
			t.Errorf("PrevMajor(%s) = %s(%d), want %s(%d)",
				c.when.Format("2006-01-02"), got.Name, got.Time.Year(), c.want, c.year)
		}
		if !got.Major {
			t.Errorf("PrevMajor(%s) 返回了「气」%s，只能返回「节」",
				c.when.Format("2006-01-02"), got.Name)
		}
		if got.Time.After(c.when) {
			t.Errorf("PrevMajor(%s) 返回了未来的 %s", c.when.Format("2006-01-02"), got.Name)
		}
	}
}

// 交节时刻前后 1 分钟，PrevMajor 必须给出不同的节。
//
// 这是整个包最要紧的一条：交节当天正是用户最可能来查的日子，也是唯一会算错的日子。
//
// 支点用本实现算出的交节时刻，不是锚点时刻——这里测的是「PrevMajor 与 Terms
// 是否在同一瞬间翻」这个内部不变量，不是「我们准不准」。后者由
// TestTerms_AgainstNAOJAnchors 管，两件事分开。
//
// 混在一起会出错：2100 年本实现比 NAOJ 早 3 分钟（ΔT 模型差异，见 deltat.go），
// 拿锚点时刻当支点、用 ±2 分钟窗口，会因为跨不过这个系统性偏移而失败——
// 失败原因和这条测试想测的东西毫无关系。
func TestPrevMajor_AcrossExactBoundary(t *testing.T) {
	cases := []struct {
		year       string
		termYear   int
		term       string // 支点：这个节
		before     string // 其前 1 分钟，PrevMajor 应给出
		beforeYear int
	}{
		{"2026 立春", 2026, "立春", "小寒", 2026},
		{"1950 小寒（跨年）", 1950, "小寒", "大雪", 1949},
		{"1900 立春（1929 前）", 1900, "立春", "小寒", 1900},
		{"2100 小寒（跨世纪上界，跨年）", 2100, "小寒", "大雪", 2099},
	}
	for _, c := range cases {
		ts, err := Terms(c.termYear)
		if err != nil {
			t.Fatalf("%s: Terms(%d): %v", c.year, c.termYear, err)
		}
		var pivot time.Time
		for _, x := range ts {
			if x.Name == c.term {
				pivot = x.Time
			}
		}
		if pivot.IsZero() {
			t.Fatalf("%s: %d 年没算出「%s」", c.year, c.termYear, c.term)
		}

		lo, err := PrevMajor(pivot.Add(-time.Minute))
		if err != nil {
			t.Fatalf("%s -1min: %v", c.year, err)
		}
		hi, err := PrevMajor(pivot.Add(time.Minute))
		if err != nil {
			t.Fatalf("%s +1min: %v", c.year, err)
		}

		if lo.Name != c.before || lo.Time.Year() != c.beforeYear {
			t.Errorf("%s（%s）前 1 分钟：PrevMajor = %s(%d), want %s(%d)",
				c.year, pivot.Format("01-02 15:04:05"),
				lo.Name, lo.Time.Year(), c.before, c.beforeYear)
		}
		if hi.Name != c.term {
			t.Errorf("%s（%s）后 1 分钟：PrevMajor = %s, want %s",
				c.year, pivot.Format("01-02 15:04:05"), hi.Name, c.term)
		}
		// 交节当刻本身应当已经算「进入」新的节
		at, err := PrevMajor(pivot)
		if err != nil {
			t.Fatalf("%s at: %v", c.year, err)
		}
		if at.Name != c.term {
			t.Errorf("%s 交节当刻：PrevMajor = %s, want %s（边界应含当刻）",
				c.year, at.Name, c.term)
		}
	}
}

// 春分目标黄经是 0°，冬至前后当前黄经接近 360°。
// 如果求解器直接相减而不做回绕处理，牛顿迭代会被甩出一整年。
func TestSolveTerm_WrapsAroundZero(t *testing.T) {
	ts, err := Terms(2026)
	if err != nil {
		t.Fatal(err)
	}
	var chunfen time.Time
	for _, x := range ts {
		if x.Name == "春分" {
			chunfen = x.Time
		}
	}
	if chunfen.IsZero() {
		t.Fatal("没算出春分")
	}
	// 锚点：2026 春分 UT 03-20 14:46
	want := time.Date(2026, time.March, 20, 14, 46, 0, 0, time.UTC)
	if d := chunfen.Truncate(time.Minute).Sub(want); d < -time.Minute || d > time.Minute {
		t.Errorf("2026 春分 = %s, 锚点 %s（差 %v）",
			chunfen.Format(time.RFC3339), want.Format(time.RFC3339), d)
	}
}

func TestWrapDiff(t *testing.T) {
	cases := []struct{ a, b, want float64 }{
		{0.1, 0.2, -0.1},
		{0, twoPi - 0.1, 0.1}, // 目标 0°、当前 359.9° → 只差 +0.1，不是 −359.9
		{twoPi - 0.1, 0, -0.1},
		{math.Pi, 0, math.Pi},
	}
	for _, c := range cases {
		if got := wrapDiff(c.a, c.b); math.Abs(got-c.want) > 1e-12 {
			t.Errorf("wrapDiff(%v, %v) = %v, want %v", c.a, c.b, got, c.want)
		}
	}
}

func TestVerified(t *testing.T) {
	for _, c := range []struct {
		y  int
		ok bool
	}{{1899, false}, {1900, true}, {2026, true}, {2100, true}, {2101, false}} {
		if got := Verified(c.y); got != c.ok {
			t.Errorf("Verified(%d) = %v, want %v", c.y, got, c.ok)
		}
	}
	lo, hi := VerifiedRange()
	if lo != 1900 || hi != 2100 {
		t.Errorf("VerifiedRange() = %d..%d, want 1900..2100", lo, hi)
	}
}

func TestNext(t *testing.T) {
	// 2026-12-25 之后的下一个节气是 2027 年的小寒——必须能跨年找到。
	got, err := Next(time.Date(2026, time.December, 25, 0, 0, 0, 0, time.UTC))
	if err != nil {
		t.Fatal(err)
	}
	if got.Name != "小寒" || got.Time.Year() != 2027 {
		t.Errorf("Next(2026-12-25) = %s(%d), want 小寒(2027)", got.Name, got.Time.Year())
	}
}

// 求解器不收敛时必须返回 error，不能返回最后一次迭代值。
//
// 这条是 /plan-eng-review 的 T5 明确要求的，但第一版只写了实现没写测试——
// 因为用真实的太阳函数，牛顿迭代从任意起点都在 3-4 步内收敛，
// 那条 error 路径够不到。所以把黄经函数抽成参数，这里注入病态函数把它走出来。
func TestSolveLongitude_NonConvergenceReturnsError(t *testing.T) {
	// 病态函数：每次调用都跳到一个和目标差 π 的位置，迭代永远追不上。
	never := func(float64) float64 { return math.Pi }

	got, err := solveLongitude(0, 2451545.0, never)
	if err == nil {
		t.Fatalf("不收敛时应返回 error，实得 jde=%v", got)
	}
	if got != 0 {
		t.Errorf("失败时应返回零值而不是最后一次迭代值，实得 %v", got)
	}
	if !strings.Contains(err.Error(), "未收敛") {
		t.Errorf("错误信息应说明未收敛，实得 %q", err)
	}

	// 反面：正常函数必须收敛，且远在迭代上限之内。
	calls := 0
	counted := func(jde float64) float64 { calls++; return apparentSolarLongitude(jde) }
	if _, err := solveLongitude(0, 2451545.0, counted); err != nil {
		t.Fatalf("真实太阳函数不该不收敛: %v", err)
	}
	if calls >= maxIter {
		t.Errorf("收敛用了 %d 次迭代，逼近上限 %d —— 余量不足", calls, maxIter)
	}
	t.Logf("真实函数收敛用了 %d 次迭代（上限 %d）", calls, maxIter)
}

// TermAt 返回 t 所处节气区间的起点，与 PrevMajor 不同：它不筛「节」，
// 中气也算。这个区别没测的话，两个函数很容易被实现成同一个东西。
func TestTermAt(t *testing.T) {
	// 2026 大寒（气）= 01-20 01:45 UT，立春（节）= 02-03 20:02 UT
	at := time.Date(2026, time.January, 25, 0, 0, 0, 0, time.UTC)

	cur, err := TermAt(at)
	if err != nil {
		t.Fatal(err)
	}
	if cur.Name != "大寒" {
		t.Errorf("TermAt(01-25) = %s, want 大寒（此刻所处的节气区间起点）", cur.Name)
	}
	if cur.Major {
		t.Errorf("大寒是「气」，Major 应为 false")
	}

	// 同一时刻 PrevMajor 要跳过中气，给出小寒
	pm, err := PrevMajor(at)
	if err != nil {
		t.Fatal(err)
	}
	if pm.Name != "小寒" {
		t.Errorf("PrevMajor(01-25) = %s, want 小寒（跳过中气大寒）", pm.Name)
	}
	if cur.Name == pm.Name {
		t.Error("TermAt 与 PrevMajor 返回了同一个节气——中气筛选没起作用")
	}
	if cur.Time.After(at) {
		t.Error("TermAt 返回了未来的节气")
	}
}

// nutationLongitude 只被 apparentSolarLongitude 间接用到，
// 单独钉一下量级与周期，免得系数抄错时只表现为「节气差几秒」而查不到源头。
func TestNutationLongitude_MagnitudeAndPeriod(t *testing.T) {
	// 主项幅度 17.2″，加上次项，|Δψ| 不应超过约 20″。
	maxAbs, at := 0.0, 0.0
	for d := 0.0; d < 365.25*20; d += 10 { // 扫 20 年，覆盖 18.6 年主周期
		jde := j2000 + d
		if v := math.Abs(nutationLongitude(jde)); v > maxAbs {
			maxAbs, at = v, jde
		}
	}
	arcsecMax := maxAbs / arcsec
	if arcsecMax < 15 || arcsecMax > 21 {
		t.Errorf("20 年内 |Δψ| 峰值 = %.2f″（JDE %.1f），预期落在 15–21″", arcsecMax, at)
	}
	t.Logf("20 年内 |Δψ| 峰值 %.2f″", arcsecMax)

	// 18.6 年主周期：相隔约 9.3 年（半周期）符号应当相反。
	a := nutationLongitude(j2000)
	b := nutationLongitude(j2000 + 365.25*9.3)
	if a*b > 0 {
		t.Errorf("相隔半个主周期（9.3 年）符号应相反，实得 %.3e 与 %.3e", a, b)
	}
}
