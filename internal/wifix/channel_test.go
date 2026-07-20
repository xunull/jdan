package wifix

import (
	"math"
	"slices"
	"testing"
)

// ---- 频段分桶 ----

// 6GHz 的信道号也从 1 开始，与 2.4GHz 直接碰撞。按信道号推断频段会把
// 6E 的 ch1 当成 2.4GHz 的 ch1，扔进 2.4GHz 重叠矩阵产出完全虚构的干扰分。
// 本机是 5GHz-only 测不出来，但代码会跑在 6E 机器上。
func TestBand_Channel1CollidesAcrossBands(t *testing.T) {
	f24, err := CenterFreqMHz(Band24, 1)
	if err != nil {
		t.Fatal(err)
	}
	f6, err := CenterFreqMHz(Band6, 1)
	if err != nil {
		t.Fatal(err)
	}
	if f24 == f6 {
		t.Fatal("2.4GHz ch1 与 6GHz ch1 的中心频率不该相同")
	}
	if f24 != 2412 {
		t.Errorf("2.4GHz ch1 = %d，应为 2412", f24)
	}
	if f6 != 5955 {
		t.Errorf("6GHz ch1 = %d，应为 5955", f6)
	}
	// 跨频段一律不重叠，哪怕信道号相同
	a := AP{Band: Band24, Channel: 1, WidthMHz: 20, RSSI: -40}
	b := AP{Band: Band6, Channel: 1, WidthMHz: 20, RSSI: -40}
	if ov := Overlap(a, b); ov != 0 {
		t.Errorf("跨频段同信道号重叠度应为 0，得到 %v", ov)
	}
}

// 信道 14 是 2484MHz，不是公式给出的 2477。不特判会错 7MHz。
func TestCenterFreq_Channel14IsException(t *testing.T) {
	got, err := CenterFreqMHz(Band24, 14)
	if err != nil {
		t.Fatal(err)
	}
	if got != 2484 {
		t.Errorf("2.4GHz ch14 = %d，应为 2484（公式 2407+5n 给出 2477，是错的）", got)
	}
	// 1-13 走公式
	for ch, want := range map[int]int{1: 2412, 6: 2437, 11: 2462, 13: 2472} {
		if g, _ := CenterFreqMHz(Band24, ch); g != want {
			t.Errorf("ch%d = %d，应为 %d", ch, g, want)
		}
	}
}

// ---- 2.4GHz 重叠 ----

func TestOverlap24(t *testing.T) {
	cases := []struct {
		a, b int
		want float64
	}{
		{1, 1, 1.0},
		{1, 2, 0.8},
		{1, 3, 0.6},
		{1, 4, 0.4},
		{1, 5, 0.2}, // Δ=4 是弱重叠，不是不重叠
		{1, 6, 0.0}, // Δ=5 才真正不重叠 —— 这是 1/6/11 的由来
		{1, 11, 0.0},
		{6, 1, 0.0}, // 对称
	}
	for _, c := range cases {
		if got := Overlap24(c.a, c.b); math.Abs(got-c.want) > 1e-9 {
			t.Errorf("Overlap24(%d,%d) = %v，应为 %v", c.a, c.b, got, c.want)
		}
	}
}

// 1/5/9/13 是折中方案不是干净方案：相邻间隔 Δ=4，按模型是弱重叠 0.2。
// 设计文档 v1 曾把它当「干净方案」推荐，与自身规则矛盾。
func TestOverlap24_1_5_9_13_IsWeaklyOverlapping(t *testing.T) {
	scheme := []int{1, 5, 9, 13}
	for i := 0; i+1 < len(scheme); i++ {
		ov := Overlap24(scheme[i], scheme[i+1])
		if ov == 0 {
			t.Errorf("ch%d 与 ch%d 不该判为完全不重叠（Δ=4 是弱重叠）", scheme[i], scheme[i+1])
		}
		if math.Abs(ov-0.2) > 1e-9 {
			t.Errorf("ch%d 与 ch%d 重叠度 = %v，应为 0.2", scheme[i], scheme[i+1], ov)
		}
	}
	// 而 1/6/11 之间确实互不重叠
	for _, p := range [][2]int{{1, 6}, {6, 11}, {1, 11}} {
		if ov := Overlap24(p[0], p[1]); ov != 0 {
			t.Errorf("ch%d 与 ch%d 应完全不重叠，得到 %v", p[0], p[1], ov)
		}
	}
}

