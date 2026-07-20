package wifix

import (
	"fmt"
	"regexp"
	"slices"
	"strconv"
	"strings"

	"howett.net/plist"
)

// RedactedName 是 macOS 在没有定位权限时给 SSID 的占位值。
//
// macOS 14 起 SSID/BSSID 被归类为位置信息（WiFi 指纹能定位到街区级）。
// CLI 二进制没法自己申请这个权限 —— TCC 授权绑定 app bundle，jdan 继承的
// 是终端的权限。所以只能检测并告知，不能绕。
const RedactedName = "<redacted>"

// ---- plist 原始结构 ----

type spReport []struct {
	Items []spItem `plist:"_items"`
}

type spItem struct {
	Interfaces []rawIface `plist:"spairport_airport_interfaces"`
}

type rawIface struct {
	Name              string       `plist:"_name"`
	Status            string       `plist:"spairport_status_information"`
	Current           *rawNetwork  `plist:"spairport_current_network_information"`
	Others            []rawNetwork `plist:"spairport_airport_other_local_wireless_networks"`
	SupportedChannels []string     `plist:"spairport_supported_channels"`
	CountryCode       string       `plist:"spairport_wireless_country_code"`
	MACAddress        string       `plist:"spairport_wireless_mac_address"`
	PHYModes          string       `plist:"spairport_supported_phymodes"`
}

type rawNetwork struct {
	Name        string `plist:"_name"`
	Channel     string `plist:"spairport_network_channel"`
	PHYMode     string `plist:"spairport_network_phymode"`
	Security    string `plist:"spairport_security_mode"`
	SignalNoise string `plist:"spairport_signal_noise"`
	Rate        int    `plist:"spairport_network_rate"`
	MCS         int    `plist:"spairport_network_mcs"`
}

// ---- 领域模型 ----

// Network 是一个 AP 的解析结果。
type Network struct {
	SSID     string // 脱敏时为空，看 Redacted
	Redacted bool   // SSID 被 macOS 脱敏（显式布尔，不让下游去比字符串）
	Band     Band
	Channel  int
	WidthMHz int
	PHYMode  string
	Security string // 已剥离前缀并人类可读
	RSSI     int    // dBm
	Noise    int    // dBm
	Rate     int    // Mbps，仅当前连接有
	MCS      int    // 仅当前连接有
}

// SNR 返回信噪比 dB。噪声为 0（未知）时返回 0。
func (n Network) SNR() int {
	if n.Noise == 0 {
		return 0
	}
	return n.RSSI - n.Noise
}

// AP 转成干扰模型用的精简结构。
func (n Network) AP() AP {
	return AP{Band: n.Band, Channel: n.Channel, WidthMHz: n.WidthMHz, RSSI: n.RSSI}
}

// ChannelSpec 是一个「频段 + 信道号」对。
//
// 必须成对保存：supported_channels 是把所有频段混在一个平表里的
// （"1 (2GHz)" 和 "36 (5GHz)" 在同一个数组），只留数字就没法区分
// 2.4GHz 的 ch1 和 6GHz 的 ch1 —— 那正是本包在解析层反复强调不能做的事。
type ChannelSpec struct {
	Band    Band
	Channel int
}

// Interface 是一个无线接口的完整状态。
type Interface struct {
	Name              string
	Connected         bool
	Current           *Network // 未连接时为 nil
	Neighbors         []Network
	SupportedChannels []ChannelSpec
	CountryCode       string
	MACAddress        string
}

// ChannelsIn 返回本机在指定频段支持的信道号，升序。
func (i Interface) ChannelsIn(b Band) []int {
	var out []int
	for _, cs := range i.SupportedChannels {
		if cs.Band == b {
			out = append(out, cs.Channel)
		}
	}
	slices.Sort(out)
	return out
}

// ---- 解析 ----

// 安全模式的前缀在 macOS 上**不统一**，这是 Apple 自己的 bug：
//
//	spairport_security_mode_wpa2_personal          （正常）
//	pairport_security_mode_wpa3_transition         （少了开头的 s）
//
// 实测同一次扫描里两种混用，且本机自己的连接正是 pairport_ 那种。按固定
// 前缀剥离会漏掉所有 WPA3 transition 网络。
var secPrefixRe = regexp.MustCompile(`^s?pairport_security_mode_`)

// "36 (5GHz, 80MHz)" / "11 (2GHz, 20MHz)" / "1 (6GHz, 160MHz)"
var chanRe = regexp.MustCompile(`^(\d+)\s*\(([^,)]+)(?:,\s*(\d+)MHz)?\)`)

// "-41 dBm / -88 dBm"
var signalRe = regexp.MustCompile(`(-?\d+)\s*dBm\s*/\s*(-?\d+)\s*dBm`)

// "36 (5GHz)" —— 频段必须一起抓，不能只留数字
var supportedChanRe = regexp.MustCompile(`^(\d+)\s*\(([^)]+)\)`)

