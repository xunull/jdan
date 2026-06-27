package cli

import (
	"bufio"
	"fmt"
	"io"
	"os"
	"strings"

	"github.com/spf13/cobra"

	"github.com/xunull/jdan/internal/qrcode"
	"github.com/xunull/jdan/internal/wifiqr"
)

type qrWifiCmdDeps struct {
	out io.Writer
	in  io.Reader
}

func newQRWifiCommand(deps qrWifiCmdDeps) *cobra.Command {
	if deps.out == nil {
		deps.out = os.Stdout
	}
	if deps.in == nil {
		deps.in = os.Stdin
	}
	cmd := &cobra.Command{
		Use:   "qrwifi [ssid]",
		Short: "生成 WiFi 入网二维码（扫码即连）",
		Long: `生成 WiFi 入网二维码：手机相机/微信扫一下直接连网，不用念密码。
按 WIFI: 标准拼 payload，自动转义 SSID/密码里的特殊字符（手搓最易错的地方），
渲染复用 jdan qr 管线（终端 / PNG / SVG），0 新依赖。

例：
  jdan qrwifi MyNetwork -p 's3cr3t'              # 终端二维码
  jdan qrwifi --ssid "Cafe Guest" --auth nopass  # 开放网络，无密码
  jdan qrwifi Home -p pw --hidden                # 隐藏网络
  jdan qrwifi Home -p pw -o wifi.png             # 存 PNG（贴墙上）
  jdan qrwifi Home --password-stdin <<< 'pw'     # 密码走 stdin，不进 shell history
  jdan qrwifi Home -p pw --json                  # {ssid,auth,hidden,payload,...}`,
		Args:          cobra.MaximumNArgs(1),
		SilenceUsage:  true,
		SilenceErrors: true,
		RunE: func(cmd *cobra.Command, args []string) error {
			ssidFlag, _ := cmd.Flags().GetString("ssid")
			password, _ := cmd.Flags().GetString("password")
			pwStdin, _ := cmd.Flags().GetBool("password-stdin")
			authStr, _ := cmd.Flags().GetString("auth")
			hidden, _ := cmd.Flags().GetBool("hidden")

			ssid := ssidFlag
			if len(args) > 0 {
				if ssid != "" {
					return fmt.Errorf("SSID 同时用位置参数和 --ssid 给了，二选一")
				}
				ssid = args[0]
			}

			auth, err := wifiqr.ParseAuth(authStr)
			if err != nil {
				return err
			}

			if pwStdin {
				if password != "" {
					return fmt.Errorf("--password 和 --password-stdin 二选一")
				}
				password, err = readPasswordLine(deps.in)
				if err != nil {
					return err
				}
			}

			payload, err := wifiqr.Payload(wifiqr.Config{
				SSID:     ssid,
				Password: password,
				Auth:     auth,
				Hidden:   hidden,
			})
			if err != nil {
				return err
			}

			// 复用 qr 的渲染选项与输出分发
			eccStr, _ := cmd.Flags().GetString("ecc")
			ecc, err := qrcode.ParseECC(eccStr)
			if err != nil {
				return err
			}
			invert, _ := cmd.Flags().GetBool("invert")
			fullBlock, _ := cmd.Flags().GetBool("full-block")
			output, _ := cmd.Flags().GetString("output")
			pngSize, _ := cmd.Flags().GetInt("png-size")
			svgModule, _ := cmd.Flags().GetInt("svg-module")
			asJSON, _ := cmd.Flags().GetBool("json")
			opts := qrcode.Options{ECC: ecc, Invert: invert, FullBlock: fullBlock}

			if asJSON {
				return emitWifiJSON(deps.out, ssid, auth, hidden, payload, opts)
			}
			return emitQR(deps.out, payload, opts, output, pngSize, svgModule, false)
		},
	}

	cmd.Flags().StringP("ssid", "s", "", "网络名（也可用位置参数）")
	cmd.Flags().StringP("password", "p", "", "密码（nopass 时忽略；注意 -p 会进 shell history）")
	cmd.Flags().Bool("password-stdin", false, "从 stdin 读密码（避免进 shell history / ps）")
	cmd.Flags().StringP("auth", "a", "wpa", "认证类型：wpa / wep / nopass")
	cmd.Flags().Bool("hidden", false, "隐藏网络（H:true）")

	// 复用 qr 的渲染 flag
	cmd.Flags().String("ecc", "M", "纠错级别 L/M/Q/H")
	cmd.Flags().Bool("invert", false, "反色（适合白底终端）")
	cmd.Flags().Bool("full-block", false, "用全角 ██（兼容老终端）")
	cmd.Flags().String("output", "", "写入文件，按扩展名识别 .png/.svg")
	cmd.Flags().Int("png-size", 256, "PNG 输出像素尺寸")
	cmd.Flags().Int("svg-module", 8, "SVG 每模块像素数")
	cmd.Flags().Bool("json", false, "输出 {ssid, auth, hidden, payload, ecc, modules} JSON")
	return cmd
}

// readPasswordLine 从 reader 读一行作为密码（去掉行尾换行，不去其它空白——密码可能含空格）。
func readPasswordLine(r io.Reader) (string, error) {
	br := bufio.NewReader(r)
	line, err := br.ReadString('\n')
	if err != nil && err != io.EOF {
		return "", err
	}
	return strings.TrimRight(line, "\r\n"), nil
}

func emitWifiJSON(out io.Writer, ssid string, auth wifiqr.Auth, hidden bool, payload string, opts qrcode.Options) error {
	info, err := qrcode.Describe(payload, opts)
	if err != nil {
		return err
	}
	// Describe 返回 {data, ecc, modules}；这里包一层 WiFi 元信息 + payload（密码明文仅在此暴露）
	return writeIndentJSON(out, map[string]any{
		"ssid":    ssid,
		"auth":    string(auth),
		"hidden":  hidden,
		"payload": payload,
		"qr":      info,
	})
}

func init() {
	rootCmd.AddCommand(newQRWifiCommand(qrWifiCmdDeps{}))
}
