package cli

import (
	"fmt"
	"io"
	"os"
	"os/exec"
	"runtime"
	"time"

	"github.com/spf13/cobra"

	"github.com/xunull/jdan/internal/wifix"
)

type wifiCmdDeps struct {
	out    io.Writer
	errOut io.Writer              // 进度写这里；非 TTY 时静默
	scan   func() ([]byte, error) // 注入便于测试
	now    func() time.Time       // 注入便于测试
}

// wifiSpinnerInterval 是进度动画的刷新间隔。
//
// system_profiler 单次约 1.7 秒且**不透明**（没有增量可读），所以这里是
// 转圈而不是计数 —— 跟 size 命令那个「已扫描 N 个条目」的进度不同，
// 那个能读原子计数器，这个只能告诉用户「还活着」。
const wifiSpinnerInterval = 120 * time.Millisecond

func newWifiCommand(deps wifiCmdDeps) *cobra.Command {
	if deps.out == nil {
		deps.out = os.Stdout
	}
	if deps.errOut == nil {
		deps.errOut = os.Stderr
	}
	if deps.scan == nil {
		deps.scan = scanAirport
	}
	if deps.now == nil {
		deps.now = time.Now
	}

	cmd := &cobra.Command{
		Use:   "wifi",
		Short: "WiFi 状态与信道拥挤分析（找出该换到哪个信道）",
		Long: `看当前无线连接状态，并分析周边 AP 的信道占用，给出换信道建议。

回答的是菜单栏不会告诉你的事：信道撞没撞、SNR 够不够、协商到了哪代协议、
邻居把哪个频段占满了。

信道分析用两个物理量而不是一个合成分：
  同信道 BSS 数 —— CSMA/CA 退避的代价，按空口时间算，与相对强度无关
  邻道噪声      —— 不可解码的能量当噪声，按线性功率求和

压成一个分会让强信号邻居主导排序，低估「很多个中等强度同信道 BSS」
这种真实很糟的情况。

关于 SSID：macOS 14 起把它归类为位置信息，CLI 拿不到（会显示为脱敏）。
其余射频数据不受影响 —— 而信道分析恰好一个都不需要 SSID。

例：
  jdan wifi                  当前状态 + 信道占用分析
  jdan wifi --samples 5      多采样 5 次（扫描结果不稳，默认 3 次）
  jdan wifi --all-channels   列出全部候选信道而非前 8 个
  jdan wifi --band 5         只看 5GHz
  jdan wifi --json | jq      完整数据`,
		Args:          cobra.NoArgs,
		SilenceUsage:  true,
		SilenceErrors: true,
		RunE: func(cmd *cobra.Command, args []string) error {
			if runtime.GOOS != "darwin" {
				return fmt.Errorf("jdan wifi 目前仅支持 macOS。"+
					"Linux 上拿完整的信道扫描需要 iw 或 nl80211，尚未实现（当前平台：%s）", runtime.GOOS)
			}

			samples, _ := cmd.Flags().GetInt("samples")
			iface, _ := cmd.Flags().GetString("interface")
			bandFlag, _ := cmd.Flags().GetString("band")
			allChannels, _ := cmd.Flags().GetBool("all-channels")
			noColor, _ := cmd.Flags().GetBool("no-color")
			asJSON, _ := cmd.Flags().GetBool("json")

			if samples < 1 {
				samples = 1
			}

			// 进度只在 stderr 是 TTY 且不输出 JSON 时显示：管道场景静默，
			// 免得污染下游。
			stop := func() {}
			if !asJSON && isTTY(deps.errOut) {
				stop = startWifiSpinner(deps.errOut, samples)
			}

			start := deps.now()
			sv, err := wifix.Collect(deps.scan, samples, iface)
			stop()
			if err != nil {
				if wifix.IsNotConnected(err) {
					// 未连接不是失败：仍然打印出来，退出码 0。
					fmt.Fprintln(deps.out, "未连接任何 WiFi 网络。")
					return nil
				}
				return err
			}
			elapsed := deps.now().Sub(start)

			bands := selectBands(sv, bandFlag)
			reports := make([]*wifix.ChannelReport, 0, len(bands))
			for _, b := range bands {
				reports = append(reports, wifix.AnalyzeBand(sv, b, selfWidth(sv, b)))
			}

			if asJSON {
				return writeIndentJSON(deps.out, wifix.JSONData(sv, reports))
			}

			maxWidth := 0
			if isTTY(deps.out) {
				maxWidth = termWidth(deps.out)
			}
			fmt.Fprint(deps.out, wifix.Render(sv, reports, wifix.RenderOptions{
				Color:    !noColor && !noColorEnv() && isTTY(deps.out),
				MaxWidth: maxWidth,
				Elapsed:  elapsed,
				ShowAll:  allChannels,
			}))
			return nil
		},
	}

	cmd.Flags().Int("samples", 3, "扫描采样次数（扫描结果不稳，多次取并集）")
	cmd.Flags().String("interface", "", "指定无线接口（默认自动选已连接的那个）")
	cmd.Flags().String("band", "", "只分析指定频段：2.4 / 5 / 6（默认全部）")
	cmd.Flags().Bool("all-channels", false, "列出全部候选信道而非前 8 个")
	cmd.Flags().Bool("no-color", false, "关闭染色（同时尊重 NO_COLOR 环境变量）")
	cmd.Flags().Bool("json", false, "输出完整数据（不受展示层裁剪影响）")
	return cmd
}

