package jieqix

import (
	"math"
	"strconv"
	"testing"
	"time"
)

// 第二层验证：1900–2100 全部 4824 个节气对 NAOJ 逐条 diff。
//
// 与第一层（solar_terms_ut.tsv，144 条手工锚点 + AstroPixels 交叉验证）分工：
//
//	第一层  少量锚点对权威源     验「算法整体对不对」
//	第二层  全区间对权威源       验「有没有只在某些年份区间显形的系统性偏移」
//
// 第一层抓不到的那类错，正是这一层的目标：比如 ΔT 分段多项式抄错了某一段，
// 而 6 个锚点年份恰好都不落在那一段里。
//
// 数据由 internal/jieqix/_tools/gen_anchors.go 一次性抓取并提交，
// 不需要重跑（那会向 NAOJ 的公益服务再发 201 次请求）。
func TestTerms_FullRangeAgainstNAOJ(t *testing.T) {
	rows := readAnchorTSV(t, "terms_1900_2100.tsv")
	if len(rows) != 4824 {
		t.Fatalf("全量表应有 4824 行（201 年 × 24），实得 %d", len(rows))
	}

	// 按年缓存，一年只算一次
	cache := map[int]map[string]Term{}
	getYear := func(y int) map[string]Term {
		if m, ok := cache[y]; ok {
			return m
		}
		ts, err := Terms(y)
		if err != nil {
			t.Fatalf("Terms(%d): %v", y, err)
		}
		m := make(map[string]Term, 24)
		for _, x := range ts {
			m[x.Name] = x
		}
		cache[y] = m
		return m
	}

	// 按年代分桶统计，让系统性偏移显形——如果某个区间整体偏，
	// 单看全局最大值会以为只是个别离群点。
	type bucket struct{ lo, hi int }
	buckets := []bucket{{1900, 1949}, {1950, 1999}, {2000, 2049}, {2050, 2100}}
	maxBy := map[bucket]float64{}
	sumBy := map[bucket]float64{}
	cntBy := map[bucket]int{}
	overBy := map[bucket]int{}

	var worst float64
	var worstDesc string
	years := map[int]bool{}

	for _, r := range rows {
		year, err := strconv.Atoi(r[0])
		if err != nil {
			t.Fatalf("年份列 %q: %v", r[0], err)
		}
		years[year] = true
		name := r[1]
		wantLon, err := strconv.Atoi(r[2])
		if err != nil {
			t.Fatalf("黄经列 %q: %v", r[2], err)
		}
		want, err := time.Parse("2006-01-02T15:04Z", r[3])
		if err != nil {
			t.Fatalf("时刻列 %q: %v", r[3], err)
		}

		got, ok := getYear(year)[name]
		if !ok {
			t.Errorf("%d 年没算出「%s」", year, name)
			continue
		}
		if got.Longitude != wantLon {
			t.Errorf("%d %s 黄经 = %d°, NAOJ = %d°", year, name, got.Longitude, wantLon)
		}
		gotMin := got.Time.Truncate(time.Minute)
		diff := math.Abs(gotMin.Sub(want).Minutes())

		// 日期不一致通常意味着整天级的错，值得单独报——但落在午夜前后
		// 容差窗口内的节气是例外：只差两三分钟，日期却跨了一天。
		// 4824 条里实测有 1 条（2086 立夏，本实现 05-04 23:58 / NAOJ 05-05 00:00）。
		// 所以只在时刻本身也超容差时才把日期当独立问题报，否则记一行日志。
		if gotMin.Format("2006-01-02") != want.Format("2006-01-02") {
			if diff > toleranceMinutes(year) {
				t.Errorf("%d %s 日期不一致：本实现 %s, NAOJ %s",
					year, name, gotMin.Format("2006-01-02 15:04"), want.Format("2006-01-02 15:04"))
			} else {
				t.Logf("跨午夜（时刻仍在容差内）：%d %s 本实现 %s / NAOJ %s",
					year, name, gotMin.Format("01-02 15:04"), want.Format("01-02 15:04"))
			}
		}
		for _, b := range buckets {
			if year >= b.lo && year <= b.hi {
				if diff > maxBy[b] {
					maxBy[b] = diff
				}
				sumBy[b] += diff
				cntBy[b]++
				if diff > toleranceMinutes(year) {
					overBy[b]++
				}
				break
			}
		}
		if diff > worst {
			worst = diff
			worstDesc = strconv.Itoa(year) + " " + name +
				" got=" + gotMin.Format("01-02 15:04") + " want=" + want.Format("01-02 15:04")
		}
		if lim := toleranceMinutes(year); diff > lim {
			t.Errorf("%d %s: 本实现 %s, NAOJ %s, 差 %.0f 分钟（容差 %.0f）",
				year, name, gotMin.Format("01-02 15:04"), want.Format("01-02 15:04"), diff, lim)
		}
	}

	if len(years) != 201 {
		t.Errorf("覆盖 %d 个年份，应为 201", len(years))
	}

	t.Logf("%-12s %8s %8s %8s %8s", "年代", "条数", "最大", "平均", "超容差")
	for _, b := range buckets {
		if cntBy[b] == 0 {
			continue
		}
		t.Logf("%d-%-7d %8d %6.1f分 %6.2f分 %8d",
			b.lo, b.hi, cntBy[b], maxBy[b], sumBy[b]/float64(cntBy[b]), overBy[b])
	}
	t.Logf("全局最大偏差: %.0f 分钟  %s", worst, worstDesc)
}
