package cli

import (
	"bytes"
	"encoding/json"
	"errors"
	"os"
	"runtime"
	"strings"
	"testing"
	"time"

	"github.com/xunull/jdan/internal/wifix"
)

func wifiFixture(t *testing.T) []byte {
	t.Helper()
	b, err := os.ReadFile("../wifix/testdata/airport-macos26.plist")
	if err != nil {
		t.Fatal(err)
	}
	return b
}

func runWifi(t *testing.T, scan func() ([]byte, error), args ...string) (stdout, stderr string, err error) {
	t.Helper()
	var out, errOut bytes.Buffer
	fixed := time.Unix(0, 0)
	cmd := newWifiCommand(wifiCmdDeps{
		out: &out, errOut: &errOut, scan: scan,
		now: func() time.Time { return fixed },
	})
	cmd.SetArgs(args)
	cmd.SetOut(&out)
	cmd.SetErr(&errOut)
	err = cmd.Execute()
	return out.String(), errOut.String(), err
}

func TestWifiCommand_RendersCurrentAndChannels(t *testing.T) {
	if runtime.GOOS != "darwin" {
		t.Skip("jdan wifi 仅支持 macOS")
	}
	data := wifiFixture(t)
	out, _, err := runWifi(t, func() ([]byte, error) { return data, nil }, "--samples", "1")
	if err != nil {
		t.Fatal(err)
	}
	for _, want := range []string{"en0", "802.11ax", "信道 36", "占用 36/40/44/48", "SNR", "5GHz 信道占用"} {
		if !strings.Contains(out, want) {
			t.Errorf("输出缺 %q：\n%s", want, out)
		}
	}
}

// 本机所在信道必须始终可见。它按干扰排序常掉出前 N —— 实测本机 ch36
// 有 4 个同信道 BSS，被排在 149/52 之后截掉了，于是建议里写着「当前信道
// 36 上有 4 个 BSS」而表格里根本没有 36 那行。
func TestWifiCommand_SelfChannelAlwaysVisible(t *testing.T) {
	if runtime.GOOS != "darwin" {
		t.Skip("jdan wifi 仅支持 macOS")
	}
	data := wifiFixture(t)
	out, _, err := runWifi(t, func() ([]byte, error) { return data, nil }, "--samples", "1", "--band", "5")
	if err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(out, "★") {
		t.Errorf("应有 ★ 标记本机信道：\n%s", out)
	}
	// 建议里提到的信道号必须在表格里出现
	if strings.Contains(out, "建议：当前信道 36") && !strings.Contains(out, "36 ★") {
		t.Errorf("建议提到 ch36 但表格里没有 36 那行：\n%s", out)
	}
}

// --json 不受展示层裁剪影响：全部信道、全部邻居。
func TestWifiCommand_JSONIsNotTruncated(t *testing.T) {
	if runtime.GOOS != "darwin" {
		t.Skip("jdan wifi 仅支持 macOS")
	}
	data := wifiFixture(t)
	txt, _, err := runWifi(t, func() ([]byte, error) { return data, nil }, "--samples", "1", "--band", "5")
	if err != nil {
		t.Fatal(err)
	}
	js, _, err := runWifi(t, func() ([]byte, error) { return data, nil }, "--samples", "1", "--band", "5", "--json")
	if err != nil {
		t.Fatal(err)
	}

	var rep wifix.JSONReport
	if err := json.Unmarshal([]byte(js), &rep); err != nil {
		t.Fatalf("JSON 不可解析：%v", err)
	}
	if len(rep.Bands) == 0 {
		t.Fatal("应有频段数据")
	}
	// 文本默认只列 8 个（+本机），JSON 必须给全
	if len(rep.Bands[0].Channels) <= 8 {
		t.Skip("候选信道不足 8 个，测不出裁剪差异")
	}
	textLines := strings.Count(txt, "░") + strings.Count(txt, "█")
	if textLines == 0 {
		t.Error("文本输出应有占用条")
	}
	t.Logf("JSON 给出 %d 个信道，文本裁剪后只列前 8 个 + 本机", len(rep.Bands[0].Channels))
}

