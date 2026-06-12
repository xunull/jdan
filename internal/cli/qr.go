package cli

import (
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"strings"

	"github.com/spf13/cobra"
	"github.com/xunull/jdan/internal/qrcode"
)

type qrCmdDeps struct {
	out io.Writer
	in  io.Reader
}

func newQRCommand(deps qrCmdDeps) *cobra.Command {
	if deps.out == nil {
		deps.out = os.Stdout
	}
	if deps.in == nil {
		deps.in = os.Stdin
	}
	cmd := &cobra.Command{
		Use:   "qr [text]",
		Short: "生成二维码（终端 / PNG / SVG）",
		Long: `生成二维码并输出到终端或文件。

例：
  jdan qr "hello"                  # 终端半角块
  jdan qr "https://example.com"
  echo "data" | jdan qr            # stdin
  jdan qr "data" --output qr.png   # 写 PNG
  jdan qr "data" --output qr.svg   # 写 SVG
  jdan qr "data" --invert          # 反色（白底终端）
  jdan qr "data" --ecc H           # 容错升到高
  jdan qr "data" --full-block      # 全角 ██（兼容老终端）
  jdan qr "data" --json            # 元信息 JSON`,
		Args: cobra.MaximumNArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			eccStr, _ := cmd.Flags().GetString("ecc")
			invert, _ := cmd.Flags().GetBool("invert")
			fullBlock, _ := cmd.Flags().GetBool("full-block")
			output, _ := cmd.Flags().GetString("output")
			pngSize, _ := cmd.Flags().GetInt("png-size")
			svgModule, _ := cmd.Flags().GetInt("svg-module")
			asJSON, _ := cmd.Flags().GetBool("json")

			ecc, err := qrcode.ParseECC(eccStr)
			if err != nil {
				return err
			}
			opts := qrcode.Options{
				ECC:       ecc,
				Invert:    invert,
				FullBlock: fullBlock,
			}
			data, err := readQRData(deps.in, args)
			if err != nil {
				return err
			}
			if asJSON {
				info, err := qrcode.Describe(data, opts)
				if err != nil {
					return err
				}
				enc := json.NewEncoder(deps.out)
				enc.SetIndent("", "  ")
				return enc.Encode(info)
			}
			if output != "" {
				return writeQRFile(output, data, opts, pngSize, svgModule)
			}
			s, err := qrcode.RenderTerminal(data, opts)
			if err != nil {
				return err
			}
			fmt.Fprint(deps.out, s)
			return nil
		},
	}
	cmd.Flags().String("ecc", "M", "纠错级别 L/M/Q/H")
	cmd.Flags().Bool("invert", false, "反色（适合白底终端）")
	cmd.Flags().Bool("full-block", false, "用全角 ██（兼容老终端）")
	cmd.Flags().String("output", "", "写入文件，按扩展名识别 .png/.svg")
	cmd.Flags().Int("png-size", 256, "PNG 输出像素尺寸")
	cmd.Flags().Int("svg-module", 8, "SVG 每模块像素数")
	cmd.Flags().Bool("json", false, "输出 {data, ecc, modules} JSON")
	return cmd
}

func readQRData(r io.Reader, args []string) (string, error) {
	if len(args) > 0 {
		return args[0], nil
	}
	b, err := io.ReadAll(r)
	if err != nil {
		return "", err
	}
	s := strings.TrimRight(string(b), "\n\r")
	if s == "" {
		return "", errors.New("no input: pass text as argument or via stdin")
	}
	return s, nil
}

func writeQRFile(path, data string, opts qrcode.Options, pngSize, svgModule int) error {
	switch strings.ToLower(filepath.Ext(path)) {
	case ".png":
		b, err := qrcode.RenderPNG(data, opts, pngSize)
		if err != nil {
			return err
		}
		return os.WriteFile(path, b, 0o644)
	case ".svg":
		s, err := qrcode.RenderSVG(data, opts, svgModule)
		if err != nil {
			return err
		}
		return os.WriteFile(path, []byte(s), 0o644)
	default:
		return fmt.Errorf("unsupported output extension %q (use .png or .svg)", filepath.Ext(path))
	}
}

func init() {
	rootCmd.AddCommand(newQRCommand(qrCmdDeps{}))
}
