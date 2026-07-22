package tradx

import "testing"

func conv(t *testing.T, target, in string) string {
	t.Helper()
	out, err := Convert(in, target)
	if err != nil {
		t.Fatalf("Convert(%q,%q): %v", in, target, err)
	}
	return out
}

// s2t 核心：短语消歧（一简对多繁必须靠词，不能逐字）。期望值全部对照真词典核过。
func TestS2T_PhraseDisambiguation(t *testing.T) {
	cases := map[string]string{
		"头发": "頭髮", // 发→髮（STPhrases 头发→頭髮）
		"发展": "發展", // 发→發（STPhrases 发展→發展）
		"干燥": "乾燥", // 干→乾
		"干部": "幹部", // 干→幹
		"台风": "颱風", // STPhrases 台风→颱風：短语赢过单字 台→臺，正是最长匹配的意义
	}
	for in, want := range cases {
		if got := conv(t, ToT, in); got != want {
			t.Errorf("s2t %q = %q，应为 %q", in, got, want)
		}
	}
}

// 孤立单字取默认（value[0]）：无词上下文时 OpenCC 就是取第一个，非 bug。
func TestS2T_SingleCharDefault(t *testing.T) {
	cases := map[string]string{
		"发": "發", // 发→發 髮，取首
		"干": "幹", // 干→幹 乾 干 榦，取首
		"软": "軟",
	}
	for in, want := range cases {
		if got := conv(t, ToT, in); got != want {
			t.Errorf("s2t %q = %q，应为 %q", in, got, want)
		}
	}
}

// 台风：不分词会得 臺風（台→臺 取首），分词后 颱風。若最长匹配写错（退化字符级）此例必挂。
func TestS2T_LongestMatchBeatsChar(t *testing.T) {
	if got := conv(t, ToT, "台风"); got != "颱風" {
		t.Errorf("s2t 台风 = %q，应为 颱風（短语最长匹配赢单字）", got)
	}
}

// 地区词汇层（s2twp = s2t 然后 TWPhrases 组）。软件→軟體 是用户点名要的。
func TestS2TWP_Vocabulary(t *testing.T) {
	cases := map[string]string{
		"软件":  "軟體",  // s2t→軟件，TWPhrases 軟件→軟體
		"网络":  "網路",  // s2t→網絡，TWPhrases 網絡→網路
		"打印机": "印表機", // s2t→打印機，TWPhrases 打印機→印表機
	}
	for in, want := range cases {
		if got := conv(t, ToTWP, in); got != want {
			t.Errorf("s2twp %q = %q，应为 %q", in, got, want)
		}
	}
}

// s2t 与 s2twp 在"软件"上必须不同：前者字形 軟件，后者词汇 軟體。
func TestS2T_vs_S2TWP_SoftwareDiffers(t *testing.T) {
	st := conv(t, ToT, "软件")
	twp := conv(t, ToTWP, "软件")
	if st != "軟件" {
		t.Errorf("s2t 软件 = %q，应为 軟件（字形）", st)
	}
	if twp != "軟體" {
		t.Errorf("s2twp 软件 = %q，应为 軟體（词汇）", twp)
	}
	if st == twp {
		t.Error("s2t 与 s2twp 在 软件 上不该相同")
	}
}

// t2s 逆向。期望对照 TSCharacters 逆向单字核过。
func TestT2S(t *testing.T) {
	cases := map[string]string{
		"頭髮": "头发",
		"發展": "发展",
		"軟體": "软体",
		"網絡": "网络",
	}
	for in, want := range cases {
		if got := conv(t, ToS, in); got != want {
			t.Errorf("t2s %q = %q，应为 %q", in, got, want)
		}
	}
}

// 往返：常用词 s2t 再 t2s 应回到原文（标准方向可逆的字）。
func TestRoundTrip_S2T_T2S(t *testing.T) {
	for _, in := range []string{"头发", "发展", "网络", "中文世界"} {
		tr := conv(t, ToT, in)
		back := conv(t, ToS, tr)
		if back != in {
			t.Errorf("往返 %q →(s2t) %q →(t2s) %q，应回到 %q", in, tr, back, in)
		}
	}
}

// 非汉字透传：英文/数字/标点/emoji/空格原样保留，只转汉字。
func TestPassthrough_NonHan(t *testing.T) {
	got := conv(t, ToT, "Hi 发 2024! 😀")
	if got != "Hi 發 2024! 😀" {
		t.Errorf("透传失败：%q，应为 %q", got, "Hi 發 2024! 😀")
	}
}

func TestEmptyAndUnknown(t *testing.T) {
	if got := conv(t, ToT, ""); got != "" {
		t.Errorf("空串应得空，得 %q", got)
	}
	if _, err := NewConverter("zzz"); err == nil {
		t.Error("未知 target 应报错")
	}
}

