package wifix

import "math"

// CCAThresholdDBm 是 20MHz OFDM 的载波侦听门限。
//
// 高于这个电平的同信道 AP 会让本机的 CSMA/CA **退避**。这是个门限而不是
// 连续量：强 10dB 和强 30dB 都是让你 defer，不是让你多 defer。
//
// 带宽翻倍门限约升 3dB（40MHz ≈ -79、80MHz ≈ -76），第一版按 20MHz 的
// -82 固定处理，够用；精确化留待有实测数据后再说。
const CCAThresholdDBm = -82

// ChannelLoad 是一个候选信道的干扰画像。
//
// 刻意是**两个量而不是一个合成分**。co-channel 和 adjacent-channel 是两种
// 不同的物理机制，量纲也不同：
//
//   - 同信道：CSMA/CA 退避，代价是被占走的空口时间，与相对强度基本无关
//   - 邻信道：不可解码的能量当作噪声，功率线性相加
//
// 压成一个分会让强信号邻居主导排序，从而低估「很多个中等强度同信道 BSS」
// 这种真实很糟的情况。商业产品那种「加权功率求和」就有这个问题。
type ChannelLoad struct {
	Channel int
	Band    Band

	// CoChannelBSS 是重叠度 = 1.0 且 RSSI 高于 CCA 门限的 AP 数量。
	// 这是主要代价，排序以它为准。
	CoChannelBSS int

	// CoChannelBelowCCA 是同信道但低于门限的 AP 数（不触发退避，仅供参考）。
	CoChannelBelowCCA int

	// AdjNoiseDBm 是邻信道能量按线性功率求和后的噪声抬升。
	// 无邻信道干扰时为 math.Inf(-1)。
	AdjNoiseDBm float64

	// SeenInSamples 是该信道在多少次采样中被观察到有 AP。
	// 零样本信道需要它来区分「真的空」和「这次没扫到」。
	SeenInSamples int
	TotalSamples  int
}

// TrulyEmpty 表示该信道在**全部**采样中都没扫到 AP。
//
// 只有满足这个条件才能说「空」。单次采样为 0 可能只是扫描盲区 ——
// 实测同一时刻连跑 6 次，邻居数在 13-17 之间波动 31%，
// absence of evidence 不等于 evidence of absence。
func (l ChannelLoad) TrulyEmpty() bool {
	return l.TotalSamples > 0 && l.SeenInSamples == 0
}

// dbmToMW 把 dBm 转成线性毫瓦。
func dbmToMW(dbm float64) float64 { return math.Pow(10, dbm/10) }

// mwToDBm 把线性毫瓦转回 dBm。输入 <= 0 返回负无穷。
func mwToDBm(mw float64) float64 {
	if mw <= 0 {
		return math.Inf(-1)
	}
	return 10 * math.Log10(mw)
}

// Analyze 计算某个候选信道在给定邻居集合下的干扰画像。
//
// selfWidth 是本机（或假设切换过去后）使用的带宽 —— 带宽决定了候选信道
// 实际占用哪些 20MHz 子信道，进而决定谁算同信道、谁算邻信道。
func Analyze(band Band, ch, selfWidthMHz int, neighbors []AP) ChannelLoad {
	self := AP{Band: band, Channel: ch, WidthMHz: selfWidthMHz}
	load := ChannelLoad{Channel: ch, Band: band, AdjNoiseDBm: math.Inf(-1)}

	var adjMW float64
	for _, n := range neighbors {
		ov := Overlap(self, n)
		if ov <= 0 {
			continue
		}
		if ov >= 1.0 {
			// 完全重叠 = 同信道。走空口时间模型：只看是否越过门限。
			if n.RSSI >= CCAThresholdDBm {
				load.CoChannelBSS++
			} else {
				load.CoChannelBelowCCA++
			}
			continue
		}
		// 部分重叠 = 邻信道。走线性功率求和。
		adjMW += ov * dbmToMW(float64(n.RSSI))
	}
	load.AdjNoiseDBm = mwToDBm(adjMW)
	return load
}

// Rank 按「更适合切换过去」排序候选信道，最优在前。
//
// 排序规则（与 ChannelLoad 的双量设计一致）：
//  1. CoChannelBSS 升序 —— 这是主要代价
//  2. 并列时 AdjNoiseDBm 升序 —— 邻道噪声越低越好
//  3. 再并列按信道号升序，保证结果确定（否则同分信道的顺序取决于输入顺序，
//     而输入来自 map 遍历，同一份数据跑两次输出会变）
func Rank(loads []ChannelLoad) []ChannelLoad {
	out := make([]ChannelLoad, len(loads))
	copy(out, loads)
	sortSlice(out, func(a, b ChannelLoad) bool {
		if a.CoChannelBSS != b.CoChannelBSS {
			return a.CoChannelBSS < b.CoChannelBSS
		}
		// -Inf 参与比较是安全的：无邻道干扰的信道排在前面
		if a.AdjNoiseDBm != b.AdjNoiseDBm {
			return a.AdjNoiseDBm < b.AdjNoiseDBm
		}
		return a.Channel < b.Channel
	})
	return out
}

// sortSlice 是个薄封装，避免在多处重复 sort.Slice 的闭包样板。
func sortSlice[T any](s []T, less func(a, b T) bool) {
	// 插入排序：候选信道最多几十个，不值得引入更复杂的实现，
	// 而且插入排序对已经有序的输入是 O(n)。
	for i := 1; i < len(s); i++ {
		for j := i; j > 0 && less(s[j], s[j-1]); j-- {
			s[j], s[j-1] = s[j-1], s[j]
		}
	}
}

// ---- DFS ----

// IsDFS 判断 5GHz 信道是否属于 DFS（动态频率选择）范围。
//
// U-NII-2A = 52/56/60/64，U-NII-2C = 100-144。65-99 之间没有任何合法的
// 5GHz 信道号，所以 [52,144] 这个区间判定不多不少刚好命中 DFS 集合。
func IsDFS(b Band, ch int) bool {
	return b == Band5 && ch >= 52 && ch <= 144
}

// DFSWarning 返回切换到 DFS 信道的代价说明。
//
// 最疼的不是「偶尔断连」，是**初次 CAC 静默期**：AP 切过去后必须先监听
// 60 秒才能发射；ETSI 气象雷达信道 120/124/128 要 600 秒。
func DFSWarning(ch int) string {
	if ch >= 120 && ch <= 128 {
		return "DFS 信道（气象雷达段）：切换后需静默监听约 600 秒才能发射，且遇雷达会强制跳频"
	}
	return "DFS 信道：切换后需静默监听约 60 秒才能发射，且遇雷达会强制跳频"
}

// SNRGrade 把信噪比分档。阈值来自常见的 802.11 链路预算经验值：
// 1024-QAM（MCS 11）约需 35dB，256-QAM 约需 25dB，低阶调制 15dB 可用。
func SNRGrade(snrDB int) string {
	switch {
	case snrDB >= 40:
		return "优"
	case snrDB >= 25:
		return "良"
	case snrDB >= 15:
		return "中"
	default:
		return "差"
	}
}