// SSID 脱敏必须是显式布尔字段，且文本输出要给出授权路径。
func TestWifiCommand_RedactionIsExplicitAndExplained(t *testing.T) {
	if runtime.GOOS != "darwin" {
		t.Skip("jdan wifi 仅支持 macOS")
	}
	data := wifiFixture(t)

	txt, _, err := runWifi(t, func() ([]byte, error) { return data, nil }, "--samples", "1")
	if err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(txt, "已脱敏") {
		t.Errorf("脱敏时应明说而不是留白：\n%s", txt)
	}
	for _, want := range []string{"位置信息", "定位服务"} {
		if !strings.Contains(txt, want) {
			t.Errorf("应给出脱敏原因和授权路径，缺 %q", want)
		}
	}

	js, _, err := runWifi(t, func() ([]byte, error) { return data, nil }, "--samples", "1", "--json")
	if err != nil {
		t.Fatal(err)
	}
	var rep wifix.JSONReport
	if err := json.Unmarshal([]byte(js), &rep); err != nil {
		t.Fatal(err)
	}
	if !rep.SSIDRedacted {
		t.Error("JSON 应有显式的 ssid_redacted 布尔字段")
	}
	if rep.RedactionNot == "" {
		t.Error("JSON 应带脱敏说明")
	}
}

// -Inf 不是合法 JSON。无邻道干扰时必须是 null 而不是 0 —— 0 dBm 是 1 毫瓦，
// 会被读成极强信号。
func TestWifiCommand_JSONInfinityBecomesNull(t *testing.T) {
	if runtime.GOOS != "darwin" {
		t.Skip("jdan wifi 仅支持 macOS")
	}
	data := wifiFixture(t)
	js, _, err := runWifi(t, func() ([]byte, error) { return data, nil }, "--samples", "1", "--json")
	if err != nil {
		t.Fatal(err)
	}
	if strings.Contains(js, "-Inf") || strings.Contains(js, "\"+Inf\"") {
		t.Errorf("JSON 里不该出现 Inf：\n%s", js)
	}
	var raw map[string]any
	if err := json.Unmarshal([]byte(js), &raw); err != nil {
		t.Fatalf("JSON 不可解析（Inf 会导致这里失败）：%v", err)
	}
	// 找一个 adjacent_noise_dbm 为 null 的信道
	bands := raw["bands"].([]any)
	var sawNull bool
	for _, b := range bands {
		for _, c := range b.(map[string]any)["channels"].([]any) {
			if c.(map[string]any)["adjacent_noise_dbm"] == nil {
				sawNull = true
			}
		}
	}
	if !sawNull {
		t.Skip("本次数据没有无邻道干扰的信道")
	}
}

// 非 TTY 时进度必须静默，否则污染管道下游。
func TestWifiCommand_NoSpinnerOnNonTTY(t *testing.T) {
	if runtime.GOOS != "darwin" {
		t.Skip("jdan wifi 仅支持 macOS")
	}
	data := wifiFixture(t)
	_, stderr, err := runWifi(t, func() ([]byte, error) { return data, nil }, "--samples", "1")
	if err != nil {
		t.Fatal(err)
	}
	if stderr != "" {
		t.Errorf("非 TTY 时 stderr 应为空，得到 %q", stderr)
	}
}

// --samples 决定扫描次数。
func TestWifiCommand_SamplesControlsScanCount(t *testing.T) {
	if runtime.GOOS != "darwin" {
		t.Skip("jdan wifi 仅支持 macOS")
	}
	data := wifiFixture(t)
	for _, n := range []string{"1", "3", "5"} {
		calls := 0
		if _, _, err := runWifi(t, func() ([]byte, error) { calls++; return data, nil }, "--samples", n); err != nil {
			t.Fatal(err)
		}
		want := map[string]int{"1": 1, "3": 3, "5": 5}[n]
		if calls != want {
			t.Errorf("--samples %s 应扫描 %d 次，实际 %d 次", n, want, calls)
		}
	}
}

func TestWifiCommand_ScanFailurePropagates(t *testing.T) {
	if runtime.GOOS != "darwin" {
		t.Skip("jdan wifi 仅支持 macOS")
	}
	boom := errors.New("system_profiler 挂了")
	_, _, err := runWifi(t, func() ([]byte, error) { return nil, boom }, "--samples", "1")
	if err == nil {
		t.Fatal("扫描失败应返回错误")
	}
	if !strings.Contains(err.Error(), "system_profiler") {
		t.Errorf("错误应保留原因：%v", err)
	}
}

