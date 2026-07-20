package wifix

import (
	"fmt"
	"math"
	"strconv"
	"strings"
	"time"

	"github.com/xunull/jdan/internal/termx"
)

// RenderOptions 控制文本渲染。
type RenderOptions struct {
	Color    bool
	MaxWidth int // 终端宽度；>0 时截断过长的行。0 = 不截断
	BarWidth int // 占用条宽度（0 → 默认 10）
	Elapsed  time.Duration
	ShowAll  bool // 列出全部候选信道而非只列前几个
}

const (
	defaultChanBarWidth = 10
	defaultTopChannels  = 8
)

// RedactionNotice 是 SSID 被脱敏时给用户的说明。
//
// 单独导出是因为它同时要出现在文本输出末尾和 --json 的说明字段里 ——
// 这条信息比 SSID 本身更有用：它告诉用户为什么看不到、以及怎么才能看到。
const RedactionNotice = `SSID 显示为脱敏，是因为 macOS 14 起把它归类为位置信息。
要看到 SSID：系统设置 → 隐私与安全性 → 定位服务 → 勾选你的终端 App。
其余射频数据（信道/信号/协议/加密）不受此限制。`

// Render 把一次勘测渲染成人类可读的报告。
func Render(sv *Survey, reports []*ChannelReport, opt RenderOptions) string {
	barW := opt.BarWidth
	if barW == 0 {
		barW = defaultChanBarWidth
	}

	var sb strings.Builder
	renderCurrent(&sb, sv)
	for _, rep := range reports {
		if rep == nil || len(rep.Loads) == 0 {
			continue
		}
		sb.WriteString("\n")
		renderBand(&sb, sv, rep, barW, opt)
	}
	renderFooter(&sb, sv, opt)
	return sb.String()
}

func renderCurrent(sb *strings.Builder, sv *Survey) {
	cur := sv.Current
	if cur == nil {
		sb.WriteString("未连接任何 WiFi 网络。\n")
		if len(sv.Neighbors) > 0 {
			sb.WriteString("以下是扫描到的周边网络占用情况，仍可用于选台。\n")
		}
		return
	}

	// 第一行：接口 + 协议 + 信道（含实际占用的子信道）
	occupied := Expand(cur.Band, cur.Channel, cur.WidthMHz)
	line := fmt.Sprintf("%s  %s  信道 %d (%s, %dMHz",
		sv.Interface, cur.PHYMode, cur.Channel, cur.Band, cur.WidthMHz)
	if len(occupied) > 1 {
		line += " → 占用 " + joinInts(occupied, "/")
	}
	line += ")"
	sb.WriteString(line + "\n")

	// 第二行：信号质量
	if cur.Noise != 0 {
		snr := cur.SNR()
		sb.WriteString(fmt.Sprintf("     信号 %ddBm / 噪声 %ddBm   SNR %ddB  %s\n",
			cur.RSSI, cur.Noise, snr, SNRGrade(snr)))
	} else if cur.RSSI != 0 {
		sb.WriteString(fmt.Sprintf("     信号 %ddBm\n", cur.RSSI))
	}

	// 第三行：加密 + 速率
	third := "     "
	if cur.Security != "" {
		third += cur.Security
	}
	if cur.Rate > 0 {
		third += fmt.Sprintf("   协商 %d Mbps", cur.Rate)
		if cur.MCS > 0 {
			third += fmt.Sprintf(" (MCS %d)", cur.MCS)
		}
	}
	if strings.TrimSpace(third) != "" {
		sb.WriteString(third + "\n")
	}

	// SSID：脱敏时明说，不留白让人以为是 bug
	if cur.Redacted {
		sb.WriteString("     SSID <已脱敏 — 见末尾说明>\n")
	} else if cur.SSID != "" {
		sb.WriteString("     SSID " + cur.SSID + "\n")
	}
}

