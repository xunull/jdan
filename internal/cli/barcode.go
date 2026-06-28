package cli

import (
	"bytes"
	"errors"
	"fmt"
	"image"
	"image/color"
	"image/png"
	"io"
	"os"
	"path/filepath"
	"strings"

	"github.com/spf13/cobra"

	"github.com/xunull/jdan/internal/barcode"
)

type barcodeCmdDeps struct {
	out io.Writer
	in  io.Reader
}

func newBarcodeCommand(deps barcodeCmdDeps) *cobra.Command {
	if deps.out == nil {
		deps.out = os.Stdout
	}
	if deps.in == nil {
		deps.in = os.Stdin
	}
	cmd := &cobra.Command{
		Use:   "barcode [data]",
		Short: "生成 Code128 一维条码（终端 / PNG / SVG）",
		Long: `生成 Code128 一维条码（库存 / 物流 / 快递单常用）。内嵌 Code128 模式表自己编码、
自己渲染，0 新依赖。字符集默认 B（可打印 ASCII）；输入全数字且偶数长度时自动用 C（更窄）。

例：
  jdan barcode "ABC-123"           # 终端
  jdan barcode 5901234123457 -o label.png
  jdan barcode "SKU42" -o tag.svg
  echo "data" | jdan barcode       # stdin
  jdan barcode "ABC-123" --json    # {data, code_set, checksum, modules, ...}`,
		Args:          cobra.MaximumNArgs(1),
		SilenceUsage:  true,
		SilenceErrors: true,
		RunE: func(cmd *cobra.Command, args []string) error {
			data, err := readQRData(deps.in, args) // 复用 qr 的「arg 或 stdin」读取
			if err != nil {
				return err
			}
			sym, err := barcode.Encode(data)
			if err != nil {
				return err
			}

			asJSON, _ := cmd.Flags().GetBool("json")
			moduleW, _ := cmd.Flags().GetInt("module")
			height, _ := cmd.Flags().GetInt("height")
			invert, _ := cmd.Flags().GetBool("invert")
			noText, _ := cmd.Flags().GetBool("no-text")
			output, _ := cmd.Flags().GetString("output")
			if moduleW < 1 {
				moduleW = 1
			}

			if asJSON {
				return writeIndentJSON(deps.out, map[string]any{
					"data":     sym.Data,
					"code_set": sym.CodeSet,
					"checksum": sym.Checksum,
					"modules":  sym.Width(),
				})
			}
			if output != "" {
				return writeBarcodeFile(output, sym, moduleW, height, invert, !noText)
			}
			emitBarcodeTerminal(deps.out, sym, moduleW, invert, !noText)
			return nil
		},
	}
	cmd.Flags().StringP("output", "o", "", "写入文件，按扩展名识别 .png/.svg")
	cmd.Flags().IntP("module", "m", 0, "模块宽（PNG 像素 / SVG 单位 / 终端列数；默认 PNG=2 其余=1）")
	cmd.Flags().Int("height", 0, "条高（PNG/SVG 像素，默认 80；终端固定几行）")
	cmd.Flags().Bool("invert", false, "反色")
	cmd.Flags().Bool("no-text", false, "不显示下方人眼可读文本")
	cmd.Flags().Bool("json", false, "输出元信息 JSON")
	return cmd
}

const barcodeTerminalRows = 3 // 终端条码高度（行）

func emitBarcodeTerminal(out io.Writer, sym barcode.Symbol, moduleW int, invert, text bool) {
	on, off := "█", " "
	if invert {
		on, off = " ", "█"
	}
	var line strings.Builder
	for _, m := range sym.Modules {
		cell := off
		if m {
			cell = on
		}
		line.WriteString(strings.Repeat(cell, moduleW))
	}
	row := line.String()
	for range barcodeTerminalRows {
		fmt.Fprintln(out, row)
	}
	if text {
		fmt.Fprintln(out, centerASCII(sym.Data, sym.Width()*moduleW))
	}
}

func writeBarcodeFile(path string, sym barcode.Symbol, moduleW, height int, invert, text bool) error {
	switch strings.ToLower(filepath.Ext(path)) {
	case ".png":
		if moduleW == 1 { // PNG 默认更粗，1px 模块太细扫不动
			moduleW = 2
		}
		if height <= 0 {
			height = 80
		}
		b, err := renderBarcodePNG(sym, moduleW, height, invert)
		if err != nil {
			return err
		}
		return os.WriteFile(path, b, 0o644)
	case ".svg":
		if height <= 0 {
			height = 80
		}
		s := renderBarcodeSVG(sym, moduleW, height, invert, text)
		return os.WriteFile(path, []byte(s), 0o644)
	default:
		return fmt.Errorf("不支持的扩展名 %q（用 .png 或 .svg）", filepath.Ext(path))
	}
}

func renderBarcodePNG(sym barcode.Symbol, moduleW, height int, invert bool) ([]byte, error) {
	w := sym.Width() * moduleW
	if w <= 0 || height <= 0 {
		return nil, errors.New("尺寸非法")
	}
	img := image.NewGray(image.Rect(0, 0, w, height))
	for i := range img.Pix {
		img.Pix[i] = 255 // 白底
	}
	black := color.Gray{Y: 0}
	for i, m := range sym.Modules {
		isBar := m
		if invert {
			isBar = !m
		}
		if !isBar {
			continue
		}
		for x := i * moduleW; x < (i+1)*moduleW; x++ {
			for y := 0; y < height; y++ {
				img.SetGray(x, y, black)
			}
		}
	}
	var buf bytes.Buffer
	if err := png.Encode(&buf, img); err != nil {
		return nil, err
	}
	return buf.Bytes(), nil
}

func renderBarcodeSVG(sym barcode.Symbol, moduleW, height int, invert, text bool) string {
	if moduleW < 1 {
		moduleW = 1
	}
	w := sym.Width() * moduleW
	textH := 0
	if text {
		textH = 18
	}
	var b strings.Builder
	fmt.Fprintf(&b, `<svg xmlns="http://www.w3.org/2000/svg" width="%d" height="%d" viewBox="0 0 %d %d">`,
		w, height+textH, w, height+textH)
	b.WriteString(`<rect width="100%" height="100%" fill="white"/>`)
	for i, m := range sym.Modules {
		isBar := m
		if invert {
			isBar = !m
		}
		if !isBar {
			continue
		}
		fmt.Fprintf(&b, `<rect x="%d" y="0" width="%d" height="%d" fill="black"/>`, i*moduleW, moduleW, height)
	}
	if text {
		fmt.Fprintf(&b, `<text x="%d" y="%d" font-family="monospace" font-size="14" text-anchor="middle" fill="black">%s</text>`,
			w/2, height+14, svgEscape(sym.Data))
	}
	b.WriteString("</svg>")
	return b.String()
}

// centerASCII 把 s 居中到宽度 w（s 为 ASCII，按字节长度）。
func centerASCII(s string, w int) string {
	if len(s) >= w {
		return s
	}
	left := (w - len(s)) / 2
	return strings.Repeat(" ", left) + s
}

func svgEscape(s string) string {
	r := strings.NewReplacer("&", "&amp;", "<", "&lt;", ">", "&gt;", `"`, "&quot;")
	return r.Replace(s)
}

func init() {
	rootCmd.AddCommand(newBarcodeCommand(barcodeCmdDeps{}))
}
