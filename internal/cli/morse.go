package cli

import (
	"encoding/json"
	"fmt"
	"io"
	"os"
	"strings"

	"github.com/spf13/cobra"

	"github.com/xunull/jdan/internal/morsex"
)

type morseCmdDeps struct {
	out    io.Writer
	errOut io.Writer
	in     io.Reader
}

func newMorseCommand(deps morseCmdDeps) *cobra.Command {
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
		Use:   "morse [text]",
		Short: "文本 ↔ 摩斯电码（ITU；自动判方向）",
		Long: `文本 ↔ 国际摩斯电码（ITU）互转。0 新依赖。

自动判方向：输入只含 . - / 空格 → 解码，否则编码。--encode / -d 可强制。
字母间单空格、单词间 " / "；大小写无关，解码输出大写。

例：
  jdan morse "SOS"            # ... --- ...
  jdan morse "... --- ..."    # SOS（自动解码）
  echo "Hello" | jdan morse
  jdan morse "E" --encode     # 极短输入破歧义
  jdan morse "SOS" --json`,
		Args:          cobra.MaximumNArgs(1),
		SilenceUsage:  true,
		SilenceErrors: true,
		RunE: func(cmd *cobra.Command, args []string) error {
			var raw string
			if len(args) == 1 {
				raw = args[0]
			} else {
				data, err := io.ReadAll(deps.in)
				if err != nil {
					return err
				}
				raw = string(data)
			}
			if strings.TrimSpace(raw) == "" {
				return fmt.Errorf("没有输入（给个文本/摩斯码或管道进来）")
			}

			forceEnc, _ := cmd.Flags().GetBool("encode")
			forceDec, _ := cmd.Flags().GetBool("decode")
			asJSON, _ := cmd.Flags().GetBool("json")

			var decode bool
			switch {
			case forceDec:
				decode = true
			case forceEnc:
				decode = false
			default:
				decode = morsex.LooksLikeMorse(raw)
			}

			var (
				output    string
				direction string
				noted     int
			)
			if decode {
				direction = "decode"
				output, noted = morsex.Decode(raw)
			} else {
				direction = "encode"
				output, noted = morsex.Encode(raw)
			}

			if asJSON {
				b, err := json.MarshalIndent(map[string]any{"direction": direction, "output": output}, "", "  ")
				if err != nil {
					return err
				}
				fmt.Fprintln(deps.out, string(b))
			} else {
				fmt.Fprintln(deps.out, output)
			}

			if noted > 0 {
				if decode {
					fmt.Fprintf(deps.errOut, "(%d 个无法识别的码 → #)\n", noted)
				} else {
					fmt.Fprintf(deps.errOut, "(跳过 %d 个无法编码的字符)\n", noted)
				}
			}
			return nil
		},
	}
	cmd.Flags().Bool("encode", false, "强制编码（文本 → 摩斯）")
	cmd.Flags().BoolP("decode", "d", false, "强制解码（摩斯 → 文本）")
	cmd.Flags().Bool("json", false, "结构化输出 {direction, output}")
	cmd.MarkFlagsMutuallyExclusive("encode", "decode")
	return cmd
}

func init() {
	rootCmd.AddCommand(newMorseCommand(morseCmdDeps{}))
}