// 未连接不是失败：打印说明，退出码 0。
func TestWifiCommand_NotConnectedExitsZero(t *testing.T) {
	if runtime.GOOS != "darwin" {
		t.Skip("jdan wifi 仅支持 macOS")
	}
	// 造一份没有已连接接口的 plist
	data := bytes.ReplaceAll(wifiFixture(t),
		[]byte("spairport_status_connected"), []byte("spairport_status_disconnected"))

	out, _, err := runWifi(t, func() ([]byte, error) { return data, nil }, "--samples", "1")
	if err != nil {
		t.Fatalf("未连接不该返回错误：%v", err)
	}
	if !strings.Contains(out, "未连接") {
		t.Errorf("应说明未连接：\n%s", out)
	}
}

func TestWifiCommand_BandFilter(t *testing.T) {
	if runtime.GOOS != "darwin" {
		t.Skip("jdan wifi 仅支持 macOS")
	}
	data := wifiFixture(t)
	only5, _, err := runWifi(t, func() ([]byte, error) { return data, nil }, "--samples", "1", "--band", "5")
	if err != nil {
		t.Fatal(err)
	}
	if strings.Contains(only5, "2.4GHz 信道占用") {
		t.Errorf("--band 5 不该输出 2.4GHz 段：\n%s", only5)
	}
	if !strings.Contains(only5, "5GHz 信道占用") {
		t.Errorf("--band 5 应输出 5GHz 段：\n%s", only5)
	}

	both, _, err := runWifi(t, func() ([]byte, error) { return data, nil }, "--samples", "1")
	if err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(both, "2.4GHz 信道占用") || !strings.Contains(both, "5GHz 信道占用") {
		t.Errorf("默认应输出全部频段：\n%s", both)
	}
}

func TestWifiCommand_NoColorStripsANSI(t *testing.T) {
	if runtime.GOOS != "darwin" {
		t.Skip("jdan wifi 仅支持 macOS")
	}
	data := wifiFixture(t)
	out, _, err := runWifi(t, func() ([]byte, error) { return data, nil }, "--samples", "1", "--no-color")
	if err != nil {
		t.Fatal(err)
	}
	if strings.Contains(out, "\x1b") {
		t.Errorf("--no-color 不该有 ANSI 转义：%q", out)
	}
}

func TestSelectBands(t *testing.T) {
	sv := &wifix.Survey{SupportedChannels: []wifix.ChannelSpec{
		{Band: wifix.Band24, Channel: 1},
		{Band: wifix.Band5, Channel: 36},
	}}
	if got := selectBands(sv, ""); len(got) != 2 {
		t.Errorf("未指定时应返回全部频段，得到 %v", got)
	}
	for _, f := range []string{"5", "5G"} {
		if got := selectBands(sv, f); len(got) != 1 || got[0] != wifix.Band5 {
			t.Errorf("--band %s 应只返回 5GHz，得到 %v", f, got)
		}
	}
	for _, f := range []string{"2.4", "2", "2G"} {
		if got := selectBands(sv, f); len(got) != 1 || got[0] != wifix.Band24 {
			t.Errorf("--band %s 应只返回 2.4GHz，得到 %v", f, got)
		}
	}
	// 本机不支持的频段返回空而不是全部
	if got := selectBands(sv, "6"); len(got) != 0 {
		t.Errorf("本机不支持 6GHz 时应返回空，得到 %v", got)
	}
	// 无法识别的值退回全部
	if got := selectBands(sv, "什么鬼"); len(got) != 2 {
		t.Errorf("无法识别的 --band 应退回全部，得到 %v", got)
	}
}

func TestSelfWidth(t *testing.T) {
	sv := &wifix.Survey{Current: &wifix.Network{Band: wifix.Band5, Channel: 36, WidthMHz: 80}}
	if got := selfWidth(sv, wifix.Band5); got != 80 {
		t.Errorf("同频段应用本机带宽，得到 %d", got)
	}
	// 其他频段退回 20MHz（最保守，所有信道都能承载）
	if got := selfWidth(sv, wifix.Band24); got != 20 {
		t.Errorf("异频段应退回 20MHz，得到 %d", got)
	}
	if got := selfWidth(&wifix.Survey{}, wifix.Band5); got != 20 {
		t.Errorf("未连接应退回 20MHz，得到 %d", got)
	}
}
