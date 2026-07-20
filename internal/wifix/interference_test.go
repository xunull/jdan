package wifix

import (
	"math"
	"strings"
	"testing"
)

// 功率换算必须是真的功率：每 10dB 是 10 倍，不是 2 倍。
//
// 设计文档 v1 写的是「每弱 10dB 减半」，即 2^(-Δ/10)。那会把 -42dBm 与
// -72dBm 的真实 1000 倍功率差压成 8 倍，系统性低估强干扰源 —— 严重到
// 可能把「推荐换到某信道」变成错误建议。这条用例把物理钉死。
func TestPowerConversion_10dBIs10x(t *testing.T) {
	p42 := dbmToMW(-42)
	p72 := dbmToMW(-72)
	ratio := p42 / p72

	if math.Abs(ratio-1000) > 1 {
		t.Errorf("-42dBm 与 -72dBm 的功率比 = %.1f，应为 1000（30dB = 10^3）", ratio)
	}
	if math.Abs(ratio-8) < 1 {
		t.Fatal("功率比算成了 8 —— 这是「每 10dB 减半」的错误模型，物理上错 125 倍")
	}
	// 3.01dB 才是 2 倍
	if r := dbmToMW(-40) / dbmToMW(-43.01); math.Abs(r-2) > 0.01 {
		t.Errorf("3.01dB 应为 2 倍功率，得到 %.3f", r)
	}
	// 往返
	for _, d := range []float64{-30, -50, -82, -95} {
		if got := mwToDBm(dbmToMW(d)); math.Abs(got-d) > 1e-9 {
			t.Errorf("dBm 往返失真：%v → %v", d, got)
		}
	}
	if !math.IsInf(mwToDBm(0), -1) {
		t.Error("零功率应转成负无穷 dBm")
	}
}

// 同信道走空口时间模型：只看是否越过 CCA 门限，不看相对强度。
// 强 10dB 和强 30dB 都是让你 defer，不是让你多 defer。
func TestAnalyze_CoChannelIsThresholdNotMagnitude(t *testing.T) {
	self := 36
	neighbors := []AP{
		{Band: Band5, Channel: 36, WidthMHz: 80, RSSI: -40}, // 远高于门限
		{Band: Band5, Channel: 36, WidthMHz: 80, RSSI: -75}, // 也高于门限
		{Band: Band5, Channel: 36, WidthMHz: 80, RSSI: -90}, // 低于门限，不触发退避
	}
	load := Analyze(Band5, self, 80, neighbors)

	if load.CoChannelBSS != 2 {
		t.Errorf("超过 CCA 门限(-82) 的同信道 BSS 应为 2，得到 %d", load.CoChannelBSS)
	}
	if load.CoChannelBelowCCA != 1 {
		t.Errorf("低于门限的应为 1，得到 %d", load.CoChannelBelowCCA)
	}
	// 关键：把 -40 换成 -60，计数不该变 —— 都在门限之上
	neighbors[0].RSSI = -60
	if l2 := Analyze(Band5, self, 80, neighbors); l2.CoChannelBSS != load.CoChannelBSS {
		t.Error("同信道计数不该随 RSSI 变化（只要都在门限之上）—— 这是空口时间模型的要点")
	}
}

// 邻信道走线性功率求和：两个等强邻居的合成噪声应比单个高 3dB。
func TestAnalyze_AdjacentChannelSumsPower(t *testing.T) {
	// 本机 36@80 = {36,40,44,48}；邻居 44@20 重叠度 0.25
	one := Analyze(Band5, 36, 80, []AP{
		{Band: Band5, Channel: 44, WidthMHz: 20, RSSI: -60},
	})
	two := Analyze(Band5, 36, 80, []AP{
		{Band: Band5, Channel: 44, WidthMHz: 20, RSSI: -60},
		{Band: Band5, Channel: 40, WidthMHz: 20, RSSI: -60},
	})

	diff := two.AdjNoiseDBm - one.AdjNoiseDBm
	if math.Abs(diff-3.0103) > 0.01 {
		t.Errorf("两个等强邻信道源合成应比单个高 3.01dB，实际高 %.3fdB", diff)
	}
}

// 无邻信道干扰时噪声是负无穷，不是 0 —— 0 dBm 是 1 毫瓦，是很强的信号。
func TestAnalyze_NoAdjacentMeansNegInf(t *testing.T) {
	load := Analyze(Band5, 149, 80, []AP{
		{Band: Band5, Channel: 36, WidthMHz: 80, RSSI: -40}, // 完全不重叠
	})
	if !math.IsInf(load.AdjNoiseDBm, -1) {
		t.Errorf("无邻道干扰应为 -Inf dBm，得到 %v（0 会被误读成 1mW 的强信号）", load.AdjNoiseDBm)
	}
	if load.CoChannelBSS != 0 {
		t.Errorf("不重叠的邻居不该计入同信道，得到 %d", load.CoChannelBSS)
	}
}