func renderBand(sb *strings.Builder, sv *Survey, rep *ChannelReport, barW int, opt RenderOptions) {
	title := fmt.Sprintf("%s 信道占用（★ = 本机", rep.Band)
	if sv.Samples > 1 {
		title += fmt.Sprintf("，%d 次采样", sv.Samples)
	}
	title += "）"
	sb.WriteString(title + "\n")

	// 条形长度按同信道 BSS 数归一化 —— 不再有单一合成分，所以不能按分排。
	maxBSS := 0
	for _, l := range rep.Loads {
		maxBSS = max(maxBSS, l.CoChannelBSS)
	}

	loads := rep.Loads
	if !opt.ShowAll && len(loads) > defaultTopChannels {
		loads = loads[:defaultTopChannels]
		// 本机所在信道必须始终可见。它按干扰排序常常掉出前 N ——
		// 实测本机 ch36 有 4 个同信道 BSS，被排在 149/52 之后截掉了，
		// 于是建议里写着「当前信道 36 上有 4 个 BSS」而表格里根本没有 36 那行。
		if rep.SelfLoad != nil && !containsChannel(loads, rep.SelfLoad.Channel) {
			loads = append(loads[:len(loads)-1:len(loads)-1], *rep.SelfLoad)
		}
	}

	rows := make([][]string, 0, len(loads))
	for _, l := range loads {
		star := " "
		if rep.SelfLoad != nil && l.Channel == rep.SelfLoad.Channel {
			star = "★"
		}

		pct := 0
		if maxBSS > 0 {
			pct = l.CoChannelBSS * 100 / maxBSS
		}
		bar := termx.Colorize(termx.Bar(pct, barW), pct, opt.Color)

		co := strconv.Itoa(l.CoChannelBSS)
		if l.CoChannelBelowCCA > 0 {
			co += fmt.Sprintf("+%d弱", l.CoChannelBelowCCA)
		}

		noise := "—"
		if !math.IsInf(l.AdjNoiseDBm, -1) {
			noise = fmt.Sprintf("%.0f dBm", l.AdjNoiseDBm)
		}

		note := ""
		switch {
		case l.TrulyEmpty():
			note = fmt.Sprintf("%d 次采样均未见 AP", l.TotalSamples)
		case IsDFS(rep.Band, l.Channel):
			note = "DFS"
		}

		rows = append(rows, []string{
			fmt.Sprintf("  %d %s", l.Channel, star),
			bar,
			co,
			noise,
			note,
		})
	}

	sb.WriteString(termx.Table(
		[]string{"  信道", "", "同信道", "邻道噪声", ""},
		rows,
		map[int]bool{2: true, 3: true},
	))

	if adv := advice(sv, rep); adv != "" {
		sb.WriteString("\n" + adv + "\n")
	}
}

// advice 生成换信道建议。
//
// 只在明显更优时给（AnalyzeBand 已经把这条判断做了），并且必须带上代价：
// DFS 的 CAC 静默期、以及「零样本可能是扫描盲区」的提醒。
// 给个边际收益的建议比不给更糟。
func advice(sv *Survey, rep *ChannelReport) string {
	if rep.SelfLoad == nil {
		return ""
	}
	self := rep.SelfLoad

	if rep.Best == nil {
		if self.CoChannelBSS == 0 {
			return "当前信道没有同信道 BSS，不需要换。"
		}
		return fmt.Sprintf("当前信道有 %d 个同信道 BSS，但其余候选信道并不更好，换了收益不明显。",
			self.CoChannelBSS)
	}

	best := rep.Best
	var b strings.Builder
	fmt.Fprintf(&b, "建议：当前信道 %d 上有 %d 个 BSS 超过载波侦听门限（%d dBm），"+
		"每次它们发包你都要退避。",
		self.Channel, self.CoChannelBSS, CCAThresholdDBm)
	fmt.Fprintf(&b, "\n      信道 %d 只有 %d 个，更空。", best.Channel, best.CoChannelBSS)

	if IsDFS(rep.Band, best.Channel) {
		b.WriteString("\n      注意：" + DFSWarning(best.Channel) + "。")
	}
	if best.TrulyEmpty() {
		fmt.Fprintf(&b, "\n      注意：%d 次采样都没在该信道扫到 AP，也可能是扫描盲区而非真空。",
			best.TotalSamples)
	}
	return b.String()
}

func renderFooter(sb *strings.Builder, sv *Survey, opt RenderOptions) {
	var parts []string
	if opt.Elapsed > 0 {
		parts = append(parts, "用时 "+opt.Elapsed.Round(time.Millisecond).String())
	}
	if sv.Samples > 0 {
		parts = append(parts, fmt.Sprintf("%d 次采样", sv.Samples))
	}
	if n := len(sv.Neighbors); n > 0 {
		inter := 0
		for _, a := range sv.Neighbors {
			if a.Intermittent(sv.Samples) {
				inter++
			}
		}
		s := fmt.Sprintf("扫到 %d 个 AP", n)
		if inter > 0 {
			// 实测同一时刻连跑 6 次邻居数波动 31%，间歇可见是常态而非异常，
			// 说出来免得用户以为数据不准。
			s += fmt.Sprintf("（%d 个间歇可见）", inter)
		}
		parts = append(parts, s)
	}
	if sv.Country != "" {
		parts = append(parts, "区域 "+sv.Country)
	}
	if len(parts) > 0 {
		sb.WriteString("\n" + strings.Join(parts, " / ") + "\n")
	}

	if redactedAnywhere(sv) {
		sb.WriteString("\n" + RedactionNotice + "\n")
	}
}

func containsChannel(ls []ChannelLoad, ch int) bool {
	for _, l := range ls {
		if l.Channel == ch {
			return true
		}
	}
	return false
}

func redactedAnywhere(sv *Survey) bool {
	if sv.Current != nil && sv.Current.Redacted {
		return true
	}
	for _, n := range sv.Neighbors {
		if n.Redacted {
			return true
		}
	}
	return false
}

func joinInts(xs []int, sep string) string {
	var sb strings.Builder
	for i, x := range xs {
		if i > 0 {
			sb.WriteString(sep)
		}
		sb.WriteString(strconv.Itoa(x))
	}
	return sb.String()
}