// Parse 解析 `system_profiler -xml SPAirPortDataType` 的输出。
//
// 返回所有无线接口。调用方通常只关心 Connected 的那个 —— 实测机器上除了
// en0 还有 awdl0（AirDrop 的 P2P 接口），后者没有 status 字段、
// CNI 只有一个键，直接遍历访问信道字段会拿到空值。
func Parse(data []byte) ([]Interface, error) {
	var report spReport
	if _, err := plist.Unmarshal(data, &report); err != nil {
		return nil, fmt.Errorf("解析 system_profiler 输出失败：%w", err)
	}
	if len(report) == 0 || len(report[0].Items) == 0 {
		return nil, fmt.Errorf("system_profiler 输出为空（本机可能无无线硬件）")
	}

	var out []Interface
	for _, item := range report[0].Items {
		for _, ri := range item.Interfaces {
			out = append(out, parseIface(ri))
		}
	}
	if len(out) == 0 {
		return nil, fmt.Errorf("未找到无线接口")
	}
	return out, nil
}

func parseIface(ri rawIface) Interface {
	iface := Interface{
		Name:        ri.Name,
		Connected:   ri.Status == "spairport_status_connected",
		CountryCode: ri.CountryCode,
		MACAddress:  ri.MACAddress,
	}

	// supported_channels 是**字符串数组**（"36 (5GHz)"），不是整数数组。
	// 频段与信道号一起保存 —— 只留数字就没法区分 2.4GHz ch1 和 6GHz ch1。
	for _, s := range ri.SupportedChannels {
		m := supportedChanRe.FindStringSubmatch(s)
		if m == nil {
			continue
		}
		ch, err := strconv.Atoi(m[1])
		if err != nil {
			continue
		}
		b := ParseBand(strings.TrimSpace(m[2]))
		if b == BandUnknown {
			continue
		}
		iface.SupportedChannels = append(iface.SupportedChannels, ChannelSpec{Band: b, Channel: ch})
	}

	// awdl0 的 CNI 只有 spairport_network_type 一个键，解析出来的 Network
	// 全是零值。只在真正连接时才认这个字段。
	if ri.Current != nil && iface.Connected {
		n := parseNetwork(*ri.Current)
		iface.Current = &n
	}
	for _, rn := range ri.Others {
		iface.Neighbors = append(iface.Neighbors, parseNetwork(rn))
	}
	return iface
}

func parseNetwork(rn rawNetwork) Network {
	n := Network{
		PHYMode: rn.PHYMode,
		Rate:    rn.Rate,
		MCS:     rn.MCS,
	}

	if rn.Name == RedactedName {
		n.Redacted = true
	} else {
		n.SSID = rn.Name
	}

	if m := chanRe.FindStringSubmatch(rn.Channel); m != nil {
		n.Channel, _ = strconv.Atoi(m[1])
		// 频段必须按 band 字符串判定，不能按信道号推断 ——
		// 6GHz 的信道号也从 1 开始，与 2.4GHz 碰撞。
		n.Band = ParseBand(strings.TrimSpace(m[2]))
		if m[3] != "" {
			n.WidthMHz, _ = strconv.Atoi(m[3])
		} else {
			n.WidthMHz = 20
		}
	}

	if m := signalRe.FindStringSubmatch(rn.SignalNoise); m != nil {
		n.RSSI, _ = strconv.Atoi(m[1])
		n.Noise, _ = strconv.Atoi(m[2])
	}

	n.Security = humanSecurity(rn.Security)
	return n
}

// humanSecurity 把 spairport_security_mode_* 枚举转成人类可读。
//
// 未知值**原样输出**而不是空串 —— 空串会让用户以为是开放网络，
// 而 macOS 随时可能加新的模式名。
func humanSecurity(raw string) string {
	if raw == "" {
		return ""
	}
	s := secPrefixRe.ReplaceAllString(raw, "")
	if s == raw {
		return raw // 前缀没匹配上，原样返回
	}
	switch s {
	case "none", "open":
		return "开放"
	case "wep":
		return "WEP"
	case "wpa_personal":
		return "WPA Personal"
	case "wpa2_personal":
		return "WPA2 Personal"
	case "wpa2_personal_mixed":
		return "WPA/WPA2 Personal"
	case "wpa3_personal":
		return "WPA3 Personal"
	case "wpa3_transition":
		return "WPA2/WPA3 Personal"
	case "wpa3_enterprise":
		return "WPA3 Enterprise"
	case "wpa2_enterprise":
		return "WPA2 Enterprise"
	case "wpa_enterprise":
		return "WPA Enterprise"
	default:
		return s // 未知模式名原样透出，不吞
	}
}

// Connected 从接口列表里挑出已连接的那个。都没连接时返回 nil。
func Connected(ifaces []Interface) *Interface {
	for i := range ifaces {
		if ifaces[i].Connected {
			return &ifaces[i]
		}
	}
	return nil
}
