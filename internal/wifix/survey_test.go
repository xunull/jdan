package wifix

import (
	"errors"
	"os"
	"slices"
	"testing"
)

func fixtureBytes(t *testing.T) []byte {
	t.Helper()
	b, err := os.ReadFile("testdata/airport-macos26.plist")
	if err != nil {
		t.Fatal(err)
	}
	return b
}

func TestCollect_MergesAcrossSamples(t *testing.T) {
	data := fixtureBytes(t)
	calls := 0
	sv, err := Collect(func() ([]byte, error) { calls++; return data, nil }, 3, "")
	if err != nil {
		t.Fatal(err)
	}
	if calls != 3 {
		t.Errorf("应扫描 3 次，实际 %d 次", calls)
	}
	if sv.Samples != 3 {
		t.Errorf("Samples = %d，应为 3", sv.Samples)
	}
	// 同一份数据扫 3 次，每个 AP 都该被看到 3 次（不该被当成 3 个不同 AP）
	for _, n := range sv.Neighbors {
		if n.SeenCount != 3 {
			t.Errorf("ch%d 的 AP SeenCount = %d，同一份数据应为 3（去重失败？）", n.Channel, n.SeenCount)
		}
		if n.Intermittent(3) {
			t.Errorf("ch%d 的 AP 不该被标为间歇可见", n.Channel)
		}
	}
}

// 采样次数不同的 AP 要能区分出来 —— 这是「间歇可见」标注的依据。
func TestCollect_TracksIntermittentAPs(t *testing.T) {
	full := fixtureBytes(t)
	// 第二次返回一份「少了邻居」的数据：把 others 数组清空
	trimmed := stripNeighbors(t, full)

	n := 0
	sv, err := Collect(func() ([]byte, error) {
		n++
		if n == 2 {
			return trimmed, nil
		}
		return full, nil
	}, 3, "")
	if err != nil {
		t.Fatal(err)
	}
	if sv.Samples != 3 {
		t.Fatalf("Samples = %d", sv.Samples)
	}
	// 邻居在 3 次里只出现 2 次
	for _, a := range sv.Neighbors {
		if a.SeenCount != 2 {
			t.Errorf("AP 应在 3 次采样中出现 2 次，得到 %d", a.SeenCount)
		}
		if !a.Intermittent(3) {
			t.Error("出现 2/3 次的 AP 应标为间歇可见")
		}
	}
	// 但并集仍要保留它们 —— 单次没扫到不代表不存在
	if len(sv.Neighbors) == 0 {
		t.Error("并集不该丢掉只在部分采样中出现的 AP")
	}
}

// stripNeighbors 造一份没有邻居的 plist（模拟扫描盲区）。
func stripNeighbors(t *testing.T, data []byte) []byte {
	t.Helper()
	// 直接在 XML 里把 others 数组换成空数组
	s := string(data)
	key := "<key>spairport_airport_other_local_wireless_networks</key>"
	i := indexOf(s, key)
	if i < 0 {
		t.Skip("fixture 里没有邻居数组")
	}
	j := indexOf(s[i:], "</array>")
	if j < 0 {
		t.Skip("找不到数组结尾")
	}
	return []byte(s[:i] + key + "<array></array>" + s[i+j+len("</array>"):])
}

func indexOf(s, sub string) int {
	for i := 0; i+len(sub) <= len(s); i++ {
		if s[i:i+len(sub)] == sub {
			return i
		}
	}
	return -1
}

// 任何一次采样失败都直接返回错误。部分成功会让 SeenCount/Samples 的比值
// 失去意义（分母不对），比干脆失败更糟。
func TestCollect_FailsFastOnScanError(t *testing.T) {
	data := fixtureBytes(t)
	boom := errors.New("扫描挂了")
	n := 0
	_, err := Collect(func() ([]byte, error) {
		n++
		if n == 2 {
			return nil, boom
		}
		return data, nil
	}, 3, "")
	if err == nil {
		t.Fatal("采样失败应返回错误而不是给出部分结果")
	}
	if !errors.Is(err, boom) {
		t.Errorf("错误应包装原始错误，得到 %v", err)
	}
}

func TestCollect_NeighborOrderIsDeterministic(t *testing.T) {
	data := fixtureBytes(t)
	scan := func() ([]byte, error) { return data, nil }

	var first []string
	for i := range 5 {
		sv, err := Collect(scan, 2, "")
		if err != nil {
			t.Fatal(err)
		}
		var order []string
		for _, n := range sv.Neighbors {
			order = append(order, n.Band.String()+"/"+itoa(n.Channel)+"/"+itoa(n.RSSI))
		}
		if i == 0 {
			first = order
			continue
		}
		if !slices.Equal(order, first) {
			t.Fatalf("第 %d 次的邻居顺序与第 1 次不同 —— map 遍历顺序泄漏了", i)
		}
	}
}