// scanAirport 跑一次 system_profiler。
//
// 为什么是它：airport 二进制在 macOS 26 上已被删除；networksetup
// -getairportnetwork 在已连接时会谎报「未关联」；wdutil info 要 sudo。
// system_profiler 是唯一免 sudo 且带邻居扫描的路径，代价是单次约 1.7 秒。
func scanAirport() ([]byte, error) {
	out, err := exec.Command("system_profiler", "-xml", "SPAirPortDataType").Output()
	if err != nil {
		return nil, fmt.Errorf("执行 system_profiler 失败：%w", err)
	}
	return out, nil
}

// selectBands 按 --band 过滤要分析的频段；未指定时用本机支持的全部频段。
func selectBands(sv *wifix.Survey, flag string) []wifix.Band {
	all := sv.Bands()
	if flag == "" {
		return all
	}
	var want wifix.Band
	switch flag {
	case "2.4", "2", "2.4G", "2G":
		want = wifix.Band24
	case "5", "5G":
		want = wifix.Band5
	case "6", "6G":
		want = wifix.Band6
	default:
		return all
	}
	for _, b := range all {
		if b == want {
			return []wifix.Band{b}
		}
	}
	return nil
}

// selfWidth 是评估候选信道时假设使用的带宽。
//
// 用本机当前带宽：换信道通常不改带宽，而带宽决定了候选信道实际占用哪些
// 20MHz 子信道，进而决定谁算同信道。本机没连接时退回 20MHz（最保守，
// 所有信道都能承载）。
func selfWidth(sv *wifix.Survey, b wifix.Band) int {
	if sv.Current != nil && sv.Current.Band == b && sv.Current.WidthMHz > 0 {
		return sv.Current.WidthMHz
	}
	return 20
}

// startWifiSpinner 起一个转圈动画，返回停止函数。
//
// 转圈而非计数：system_profiler 是不透明的外部命令，没有增量进度可读。
// size 命令那套「已扫描 N 个条目」在这里用不了 —— 那个能读原子计数器。
func startWifiSpinner(w io.Writer, samples int) func() {
	done := make(chan struct{})
	finished := make(chan struct{})

	go func() {
		defer close(finished)
		frames := []rune{'⠋', '⠙', '⠹', '⠸', '⠼', '⠴', '⠦', '⠧', '⠇', '⠏'}
		ticker := time.NewTicker(wifiSpinnerInterval)
		defer ticker.Stop()
		i := 0
		painted := false
		for {
			select {
			case <-done:
				if painted {
					fmt.Fprint(w, "\r\033[K") // 擦掉，不留在最终输出里
				}
				return
			case <-ticker.C:
				hint := "正在扫描无线环境…"
				if samples > 1 {
					hint = fmt.Sprintf("正在扫描无线环境（%d 次采样，约 %.0f 秒）…",
						samples, float64(samples)*1.8)
				}
				fmt.Fprintf(w, "\r\033[K%c %s", frames[i%len(frames)], hint)
				painted = true
				i++
			}
		}
	}()

	return func() {
		close(done)
		<-finished
	}
}

func init() {
	rootCmd.AddCommand(newWifiCommand(wifiCmdDeps{}))
}