// ---- JSON ----

// JSONReport 是 --json 的顶层结构。
type JSONReport struct {
	Interface string        `json:"interface"`
	Connected bool          `json:"connected"`
	Current   *JSONNetwork  `json:"current"`
	Country   string        `json:"country_code"`
	MAC       string        `json:"mac_address"`
	Samples   int           `json:"samples"`
	Neighbors []JSONNeighor `json:"neighbors"`
	Bands     []JSONBand    `json:"bands"`

	// SSIDRedacted 是显式布尔，不让下游去比 "<redacted>" 字符串。
	SSIDRedacted bool   `json:"ssid_redacted"`
	RedactionNot string `json:"redaction_notice,omitempty"`
}

type JSONNetwork struct {
	SSID     string `json:"ssid"`
	Redacted bool   `json:"redacted"`
	Band     string `json:"band"`
	Channel  int    `json:"channel"`
	WidthMHz int    `json:"width_mhz"`
	Occupied []int  `json:"occupied_subchannels"`
	PHYMode  string `json:"phy_mode"`
	Security string `json:"security"`
	RSSI     int    `json:"rssi_dbm"`
	Noise    int    `json:"noise_dbm"`
	SNR      int    `json:"snr_db"`
	SNRGrade string `json:"snr_grade"`
	RateMbps int    `json:"rate_mbps,omitempty"`
	MCSIndex int    `json:"mcs_index,omitempty"`
}

type JSONNeighor struct {
	JSONNetwork
	SeenInSamples int  `json:"seen_in_samples"`
	Intermittent  bool `json:"intermittent"`
}

type JSONBand struct {
	Band     string        `json:"band"`
	Channels []JSONChannel `json:"channels"`
	Best     *int          `json:"recommended_channel"`
}

type JSONChannel struct {
	Channel           int      `json:"channel"`
	CoChannelBSS      int      `json:"co_channel_bss"`
	CoChannelBelowCCA int      `json:"co_channel_below_cca"`
	AdjNoiseDBm       *float64 `json:"adjacent_noise_dbm"`
	IsSelf            bool     `json:"is_current"`
	IsDFS             bool     `json:"is_dfs"`
	TrulyEmpty        bool     `json:"truly_empty"`
	SeenInSamples     int      `json:"seen_in_samples"`
}

// JSONData 组装 --json 输出。不受展示层裁剪影响：全部信道、全部邻居。
func JSONData(sv *Survey, reports []*ChannelReport) *JSONReport {
	out := &JSONReport{
		Interface:    sv.Interface,
		Connected:    sv.Current != nil,
		Country:      sv.Country,
		MAC:          sv.MAC,
		Samples:      sv.Samples,
		SSIDRedacted: redactedAnywhere(sv),
	}
	if out.SSIDRedacted {
		out.RedactionNot = RedactionNotice
	}
	if sv.Current != nil {
		n := jsonNetwork(*sv.Current)
		out.Current = &n
	}
	for _, a := range sv.Neighbors {
		out.Neighbors = append(out.Neighbors, JSONNeighor{
			JSONNetwork:   jsonNetwork(a.Network),
			SeenInSamples: a.SeenCount,
			Intermittent:  a.Intermittent(sv.Samples),
		})
	}
	for _, rep := range reports {
		if rep == nil {
			continue
		}
		jb := JSONBand{Band: rep.Band.String()}
		if rep.Best != nil {
			ch := rep.Best.Channel
			jb.Best = &ch
		}
		for _, l := range rep.Loads {
			jc := JSONChannel{
				Channel:           l.Channel,
				CoChannelBSS:      l.CoChannelBSS,
				CoChannelBelowCCA: l.CoChannelBelowCCA,
				IsSelf:            rep.SelfLoad != nil && l.Channel == rep.SelfLoad.Channel,
				IsDFS:             IsDFS(rep.Band, l.Channel),
				TrulyEmpty:        l.TrulyEmpty(),
				SeenInSamples:     l.SeenInSamples,
			}
			// -Inf 不是合法 JSON，用 null 表示「无邻道干扰」
			if !math.IsInf(l.AdjNoiseDBm, -1) {
				v := math.Round(l.AdjNoiseDBm*10) / 10
				jc.AdjNoiseDBm = &v
			}
			jb.Channels = append(jb.Channels, jc)
		}
		out.Bands = append(out.Bands, jb)
	}
	return out
}

func jsonNetwork(n Network) JSONNetwork {
	return JSONNetwork{
		SSID:     n.SSID,
		Redacted: n.Redacted,
		Band:     n.Band.String(),
		Channel:  n.Channel,
		WidthMHz: n.WidthMHz,
		Occupied: Expand(n.Band, n.Channel, n.WidthMHz),
		PHYMode:  n.PHYMode,
		Security: n.Security,
		RSSI:     n.RSSI,
		Noise:    n.Noise,
		SNR:      n.SNR(),
		SNRGrade: SNRGrade(n.SNR()),
		RateMbps: n.Rate,
		MCSIndex: n.MCS,
	}
}