// ---- 信道分析编排 ----

func TestAnalyzeBand_OnlyConsidersSupportedChannels(t *testing.T) {
	sv := mustCollect(t, 1)

	rep := AnalyzeBand(sv, Band5, 80)
	supported := sv.SupportedIn(Band5)
	if len(rep.Loads) == 0 {
		t.Fatal("应产出候选信道")
	}
	for _, l := range rep.Loads {
		if !slices.Contains(supported, l.Channel) {
			t.Errorf("候选信道 %d 不在本机支持列表里 —— 这条同时是区域合规兜底", l.Channel)
		}
	}
	// 本机 country=CN，不支持 100-144，不该出现在候选里
	for _, l := range rep.Loads {
		if l.Channel >= 100 && l.Channel <= 144 {
			t.Errorf("CN 不支持 ch%d，不该作为候选", l.Channel)
		}
	}
}

// 频段来自解析时保留的 band 字符串，不是按信道号范围反推的。
func TestSurvey_SupportedInUsesParsedBand(t *testing.T) {
	sv := mustCollect(t, 1)

	ch24 := sv.SupportedIn(Band24)
	ch5 := sv.SupportedIn(Band5)
	if len(ch24) == 0 || len(ch5) == 0 {
		t.Fatal("2.4GHz 和 5GHz 都该有支持的信道")
	}
	for _, c := range ch24 {
		if c > 14 {
			t.Errorf("2.4GHz 列表里出现了 ch%d", c)
		}
	}
	for _, c := range ch5 {
		if c < 36 {
			t.Errorf("5GHz 列表里出现了 ch%d", c)
		}
	}
	// 交集必须为空
	for _, c := range ch24 {
		if slices.Contains(ch5, c) {
			t.Errorf("ch%d 同时出现在两个频段", c)
		}
	}
}

// 只有明显更优才推荐。「差不多」时不给建议 —— 换信道有成本（DFS 更甚），
// 给个边际收益的建议比不给更糟。
func TestAnalyzeBand_NoRecommendationWhenNotClearlyBetter(t *testing.T) {
	sv := &Survey{
		Samples: 3,
		Current: &Network{Band: Band5, Channel: 36, WidthMHz: 80, RSSI: -41, Noise: -88},
		SupportedChannels: []ChannelSpec{
			{Band: Band5, Channel: 36}, {Band: Band5, Channel: 149},
		},
		// 两个信道各一个同信道 BSS —— 一样糟，不该推荐换
		Neighbors: []SurveyAP{
			{Network: Network{Band: Band5, Channel: 36, WidthMHz: 80, RSSI: -50}, SeenCount: 3},
			{Network: Network{Band: Band5, Channel: 149, WidthMHz: 80, RSSI: -50}, SeenCount: 3},
		},
	}
	rep := AnalyzeBand(sv, Band5, 80)
	if rep.Best != nil {
		t.Errorf("同信道数相同时不该推荐换信道，却推荐了 ch%d", rep.Best.Channel)
	}
	if rep.SelfLoad == nil {
		t.Fatal("应识别出本机当前信道")
	}
	if rep.SelfLoad.Channel != 36 {
		t.Errorf("本机信道 = %d，应为 36", rep.SelfLoad.Channel)
	}

	// 149 上没有 BSS 时才该推荐
	sv.Neighbors = sv.Neighbors[:1]
	if rep2 := AnalyzeBand(sv, Band5, 80); rep2.Best == nil || rep2.Best.Channel != 149 {
		t.Errorf("149 明显更空时应推荐它，得到 %v", rep2.Best)
	}
}