// 词典完整性：9 部全部加载、非空、maxKeyLen 合理。
func TestDictIntegrity(t *testing.T) {
	dd, err := getDicts()
	if err != nil {
		t.Fatal(err)
	}
	if len(dd) != len(dictFiles) {
		t.Fatalf("应加载 %d 部词典，实际 %d", len(dictFiles), len(dd))
	}
	for _, name := range dictFiles {
		d := dd[name]
		if d == nil || len(d.m) == 0 {
			t.Errorf("词典 %s 为空", name)
			continue
		}
		if d.max < 1 || d.max > 32 {
			t.Errorf("词典 %s maxKeyLen=%d 不合理", name, d.max)
		}
		// 确认没吞掉注释头以外的真数据：STPhrases 应是万级。
		if name == "STPhrases" && len(d.m) < 40000 {
			t.Errorf("STPhrases 只 %d 条，疑似解析吞行", len(d.m))
		}
	}
}

// 直接验证两种 group policy 语义不同——short_circuit ≠ union。
// 这是设计的核心不变量：s2twp 地区阶段是 short_circuit，且 TWPhrases 有 13 个单字 key，
// 若误用 union 会取更长的匹配，与 OpenCC 不一致。
func TestGroupPolicy_ShortCircuitVsUnion(t *testing.T) {
	d1 := &dict{m: map[string]string{"甲": "1"}, max: 1}  // 单字 key
	d2 := &dict{m: map[string]string{"甲乙": "2"}, max: 2} // 双字 key，与 d1 在位置 0 重叠
	rs := []rune("甲乙")

	sc := &group{shortCircuit: true, members: []matcher{d1, d2}}
	if n, v, ok := sc.matchPrefix(rs, 0); !ok || n != 1 || v != "1" {
		t.Errorf("short_circuit 应取第一个成员的匹配 (1,\"1\")，得 (%d,%q,%v)", n, v, ok)
	}
	un := &group{shortCircuit: false, members: []matcher{d1, d2}}
	if n, v, ok := un.matchPrefix(rs, 0); !ok || n != 2 || v != "2" {
		t.Errorf("union 应取最长匹配 (2,\"2\")，得 (%d,%q,%v)", n, v, ok)
	}
}

func TestDiff(t *testing.T) {
	// 等长逐字替换。
	ch := Diff("发展", "發展")
	if len(ch) != 1 || ch[0].Pos != 0 || ch[0].Orig != "发" || ch[0].Conv != "發" {
		t.Errorf("Diff(发展,發展) = %+v，应 [{0 发 發}]", ch)
	}
	// 变长（3 rune → 2 rune）。
	ch = Diff("方便面", "泡麵")
	if len(ch) != 1 || ch[0].Pos != 0 || ch[0].Orig != "方便面" || ch[0].Conv != "泡麵" {
		t.Errorf("Diff(方便面,泡麵) = %+v，应 [{0 方便面 泡麵}]", ch)
	}
	// 无差异。
	if ch := Diff("中文", "中文"); ch != nil {
		t.Errorf("相同串 Diff 应为 nil，得 %+v", ch)
	}
	// 部分改动，pos 用输入侧偏移。
	ch = Diff("我的头发", "我的頭髮")
	if len(ch) != 1 || ch[0].Pos != 2 || ch[0].Orig != "头发" || ch[0].Conv != "頭髮" {
		t.Errorf("Diff(我的头发,我的頭髮) = %+v，应 [{2 头发 頭髮}]", ch)
	}
}

// 量化 premise #3 的缺口（评审 5.4）：TWPhrases 键 K 可达 ⟺ s2t(t2s(K))==K。
// 统计并打印不可达条数，作为缺失生成词典的真实代价，写进回归。
func TestTWPhrasesReachability(t *testing.T) {
	dd, err := getDicts()
	if err != nil {
		t.Fatal(err)
	}
	tw := dd["TWPhrases"]
	s2t, _ := NewConverter(ToT)
	t2s, _ := NewConverter(ToS)

	var unreachable int
	var sample []string
	for k := range tw.m {
		if s2t.Convert(t2s.Convert(k)) != k {
			unreachable++
			if len(sample) < 8 {
				sample = append(sample, k)
			}
		}
	}
	total := len(tw.m)
	t.Logf("TWPhrases 可达性：%d/%d 不可达（%.1f%%），样例 %v",
		unreachable, total, 100*float64(unreachable)/float64(total), sample)
	// 底线：大多数地区词应可达。若大面积不可达，说明两阶段建模或数据有问题。
	if unreachable*2 > total {
		t.Errorf("过半 TWPhrases 键不可达（%d/%d），two-stage 建模可能有误", unreachable, total)
	}
}
