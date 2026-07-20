package wifix

import (
	"os"
	"slices"
	"testing"
)

func fixture(t *testing.T) []Interface {
	t.Helper()
	data, err := os.ReadFile("testdata/airport-macos26.plist")
	if err != nil {
		t.Fatal(err)
	}
	ifaces, err := Parse(data)
	if err != nil {
		t.Fatal(err)
	}
	return ifaces
}

// 实测机器上除了 en0 还有 awdl0（AirDrop 的 P2P 接口）。它没有 status
// 字段、CNI 只有一个键 —— 直接遍历接口访问信道字段会拿到全零的 Network。
func TestParse_SkipsAWDLInterface(t *testing.T) {
	ifaces := fixture(t)

	if len(ifaces) < 2 {
		t.Fatalf("fixture 应含 en0 和 awdl0，得到 %d 个接口", len(ifaces))
	}
	var sawAWDL bool
	for _, i := range ifaces {
		if i.Name == "awdl0" {
			sawAWDL = true
			if i.Connected {
				t.Error("awdl0 不该被判为已连接（它没有 status 字段）")
			}
			if i.Current != nil {
				t.Error("awdl0 不该有 Current —— 它的 CNI 只有 network_type 一个键，" +
					"认下来会得到全零的 Network")
			}
		}
	}
	if !sawAWDL {
		t.Skip("fixture 里没有 awdl0，跳过")
	}

	conn := Connected(ifaces)
	if conn == nil {
		t.Fatal("应能选出已连接的接口")
	}
	if conn.Name != "en0" {
		t.Errorf("已连接接口应为 en0，得到 %s", conn.Name)
	}
}

// macOS 的 security_mode 前缀不统一（Apple 自己的 bug）：WPA3 transition
// 是 pairport_ 少个 s。按固定前缀剥离会漏掉所有 WPA3 transition 网络，
// 而本机自己的连接正是这种。
func TestParse_HandlesBothSecurityPrefixes(t *testing.T) {
	cases := map[string]string{
		"spairport_security_mode_wpa2_personal":       "WPA2 Personal",
		"spairport_security_mode_wpa2_personal_mixed": "WPA/WPA2 Personal",
		"pairport_security_mode_wpa3_transition":      "WPA2/WPA3 Personal", // 少个 s
		"spairport_security_mode_wpa3_personal":       "WPA3 Personal",
		"spairport_security_mode_none":                "开放",
	}
	for raw, want := range cases {
		if got := humanSecurity(raw); got != want {
			t.Errorf("humanSecurity(%q) = %q，应为 %q", raw, got, want)
		}
	}

	// 未知模式名必须原样透出，不能吞成空串 —— 空串会被读成开放网络
	unknown := "spairport_security_mode_wpa4_quantum"
	if got := humanSecurity(unknown); got != "wpa4_quantum" {
		t.Errorf("未知模式应原样透出模式名，得到 %q", got)
	}
	if humanSecurity("完全不认识的东西") != "完全不认识的东西" {
		t.Error("前缀不匹配时应原样返回")
	}
	if humanSecurity("") != "" {
		t.Error("空输入应返回空")
	}
}

// fixture 里两种前缀都有，端到端确认没有漏解析成空串。
func TestParse_NoEmptySecurityInFixture(t *testing.T) {
	conn := Connected(fixture(t))
	if conn == nil {
		t.Fatal("fixture 应有已连接接口")
	}
	if conn.Current.Security == "" {
		t.Error("当前连接的安全模式解析成了空串")
	}
	if conn.Current.Security != "WPA2/WPA3 Personal" {
		t.Errorf("本机是 wpa3_transition（pairport_ 前缀），应解析为 WPA2/WPA3 Personal，得到 %q",
			conn.Current.Security)
	}
	for i, n := range conn.Neighbors {
		if n.Security == "" {
			t.Errorf("邻居 %d 的安全模式解析成了空串", i)
		}
	}
}

func TestParse_ChannelString(t *testing.T) {
	cases := []struct {
		in    string
		band  Band
		ch    int
		width int
	}{
		{"36 (5GHz, 80MHz)", Band5, 36, 80},
		{"11 (2GHz, 20MHz)", Band24, 11, 20},
		{"44 (5GHz, 160MHz)", Band5, 44, 160},
		{"1 (6GHz, 160MHz)", Band6, 1, 160}, // 6GHz ch1，不能被当成 2.4GHz
		{"165 (5GHz)", Band5, 165, 20},      // 无宽度时默认 20
	}
	for _, c := range cases {
		n := parseNetwork(rawNetwork{Channel: c.in})
		if n.Band != c.band || n.Channel != c.ch || n.WidthMHz != c.width {
			t.Errorf("%q → band=%v ch=%d width=%d，应为 %v/%d/%d",
				c.in, n.Band, n.Channel, n.WidthMHz, c.band, c.ch, c.width)
		}
	}
}