// 零样本信道要能区分「真的空」和「这次没扫到」。
func TestAnalyzeBand_ZeroSampleConfidence(t *testing.T) {
	sv := &Survey{
		Samples: 3,
		Current: &Network{Band: Band5, Channel: 36, WidthMHz: 80},
		SupportedChannels: []ChannelSpec{
			{Band: Band5, Channel: 36}, {Band: Band5, Channel: 149},
		},
		Neighbors: []SurveyAP{
			{Network: Network{Band: Band5, Channel: 36, WidthMHz: 80, RSSI: -50}, SeenCount: 3},
		},
	}
	rep := AnalyzeBand(sv, Band5, 80)

	var ch149 *ChannelLoad
	for i := range rep.Loads {
		if rep.Loads[i].Channel == 149 {
			ch149 = &rep.Loads[i]
		}
	}
	if ch149 == nil {
		t.Fatal("149 应在候选里")
	}
	if ch149.TotalSamples != 3 {
		t.Errorf("TotalSamples = %d，应为 3", ch149.TotalSamples)
	}
	if !ch149.TrulyEmpty() {
		t.Error("3 次采样都没在 149 上见到 AP，应可判为真空")
	}

	// 只采样 1 次时，即使为 0 也是可判的（1 次全为 0），但语义弱得多 ——
	// 这就是为什么默认采样 3 次
	sv.Samples = 1
	rep1 := AnalyzeBand(sv, Band5, 80)
	for _, l := range rep1.Loads {
		if l.Channel == 149 && l.TotalSamples != 1 {
			t.Errorf("单次采样时 TotalSamples 应为 1，得到 %d", l.TotalSamples)
		}
	}
}

func mustCollect(t *testing.T, n int) *Survey {
	t.Helper()
	data := fixtureBytes(t)
	sv, err := Collect(func() ([]byte, error) { return data, nil }, n, "")
	if err != nil {
		t.Fatal(err)
	}
	return sv
}

// 端到端：真机 fixture → 采样 → 分析 → 推荐。
func TestAnalyzeBand_EndToEndOnFixture(t *testing.T) {
	sv := mustCollect(t, 3)
	if sv.Current == nil {
		t.Fatal("fixture 应有当前连接")
	}
	rep := AnalyzeBand(sv, Band5, sv.Current.WidthMHz)

	t.Logf("本机 ch%d@%dMHz", sv.Current.Channel, sv.Current.WidthMHz)
	for _, l := range rep.Loads[:min(5, len(rep.Loads))] {
		star := " "
		if rep.SelfLoad != nil && l.Channel == rep.SelfLoad.Channel {
			star = "★"
		}
		t.Logf("  %s ch%-4d 同信道 %d 个（低于门限 %d）  邻道噪声 %.1f dBm  真空=%v",
			star, l.Channel, l.CoChannelBSS, l.CoChannelBelowCCA, l.AdjNoiseDBm, l.TrulyEmpty())
	}
	if rep.Best != nil {
		t.Logf("推荐换到 ch%d（同信道 %d 个）DFS=%v",
			rep.Best.Channel, rep.Best.CoChannelBSS, IsDFS(Band5, rep.Best.Channel))
	} else {
		t.Log("无明显更优信道，不给建议")
	}
}

// 回归：可见性统计与干扰计算必须用同一套重叠判据。
//
// 实测踩到过「同信道 3 个 BSS」和「三次采样都没见到 AP」同时成立 ——
// 成因是按邻居的 primary 信道查表，而 160MHz 邻居的 primary 在 44、
// 实际覆盖到 52，干扰模型算到了它、可见性统计没算到。
func TestAnalyzeBand_SeenCountUsesSameOverlapAsInterference(t *testing.T) {
	sv := mustCollect(t, 3)
	rep := AnalyzeBand(sv, Band5, 80)

	for _, l := range rep.Loads {
		hasInterference := l.CoChannelBSS > 0 || l.CoChannelBelowCCA > 0
		if hasInterference && l.TrulyEmpty() {
			t.Errorf("ch%d 自相矛盾：同信道 %d 个（低于门限 %d），却判为三次采样都没见到 AP",
				l.Channel, l.CoChannelBSS, l.CoChannelBelowCCA)
		}
	}
}

// 承载不了当前带宽的信道不该进候选，更不该被推荐。
func TestAnalyzeBand_ExcludesChannelsThatCannotHostWidth(t *testing.T) {
	sv := mustCollect(t, 1)

	rep80 := AnalyzeBand(sv, Band5, 80)
	for _, l := range rep80.Loads {
		if l.Channel == 165 {
			t.Error("本机 80MHz 时 ch165 不该进候选 —— 它是 20MHz-only 的孤立信道，" +
				"换过去带宽掉 4 倍，而它显得空恰恰因为只占一个 20MHz 信道")
		}
	}
	// 20MHz 时 165 是合法候选
	rep20 := AnalyzeBand(sv, Band5, 20)
	var saw165 bool
	for _, l := range rep20.Loads {
		if l.Channel == 165 {
			saw165 = true
		}
	}
	if !saw165 {
		t.Error("本机 20MHz 时 ch165 应可作为候选")
	}
}