// ---- 5GHz 对齐块展开（这批是核心：v1 的验收只测块起始，会放行错误实现）----

func TestExpand5GHz_AlignmentBlocks(t *testing.T) {
	cases := []struct {
		name  string
		ch    int
		width int
		want  []int
	}{
		// 块起始 —— v1 只测了这两个
		{"36@80 块起始", 36, 80, []int{36, 40, 44, 48}},
		{"36@40 块起始", 36, 40, []int{36, 40}},

		// 非块起始 —— 「往后数 N 个」的错误实现会在这里全部挂掉
		{"44@80 非块起始", 44, 80, []int{36, 40, 44, 48}},
		{"48@80 块末尾", 48, 80, []int{36, 40, 44, 48}},
		{"40@80", 40, 80, []int{36, 40, 44, 48}},
		{"161@80 高段", 161, 80, []int{149, 153, 157, 161}},
		{"157@80", 157, 80, []int{149, 153, 157, 161}},
		{"64@80", 64, 80, []int{52, 56, 60, 64}},

		// 160MHz —— 全世界只有两个合法块
		{"44@160", 44, 160, []int{36, 40, 44, 48, 52, 56, 60, 64}},
		{"36@160", 36, 160, []int{36, 40, 44, 48, 52, 56, 60, 64}},
		{"128@160", 128, 160, []int{100, 104, 108, 112, 116, 120, 124, 128}},

		// 40MHz 分组
		{"44@40", 44, 40, []int{44, 48}},
		{"48@40", 48, 40, []int{44, 48}},

		// 20MHz
		{"36@20", 36, 20, []int{36}},

		// 165 是孤立的 20MHz-only 信道，不参与任何组合
		{"165@20", 165, 20, []int{165}},
		{"165@80 非法组合降级", 165, 80, []int{165}},
	}
	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			got := Expand(Band5, c.ch, c.width)
			slices.Sort(got)
			want := slices.Clone(c.want)
			slices.Sort(want)
			if !slices.Equal(got, want) {
				t.Errorf("Expand(5GHz, %d, %d) = %v，应为 %v", c.ch, c.width, got, want)
			}
		})
	}
}

// 「往后数 N 个」这个错误实现的具体表现：44@80 会得到 {44,48,52,56}。
// 这个集合形状正常、看起来很合理，但完全错误 —— 它把 52/56（另一个块，
// 而且是 DFS 段）算成了本机占用。单独立一条用例把它钉死。
func TestExpand5GHz_44At80IsNotNaiveArithmetic(t *testing.T) {
	got := Expand(Band5, 44, 80)
	naive := []int{44, 48, 52, 56} // 错误实现会给出这个
	slices.Sort(got)
	if slices.Equal(got, naive) {
		t.Fatalf("44@80 展开成了 %v —— 这是「primary 往后数 4 个」的错误结果，"+
			"正确答案是 {36,40,44,48}（44 所在的对齐块）", got)
	}
	if !slices.Contains(got, 36) {
		t.Errorf("44@80 的对齐块必须含 36，得到 %v", got)
	}
	if slices.Contains(got, 52) {
		t.Errorf("44@80 不该占用 52（那是另一个 80MHz 块，且属 DFS 段），得到 %v", got)
	}
}

// ---- 跨宽度重叠 ----

func TestOverlap_CrossWidth(t *testing.T) {
	self := AP{Band: Band5, Channel: 36, WidthMHz: 80} // 占 {36,40,44,48}
	cases := []struct {
		name string
		n    AP
		want float64
	}{
		{"邻居 44@20 落在本机块内", AP{Band: Band5, Channel: 44, WidthMHz: 20}, 0.25},
		{"邻居 36@20", AP{Band: Band5, Channel: 36, WidthMHz: 20}, 0.25},
		{"邻居 44@160 完全覆盖本机", AP{Band: Band5, Channel: 44, WidthMHz: 160}, 1.0},
		{"邻居 36@80 同块", AP{Band: Band5, Channel: 36, WidthMHz: 80}, 1.0},
		{"邻居 44@40 覆盖两个子信道", AP{Band: Band5, Channel: 44, WidthMHz: 40}, 0.5},
		{"邻居 149@80 完全不同块", AP{Band: Band5, Channel: 149, WidthMHz: 80}, 0.0},
		{"邻居 52@20 相邻块", AP{Band: Band5, Channel: 52, WidthMHz: 20}, 0.0},
	}
	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			if got := Overlap(self, c.n); math.Abs(got-c.want) > 1e-9 {
				t.Errorf("重叠度 = %v，应为 %v", got, c.want)
			}
		})
	}
}