// 6GHz 的信道号从 1 开始，与 2.4GHz 碰撞。必须以 band 字符串为准。
func TestParse_6GHzChannel1NotMistakenFor24(t *testing.T) {
	six := parseNetwork(rawNetwork{Channel: "1 (6GHz, 160MHz)"})
	two := parseNetwork(rawNetwork{Channel: "1 (2GHz, 20MHz)"})

	if six.Band != Band6 {
		t.Errorf("6GHz ch1 的频段判成了 %v", six.Band)
	}
	if two.Band != Band24 {
		t.Errorf("2.4GHz ch1 的频段判成了 %v", two.Band)
	}
	if Overlap(six.AP(), two.AP()) != 0 {
		t.Error("6GHz ch1 与 2.4GHz ch1 不该互相干扰")
	}
}

func TestParse_SignalNoise(t *testing.T) {
	n := parseNetwork(rawNetwork{SignalNoise: "-41 dBm / -88 dBm"})
	if n.RSSI != -41 || n.Noise != -88 {
		t.Errorf("信号/噪声 = %d/%d，应为 -41/-88", n.RSSI, n.Noise)
	}
	if n.SNR() != 47 {
		t.Errorf("SNR = %d，应为 47", n.SNR())
	}
	// 缺失时不该崩，SNR 返回 0 而不是一个假的大数
	empty := parseNetwork(rawNetwork{SignalNoise: ""})
	if empty.SNR() != 0 {
		t.Errorf("噪声未知时 SNR 应为 0，得到 %d", empty.SNR())
	}
}

// SSID 脱敏必须是显式布尔字段，不让下游去比字符串。
func TestParse_RedactionIsExplicitFlag(t *testing.T) {
	red := parseNetwork(rawNetwork{Name: RedactedName})
	if !red.Redacted {
		t.Error("<redacted> 应置 Redacted 标志")
	}
	if red.SSID != "" {
		t.Errorf("脱敏时 SSID 应为空而不是字面量 <redacted>，得到 %q", red.SSID)
	}

	normal := parseNetwork(rawNetwork{Name: "MyNetwork"})
	if normal.Redacted || normal.SSID != "MyNetwork" {
		t.Errorf("正常 SSID 解析错误：redacted=%v ssid=%q", normal.Redacted, normal.SSID)
	}

	// fixture 是在无定位权限下抓的，应当全部脱敏
	conn := Connected(fixture(t))
	if !conn.Current.Redacted {
		t.Error("fixture 的当前连接应为脱敏状态")
	}
}

// supported_channels 是字符串数组（"36 (5GHz)"），不是整数数组。
// 它同时承担区域合规兜底：推荐信道必须在这个集合里。
func TestParse_SupportedChannelsAreStrings(t *testing.T) {
	conn := Connected(fixture(t))
	if len(conn.SupportedChannels) == 0 {
		t.Fatal("应解析出支持的信道列表")
	}
	has := func(ch int) bool {
		return slices.Contains(conn.ChannelsIn(Band24), ch) || slices.Contains(conn.ChannelsIn(Band5), ch)
	}
	// 本机 country=CN，实测含 1-13 和 149-165，不含 100-144
	if !has(13) {
		t.Error("CN 应支持 2.4GHz ch13")
	}
	if !has(149) {
		t.Error("CN 应支持 5GHz ch149")
	}
	if has(100) {
		t.Error("CN 不该支持 ch100（实测 supported_channels 里没有）")
	}
	if conn.CountryCode != "CN" {
		t.Errorf("国家码 = %q，fixture 应为 CN", conn.CountryCode)
	}
}

// 端到端：fixture 解析出来的邻居能直接喂进干扰模型。
func TestParse_EndToEndIntoInterferenceModel(t *testing.T) {
	conn := Connected(fixture(t))
	if conn.Current == nil {
		t.Fatal("应有当前连接")
	}
	self := conn.Current

	var neighbors []AP
	for _, n := range conn.Neighbors {
		if n.Band != BandUnknown {
			neighbors = append(neighbors, n.AP())
		}
	}
	if len(neighbors) == 0 {
		t.Fatal("fixture 应有可用邻居")
	}

	load := Analyze(self.Band, self.Channel, self.WidthMHz, neighbors)
	t.Logf("本机 ch%d@%dMHz：同信道 BSS %d 个（另有 %d 个低于 CCA 门限），邻道噪声 %.1f dBm",
		self.Channel, self.WidthMHz, load.CoChannelBSS, load.CoChannelBelowCCA, load.AdjNoiseDBm)

	if load.Channel != self.Channel {
		t.Errorf("Analyze 返回的信道 = %d，应为 %d", load.Channel, self.Channel)
	}
	// fixture 里有 160MHz 的邻居，本机 80MHz —— 跨宽度重叠必须被算到
	var sawWide bool
	for _, n := range conn.Neighbors {
		if n.WidthMHz == 160 {
			sawWide = true
		}
	}
	if !sawWide {
		t.Skip("fixture 无 160MHz 邻居")
	}
	if load.CoChannelBSS == 0 && load.CoChannelBelowCCA == 0 {
		t.Error("fixture 含与本机同块的 160MHz 邻居，同信道计数不该为 0")
	}
}

func TestParse_BadInput(t *testing.T) {
	if _, err := Parse([]byte("not a plist")); err == nil {
		t.Error("非法输入应返回错误")
	}
	if _, err := Parse(nil); err == nil {
		t.Error("空输入应返回错误")
	}
}
