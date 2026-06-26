package cli

import (
	"fmt"
	"io"
	"os"

	"github.com/spf13/cobra"

	"github.com/xunull/jdan/internal/entropyx"
)

type entropyCmdDeps struct {
	out io.Writer
	in  io.Reader
}

func newEntropyCommand(deps entropyCmdDeps) *cobra.Command {
	if deps.out == nil {
		deps.out = os.Stdout
	}
	if deps.in == nil {
		deps.in = os.Stdin
	}
	cmd := &cobra.Command{
		Use:   "entropy [string]",
		Short: "算输入的 Shannon 熵（bits/byte），判断是否加密/压缩/随机",
		Long: `算一段字符串/文件/stdin 的 Shannon 熵（字节分布的信息量，0–8 bits/byte）。
高（≥7.5）≈ 加密/压缩/随机，低 ≈ 重复/结构化文本。0 新依赖（纯 stdlib math）。

输入：位置参数=字符串 / -f 文件 / 无参=stdin。

例：
  jdan entropy "hello world"
  head -c 4096 /dev/urandom | jdan entropy
  jdan entropy -f firmware.bin --window 512   # 滑窗 sparkline 找高熵区段
  jdan entropy "Tr0ub4dour" --charset          # 附加搜索空间 bits 估算
  jdan entropy -f data.bin --json`,
		Args:          cobra.MaximumNArgs(1),
		SilenceUsage:  true,
		SilenceErrors: true,
		RunE: func(cmd *cobra.Command, args []string) error {
			file, _ := cmd.Flags().GetString("file")
			window, _ := cmd.Flags().GetInt("window")
			showCharset, _ := cmd.Flags().GetBool("charset")
			asJSON, _ := cmd.Flags().GetBool("json")

			var (
				data []byte
				err  error
			)
			switch {
			case file != "":
				data, err = os.ReadFile(file)
			case len(args) == 1:
				data = []byte(args[0])
			default:
				data, err = io.ReadAll(deps.in)
			}
			if err != nil {
				return err
			}
			if len(data) == 0 {
				return fmt.Errorf("没有输入（给个字符串、-f 文件或管道进来）")
			}

			res := entropyx.Analyze(data, window)
			if showCharset {
				res.Charset, res.CharsetBits = entropyx.CharsetBits(string(data))
			}

			if asJSON {
				s, err := res.FormatJSON()
				if err != nil {
					return err
				}
				fmt.Fprintln(deps.out, s)
			} else {
				fmt.Fprint(deps.out, res.FormatText(showCharset))
			}
			return nil
		},
	}
	cmd.Flags().StringP("file", "f", "", "量文件而非位置参数字符串")
	cmd.Flags().Int("window", 0, "滑窗字节数，开启逐块 sparkline")
	cmd.Flags().Bool("charset", false, "附加搜索空间 bits 估算（非强度评分）")
	cmd.Flags().Bool("json", false, "结构化输出")
	return cmd
}

func init() {
	rootCmd.AddCommand(newEntropyCommand(entropyCmdDeps{}))
}
