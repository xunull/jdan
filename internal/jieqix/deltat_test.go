package jieqix

import (
	"bufio"
	"math"
	"os"
	"strconv"
	"strings"
	"testing"
)

// readAnchorTSV 读 testdata 下的锚点表，跳过 # 注释与空行。
// 锚点表的表头写了来源、抓取日期和交叉验证结果——那些注释是数据的一部分，
// 别改成 CSV 或 JSON 把它们丢掉。
func readAnchorTSV(t *testing.T, name string) [][]string {
	t.Helper()
	f, err := os.Open("testdata/" + name)
	if err != nil {
		t.Fatalf("打不开锚点表 %s: %v", name, err)
	}
	defer f.Close()

	var rows [][]string
	sc := bufio.NewScanner(f)
	sc.Buffer(make([]byte, 0, 64*1024), 1<<20)
	for sc.Scan() {
		line := sc.Text()
		if strings.TrimSpace(line) == "" || strings.HasPrefix(line, "#") {
			continue
		}
		rows = append(rows, strings.Split(line, "\t"))
	}
	if err := sc.Err(); err != nil {
		t.Fatalf("读 %s 出错: %v", name, err)
	}
	if len(rows) == 0 {
		t.Fatalf("%s 里一条数据都没有", name)
	}
	return rows
}

// 对 NAOJ 实测 ΔT 的偏差上限。
//
// 这不是「实现允许错多少」，是「两个权威 ΔT 模型之间本来差多少」。
// 本实现用 Espenak–Meeus，NAOJ 远期用 S2020，两者是不同的模型，
// 不可能也不应该逐秒相同。数字是实测出来的，不是拍的：
// 见 deltat.go 顶部注释里的对照表。
var deltaTTolerance = []struct {
	maxYear int
	limitS  float64
	why     string
}{
	{2005, 1.0, "E–M 与 S2020 在有观测数据的年代吻合到亚秒"},
	{2050, 10.0, "E–M 的 2005–2050 段是 2005 年拟合的，此后地球自转没按预期继续变慢"},
	{2150, 120.0, "远期外推：E–M 用 −20+32u² 抛物线，S2020 重新定标过"},
}

func toleranceFor(year int) (float64, string) {
	for _, b := range deltaTTolerance {
		if year <= b.maxYear {
			return b.limitS, b.why
		}
	}
	return 300.0, "超出所有已知模型的可信外推区间"
}

func TestDeltaT_AgainstNAOJAnchors(t *testing.T) {
	rows := readAnchorTSV(t, "deltat_ref.tsv")
	if len(rows) < 6 {
		t.Fatalf("deltat_ref.tsv 只有 %d 行，锚点表被截断了？", len(rows))
	}

	t.Logf("%-6s %10s %10s %9s  %s", "年", "本实现", "NAOJ", "差(s)", "NAOJ模型")
	for _, r := range rows {
		year, err := strconv.Atoi(r[0])
		if err != nil {
			t.Fatalf("年份列解析失败 %q: %v", r[0], err)
		}
		want, err := strconv.ParseFloat(r[1], 64)
		if err != nil {
			t.Fatalf("ΔT 列解析失败 %q: %v", r[1], err)
		}
		model := ""
		if len(r) > 2 {
			model = r[2]
		}

		// NAOJ 对一整年报一个值，取年中求值与之对应。
		got := DeltaT(float64(year) + 0.5)
		diff := got - want
		t.Logf("%-6d %10.2f %10.2f %+9.2f  %s", year, got, want, diff, model)

		limit, why := toleranceFor(year)
		if math.Abs(diff) > limit {
			t.Errorf("ΔT(%d) = %.2f s，NAOJ = %.2f s，差 %.2f s 超出容差 %.1f s\n  容差依据: %s",
				year, got, want, diff, limit, why)
		}
	}
}

// ΔT 必须连续：分段多项式的段边界如果系数抄错，会在边界上跳一个台阶。
// 逐年扫过所有段边界，相邻两年的差不该突然放大。
func TestDeltaT_ContinuousAcrossSegments(t *testing.T) {
	bounds := []int{500, 1600, 1700, 1800, 1860, 1900, 1920, 1941, 1961, 1986, 2005, 2050, 2150}
	for _, b := range bounds {
		lo := DeltaT(float64(b) - 1e-6)
		hi := DeltaT(float64(b) + 1e-6)
		jump := math.Abs(hi - lo)
		// Espenak–Meeus 各段是独立拟合的，边界本来就不严格连续，
		// 但正常的接缝是秒级。跳到几十秒就说明某段系数抄错了。
		if jump > 15 {
			t.Errorf("ΔT 在 %d 年边界跳变 %.2f s（%.2f -> %.2f），该段系数可能抄错",
				b, jump, lo, hi)
		}
	}
}

// ΔT 在 1700 年之后应当是单调上升的（地球自转持续变慢），
// 中间有小幅起伏但不该出现大段下降。抄错次数或正负号会破坏这个趋势。
func TestDeltaT_MonotonicTrendSince1700(t *testing.T) {
	prev := DeltaT(1700.5)
	worstDrop, worstYear := 0.0, 0
	for y := 1701; y <= 2150; y++ {
		cur := DeltaT(float64(y) + 0.5)
		if d := prev - cur; d > worstDrop {
			worstDrop, worstYear = d, y
		}
		prev = cur
	}
	if worstDrop > 3 {
		t.Errorf("ΔT 在 %d 年单年下降 %.2f s，超出正常起伏；系数或符号可能有误",
			worstYear, worstDrop)
	}
}

func TestDeltaTAt_UsesMonthFraction(t *testing.T) {
	// deltaTAt 要把「年 + 月」换成小数年 y = 年 + (月−0.5)/12。
	// 只传整数年会在段边界附近偏出零点几秒。
	if got, want := deltaTAt(2000, 1), DeltaT(2000+(1-0.5)/12); got != want {
		t.Errorf("deltaTAt(2000, 1) = %v, want %v", got, want)
	}
	jan := deltaTAt(2026, 1)
	dec := deltaTAt(2026, 12)
	if jan >= dec {
		t.Errorf("同一年内 1 月的 ΔT (%.3f) 应小于 12 月 (%.3f)", jan, dec)
	}
}
