package wifix

import (
	"fmt"
	"slices"
	"sort"
)

// bucketKey 是「频段 + 信道 + 宽度」。
//
// SSID 被脱敏后没有稳定的 AP 标识可用，所以不做逐 AP 去重 —— 那样会把
// 同信道同宽度、RSSI 相近的两个不同 AP 塌成一个，**少算同信道 AP 数量**，
// 而这个数正是干扰模型的主要输入。
//
// 改为按桶统计：跨采样取「单次出现最多的那一次」的 AP 列表。既不会因去重
// 而少算，也不会因跨采样累加而多算。
type bucketKey struct {
	band    Band
	channel int
	width   int
}

func bucketOf(n Network) bucketKey {
	return bucketKey{band: n.Band, channel: n.Channel, width: n.WidthMHz}
}

// Survey 是多次采样合并后的结果。
type Survey struct {
	Interface string
	Current   *Network
	Country   string
	MAC       string

	// Neighbors 是全部采样的并集。每个 AP 带出现频次。
	Neighbors []SurveyAP

	SupportedChannels []ChannelSpec
	Samples           int // 实际完成的采样次数
}

// SurveyAP 是一个邻居 AP 及其在采样中的出现情况。
type SurveyAP struct {
	Network
	SeenCount int // 在多少次采样中被看到
}

// Intermittent 表示该 AP 没有在每次采样中都出现。
//
// 实测同一时刻连跑 6 次，邻居数在 13-17 之间波动 31% —— 间歇可见是常态
// 而非异常，展示时值得标出来，免得用户以为数据不准。
func (a SurveyAP) Intermittent(totalSamples int) bool {
	return totalSamples > 1 && a.SeenCount < totalSamples
}

// ScanFunc 抓一次原始 plist。注入便于测试。
type ScanFunc func() ([]byte, error)

// Collect 采样 n 次并合并。
//
// n > 1 是**正确性需求不是可选项**：单次扫描可能漏掉 17 个 AP 里的 4 个，
// 而「某信道 0 个 AP」是推荐算法的关键输入 —— 基于单次快照说「149 最空」
// 可能只是这次没扫到。
//
// 任何一次采样失败都直接返回错误：部分成功的采样会让 SeenCount/Samples
// 的比值失去意义（分母不对），比干脆失败更糟。
func Collect(scan ScanFunc, n int, ifaceName string) (*Survey, error) {
	if n < 1 {
		n = 1
	}

	// 每个桶记录：出现在几次采样里、以及单次采样中见过的最多的那组 AP。
	type bucketState struct {
		seenSamples int
		best        []Network // 单次采样中出现最多的那一组
	}
	buckets := map[bucketKey]*bucketState{}
	var sv Survey

	for i := range n {
		data, err := scan()
		if err != nil {
			return nil, fmt.Errorf("第 %d 次扫描失败：%w", i+1, err)
		}
		ifaces, err := Parse(data)
		if err != nil {
			return nil, fmt.Errorf("第 %d 次扫描解析失败：%w", i+1, err)
		}

		iface := pick(ifaces, ifaceName)
		if iface == nil {
			if ifaceName != "" {
				return nil, fmt.Errorf("找不到已连接的接口 %q", ifaceName)
			}
			return nil, errNotConnected
		}

		// 当前连接信息取最后一次采样的（最新）。
		sv.Interface = iface.Name
		sv.Current = iface.Current
		sv.Country = iface.CountryCode
		sv.MAC = iface.MACAddress
		if len(iface.SupportedChannels) > 0 {
			sv.SupportedChannels = iface.SupportedChannels
		}

		// 先按桶归拢本次采样的 AP，再与历史比较 —— 必须先归拢，否则
		// 同一次采样里的多个 AP 会被当成多次「出现」。
		thisSample := map[bucketKey][]Network{}
		for _, nb := range iface.Neighbors {
			if nb.Band == BandUnknown {
				continue // 信道字符串没解析出来，跳过而不是当成 2.4GHz
			}
			k := bucketOf(nb)
			thisSample[k] = append(thisSample[k], nb)
		}
		for k, list := range thisSample {
			st, ok := buckets[k]
			if !ok {
				st = &bucketState{}
				buckets[k] = st
			}
			st.seenSamples++ // 每次采样最多 +1
			// 取「见到最多 AP」的那一次。扫描会漏，取最大值比取并集更接近真实
			// （并集可能把同一个 AP 的两次抖动读数算成两个）。
			if len(list) > len(st.best) {
				st.best = list
			}
		}
		sv.Samples++
	}

	for _, st := range buckets {
		for _, nb := range st.best {
			sv.Neighbors = append(sv.Neighbors, SurveyAP{Network: nb, SeenCount: st.seenSamples})
		}
	}
	// 确定性排序：(频段, 信道, RSSI 降序, 宽度)。不排的话顺序来自 map 遍历，
	// 同一份数据跑两次 --json 就不逐字节相同了。
	sort.Slice(sv.Neighbors, func(i, j int) bool {
		a, b := sv.Neighbors[i], sv.Neighbors[j]
		if a.Band != b.Band {
			return a.Band < b.Band
		}
		if a.Channel != b.Channel {
			return a.Channel < b.Channel
		}
		if a.RSSI != b.RSSI {
			return a.RSSI > b.RSSI
		}
		return a.WidthMHz < b.WidthMHz
	})
	return &sv, nil
}