// 双模型的意义：很多个中等强度同信道 BSS 应该排在「一个强邻信道源」之后。
// 单一加权功率分会把前者算得更好，这正是要避免的错误建议。
func TestRank_ManyCoChannelBeatsOneStrongAdjacent(t *testing.T) {
	neighbors := []AP{
		// ch36 上四个中等强度同信道 BSS
		{Band: Band5, Channel: 36, WidthMHz: 80, RSSI: -70},
		{Band: Band5, Channel: 36, WidthMHz: 80, RSSI: -70},
		{Band: Band5, Channel: 36, WidthMHz: 80, RSSI: -70},
		{Band: Band5, Channel: 36, WidthMHz: 80, RSSI: -70},
		// ch149 块内一个很强的窄带邻居（部分重叠）
		{Band: Band5, Channel: 153, WidthMHz: 20, RSSI: -35},
	}
	loads := []ChannelLoad{
		Analyze(Band5, 36, 80, neighbors),
		Analyze(Band5, 149, 80, neighbors),
	}
	ranked := Rank(loads)

	if ranked[0].Channel != 149 {
		t.Errorf("149（0 个同信道 BSS）应排在 36（4 个同信道 BSS）之前，实际首位是 %d", ranked[0].Channel)
	}
	if ranked[0].CoChannelBSS != 0 {
		t.Errorf("149 的同信道数应为 0，得到 %d", ranked[0].CoChannelBSS)
	}
	if ranked[1].CoChannelBSS != 4 {
		t.Errorf("36 的同信道数应为 4，得到 %d", ranked[1].CoChannelBSS)
	}
}

// 排序必须确定：同分信道的顺序不能取决于输入顺序（输入来自 map 遍历）。
func TestRank_IsDeterministic(t *testing.T) {
	mk := func(ch, co int) ChannelLoad {
		return ChannelLoad{Channel: ch, Band: Band5, CoChannelBSS: co, AdjNoiseDBm: math.Inf(-1)}
	}
	a := []ChannelLoad{mk(161, 0), mk(36, 2), mk(149, 0), mk(153, 0)}
	b := []ChannelLoad{mk(153, 0), mk(149, 0), mk(36, 2), mk(161, 0)}

	ra, rb := Rank(a), Rank(b)
	for i := range ra {
		if ra[i].Channel != rb[i].Channel {
			t.Fatalf("同一份数据不同输入顺序得到不同排名：%v vs %v", chans(ra), chans(rb))
		}
	}
	// 同分时按信道号升序
	if got := chans(ra); got != "149,153,161,36" {
		t.Errorf("排名 = %s，同分应按信道号升序（36 因 co=2 排最后）", got)
	}
	// Rank 不得原地改动输入
	if a[0].Channel != 161 {
		t.Error("Rank 不该修改传入的 slice")
	}
}

func chans(ls []ChannelLoad) string {
	var sb strings.Builder
	for i, l := range ls {
		if i > 0 {
			sb.WriteByte(',')
		}
		sb.WriteString(itoa(l.Channel))
	}
	return sb.String()
}

func itoa(n int) string {
	if n == 0 {
		return "0"
	}
	neg := n < 0
	if neg {
		n = -n
	}
	var b []byte
	for n > 0 {
		b = append([]byte{byte('0' + n%10)}, b...)
		n /= 10
	}
	if neg {
		return "-" + string(b)
	}
	return string(b)
}

// 零样本信道要能区分「真的空」和「这次没扫到」。
// 实测同一时刻连跑 6 次邻居数波动 31%，单次为 0 完全可能是扫描盲区。
func TestTrulyEmpty_RequiresAllSamples(t *testing.T) {
	cases := []struct {
		seen, total int
		want        bool
	}{
		{0, 3, true},  // 三次都没见到 → 可以说空
		{1, 3, false}, // 见过一次 → 不能说空
		{0, 0, false}, // 没采过样 → 不能下结论
		{3, 3, false},
	}
	for _, c := range cases {
		l := ChannelLoad{SeenInSamples: c.seen, TotalSamples: c.total}
		if got := l.TrulyEmpty(); got != c.want {
			t.Errorf("seen=%d total=%d → TrulyEmpty=%v，应为 %v", c.seen, c.total, got, c.want)
		}
	}
}

// 跨频段的邻居不得互相干扰。
func TestAnalyze_IgnoresOtherBands(t *testing.T) {
	load := Analyze(Band5, 36, 80, []AP{
		{Band: Band24, Channel: 36, WidthMHz: 20, RSSI: -30}, // 2.4GHz 没有 ch36，但就算有也不该算
		{Band: Band6, Channel: 36, WidthMHz: 80, RSSI: -30},
	})
	if load.CoChannelBSS != 0 || !math.IsInf(load.AdjNoiseDBm, -1) {
		t.Errorf("跨频段不该产生干扰：co=%d adj=%v", load.CoChannelBSS, load.AdjNoiseDBm)
	}
}

// 2.4GHz 走连续重叠模型：ch1 的邻居在 ch3 应算邻信道（部分重叠），
// 在 ch1 应算同信道。
func TestAnalyze_24GHzUsesContinuousOverlap(t *testing.T) {
	load := Analyze(Band24, 1, 20, []AP{
		{Band: Band24, Channel: 1, WidthMHz: 20, RSSI: -50}, // 同信道
		{Band: Band24, Channel: 3, WidthMHz: 20, RSSI: -50}, // 重叠 0.6 → 邻信道
		{Band: Band24, Channel: 6, WidthMHz: 20, RSSI: -50}, // 不重叠
	})
	if load.CoChannelBSS != 1 {
		t.Errorf("同信道 BSS 应为 1，得到 %d", load.CoChannelBSS)
	}
	if math.IsInf(load.AdjNoiseDBm, -1) {
		t.Error("ch3 的部分重叠应产生邻道噪声")
	}
}
