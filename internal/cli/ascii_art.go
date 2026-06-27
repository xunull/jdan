package cli

import (
	"fmt"
	"image"
	_ "image/gif"  // 注册 GIF 解码器
	_ "image/jpeg" // 注册 JPEG 解码器
	_ "image/png"  // 注册 PNG 解码器
	"io"
	"os"
	"strings"

	"github.com/spf13/cobra"

	"github.com/xunull/jdan/internal/asciiart"
)

type asciiArtCmdDeps struct {
	out    io.Writer
	errOut io.Writer
	in     io.Reader
}

func newAsciiArtCommand(deps asciiArtCmdDeps) *cobra.Command {
	if deps.out == nil {
		deps.out = os.Stdout
	}
	if deps.errOut == nil {
		deps.errOut = os.Stderr
	}
	if deps.in == nil {
		deps.in = os.Stdin
	}
	cmd := &cobra.Command{
		Use:   "ascii-art [image]",
		Short: "图片 → ASCII 字符画（PNG/JPEG/GIF）",
		Long: `把图片渲染成 ASCII 字符画。复用 stdlib 图片解码，0 新依赖。

例：
  jdan ascii-art logo.png            按终端宽度自动缩放
  jdan ascii-art photo.jpg -w 60     指定列宽
  jdan ascii-art logo.png --color    24-bit 真彩（仅 TTY）
  jdan ascii-art logo.png --invert   反明暗（浅底终端）
  cat x.png | jdan ascii-art         stdin

ramp（暗→亮）：standard（默认 10 级）/ detailed（70 级）/ blocks（░▒▓█）/ 或自定义串。
格式 PNG/JPEG/GIF（GIF 取第一帧）；WebP/HEIC 不支持（需新依赖）。`,
		Args:          cobra.MaximumNArgs(1),
		SilenceUsage:  true,
		SilenceErrors: true,
		RunE: func(cmd *cobra.Command, args []string) error {
			width, _ := cmd.Flags().GetInt("width")
			rampFlag, _ := cmd.Flags().GetString("ramp")
			invert, _ := cmd.Flags().GetBool("invert")
			colorFlag, _ := cmd.Flags().GetBool("color")
			charAspect, _ := cmd.Flags().GetFloat64("char-aspect")

			// 取输入
			var reader io.Reader = deps.in
			if len(args) == 1 {
				f, err := os.Open(args[0])
				if err != nil {
					return fmt.Errorf("无法打开 %q：%w", args[0], err)
				}
				defer f.Close()
				reader = f
			}

			img, format, err := image.Decode(reader)
			if err != nil {
				return fmt.Errorf("无法解码图片（支持 PNG/JPEG/GIF）：%w", err)
			}
			_ = format

			ramp := asciiart.ResolveRamp(rampFlag)
			if containsBlockChars(ramp) {
				fmt.Fprintln(deps.errOut, "提示：blocks ramp 的方块字符在中文终端可能按 2 列宽渲染，字符画会横向拉伸。")
			}

			// 列宽：未指定时按终端宽度，拿不到则 80
			if width <= 0 {
				width = termWidth(deps.out)
			}

			out := asciiart.Render(img, asciiart.Options{
				Width:      width,
				Ramp:       ramp,
				Invert:     invert,
				Color:      colorFlag && isTTY(deps.out),
				CharAspect: charAspect,
			})
			fmt.Fprint(deps.out, out)
			return nil
		},
	}
	cmd.Flags().IntP("width", "w", 0, "输出列宽（默认按终端宽度，拿不到则 80）")
	cmd.Flags().String("ramp", "standard", "字符 ramp：standard/detailed/blocks/<自定义串>")
	cmd.Flags().Bool("invert", false, "反明暗（浅底终端 / 打印）")
	cmd.Flags().Bool("color", false, "每字符 24-bit 真彩（仅 TTY；管道自动剥离）")
	cmd.Flags().Float64("char-aspect", 0.5, "字符高/宽比（默认 0.5，纠正纵向拉伸）")
	return cmd
}

func containsBlockChars(s string) bool {
	return strings.ContainsAny(s, "░▒▓█")
}

func init() {
	rootCmd.AddCommand(newAsciiArtCommand(asciiArtCmdDeps{}))
}