// 重叠度的分母是**本机**集合大小，不是邻居的。问的是「本机频谱有多少被占」。
func TestOverlap_DenominatorIsSelf(t *testing.T) {
	narrow := AP{Band: Band5, Channel: 44, WidthMHz: 20} // {44}
	wide := AP{Band: Band5, Channel: 36, WidthMHz: 80}   // {36,40,44,48}

	// 窄的看宽的：本机 1 个子信道全被覆盖 → 1.0
	if got := Overlap(narrow, wide); math.Abs(got-1.0) > 1e-9 {
		t.Errorf("窄看宽应为 1.0，得到 %v", got)
	}
	// 宽的看窄的：本机 4 个子信道只有 1 个被占 → 0.25
	if got := Overlap(wide, narrow); math.Abs(got-0.25) > 1e-9 {
		t.Errorf("宽看窄应为 0.25，得到 %v", got)
	}
}

// ---- DFS ----

func TestIsDFS(t *testing.T) {
	dfs := []int{52, 56, 60, 64, 100, 116, 132, 144}
	for _, ch := range dfs {
		if !IsDFS(Band5, ch) {
			t.Errorf("ch%d 应判为 DFS", ch)
		}
	}
	nonDFS := []int{36, 40, 44, 48, 149, 153, 157, 161, 165}
	for _, ch := range nonDFS {
		if IsDFS(Band5, ch) {
			t.Errorf("ch%d 不该判为 DFS", ch)
		}
	}
	// 2.4GHz 没有 DFS
	if IsDFS(Band24, 6) {
		t.Error("2.4GHz 不该有 DFS")
	}
}

// 气象雷达段的 CAC 静默期是 600 秒不是 60 秒 —— 这是切 DFS 最疼的代价，
// 设计文档 v1 只写了「短暂断连」，严重低估。
func TestDFSWarning_WeatherRadarIs600s(t *testing.T) {
	for _, ch := range []int{120, 124, 128} {
		if w := DFSWarning(ch); !contains(w, "600") {
			t.Errorf("ch%d 是气象雷达段，警告应提到 600 秒：%q", ch, w)
		}
	}
	for _, ch := range []int{52, 100, 144} {
		if w := DFSWarning(ch); !contains(w, "60") {
			t.Errorf("ch%d 的警告应提到 60 秒：%q", ch, w)
		}
	}
}

func contains(s, sub string) bool {
	return len(s) >= len(sub) && (func() bool {
		for i := 0; i+len(sub) <= len(s); i++ {
			if s[i:i+len(sub)] == sub {
				return true
			}
		}
		return false
	})()
}

func TestSNRGrade(t *testing.T) {
	cases := map[int]string{48: "优", 40: "优", 39: "良", 25: "良", 24: "中", 15: "中", 14: "差", 0: "差"}
	for snr, want := range cases {
		if got := SNRGrade(snr); got != want {
			t.Errorf("SNRGrade(%d) = %s，应为 %s", snr, got, want)
		}
	}
}

// 承载不了当前带宽的信道不能当候选。
//
// 实测踩到过：本机 80MHz 时 ch165 被推荐为「最空」—— 但 165 是孤立的
// 20MHz-only 信道，换过去带宽掉 4 倍，而它显得空恰恰因为只占一个
// 20MHz 信道。典型的「看起来合理的错建议」。
func TestCanHostWidth(t *testing.T) {
	cases := []struct {
		ch, width int
		want      bool
	}{
		{36, 20, true}, {36, 40, true}, {36, 80, true}, {36, 160, true},
		{44, 80, true}, {44, 160, true},
		{165, 20, true},  // 165 只能 20MHz
		{165, 40, false}, // 不能更宽
		{165, 80, false},
		{165, 160, false},
		{149, 80, true},   // 149 在 {149,153,157,161} 块里
		{149, 160, false}, // 但 5GHz 高段没有合法 160MHz 块
		{161, 80, true},
		{161, 160, false},
	}
	for _, c := range cases {
		if got := CanHostWidth(Band5, c.ch, c.width); got != c.want {
			t.Errorf("CanHostWidth(5GHz, %d, %d) = %v，应为 %v", c.ch, c.width, got, c.want)
		}
	}
	// 2.4GHz 一律放行（走连续重叠模型）
	if !CanHostWidth(Band24, 6, 40) {
		t.Error("2.4GHz 应一律放行")
	}
}