// errNotConnected 是「有无线硬件但没连网」的哨兵错误。
// 调用方可以据此给出「未连接」而非「命令失败」。
var errNotConnected = fmt.Errorf("当前未连接任何 WiFi 网络")

// IsNotConnected 判断错误是否为「未连接」。
func IsNotConnected(err error) bool { return err == errNotConnected }

func pick(ifaces []Interface, name string) *Interface {
	if name != "" {
		for i := range ifaces {
			if ifaces[i].Name == name && ifaces[i].Connected {
				return &ifaces[i]
			}
		}
		return nil
	}
	return Connected(ifaces)
}

// ---- 信道分析编排 ----

// ChannelReport 是某个频段下全部候选信道的分析结果。
type ChannelReport struct {
	Band     Band
	Loads    []ChannelLoad // 按 Rank 排序，最优在前
	SelfLoad *ChannelLoad  // 本机当前信道的画像
	Best     *ChannelLoad  // 推荐信道；无更优选择时为 nil
}

// AnalyzeBand 对某频段的所有候选信道做干扰分析并给出推荐。
//
// 候选信道来自 supported_channels 与本频段的交集 —— 这条同时承担**区域
// 合规兜底**：EU/JP 不分配 5725-5875，那边的机器 supported 里没有 149+，
// 自然不会被推荐。
//
// selfWidth 是假设切换过去后仍用的带宽。
func AnalyzeBand(sv *Survey, band Band, selfWidth int) *ChannelReport {
	// 保留 SeenCount 与 AP 的对应关系：可见性统计必须跟干扰计算用同一套
	// 重叠逻辑，否则会出现「同信道 3 个 BSS」和「三次采样都没见到 AP」
	// 同时成立的自相矛盾 —— 实测踩到过，成因是按邻居的 primary 信道查表，
	// 而 160MHz 邻居的 primary 在 44、实际覆盖到 52。
	type seenAP struct {
		ap   AP
		seen int
	}
	var withSeen []seenAP
	var neighbors []AP
	for _, nb := range sv.Neighbors {
		if nb.Band != band {
			continue
		}
		a := nb.AP()
		neighbors = append(neighbors, a)
		withSeen = append(withSeen, seenAP{ap: a, seen: nb.SeenCount})
	}

	rep := &ChannelReport{Band: band}
	for _, cs := range sv.SupportedChannels {
		if cs.Band != band {
			continue
		}
		ch := cs.Channel
		if !CanHostWidth(band, ch, selfWidth) {
			// 承载不了当前带宽的信道不作为候选。
			//
			// 实测踩到过：本机 80MHz 时 ch165 会被推荐，因为它「同信道 0 个」——
			// 但 165 是 20MHz-only 的孤立信道，换过去带宽掉 4 倍。它显得空
			// 恰恰因为它只占一个 20MHz 信道。这是典型的「看起来合理的错建议」。
			continue
		}
		l := Analyze(band, ch, selfWidth, neighbors)
		l.TotalSamples = sv.Samples
		// 与干扰计算同一套判据：任何与该候选信道有重叠的 AP 都算「见到过」。
		self := AP{Band: band, Channel: ch, WidthMHz: selfWidth}
		for _, s := range withSeen {
			if Overlap(self, s.ap) > 0 {
				l.SeenInSamples = max(l.SeenInSamples, s.seen)
			}
		}
		rep.Loads = append(rep.Loads, l)
	}
	rep.Loads = Rank(rep.Loads)

	if sv.Current != nil && sv.Current.Band == band {
		for i := range rep.Loads {
			if rep.Loads[i].Channel == sv.Current.Channel {
				rep.SelfLoad = &rep.Loads[i]
				break
			}
		}
	}

	// 只有在明显更优时才推荐：同信道 BSS 数必须严格更少。
	// 「差不多」的时候不给建议 —— 换信道有成本（DFS 更甚），
	// 给个边际收益的建议比不给更糟。
	if rep.SelfLoad != nil && len(rep.Loads) > 0 {
		if best := rep.Loads[0]; best.Channel != rep.SelfLoad.Channel &&
			best.CoChannelBSS < rep.SelfLoad.CoChannelBSS {
			rep.Best = &rep.Loads[0]
		}
	}
	return rep
}

// SupportedIn 返回本机在该频段支持的信道号，升序。
//
// 频段是解析时从 macOS 的原始字符串带下来的，不是按信道号范围反推的 ——
// 后者在 6GHz 上会出错（6GHz 信道号也从 1 开始）。
func (s *Survey) SupportedIn(band Band) []int {
	var out []int
	for _, cs := range s.SupportedChannels {
		if cs.Band == band {
			out = append(out, cs.Channel)
		}
	}
	slices.Sort(out)
	return out
}

// Bands 返回本机支持的全部频段，按 2.4/5/6 顺序。
func (s *Survey) Bands() []Band {
	seen := map[Band]bool{}
	for _, cs := range s.SupportedChannels {
		seen[cs.Band] = true
	}
	var out []Band
	for _, b := range []Band{Band24, Band5, Band6} {
		if seen[b] {
			out = append(out, b)
		}
	}
	return out
}
