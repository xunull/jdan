package cli

import (
	"fmt"
	"io"
	"os"

	"github.com/spf13/cobra"

	"github.com/xunull/jdan/internal/pemx"
)

type pemCmdDeps struct {
	out io.Writer
	in  io.Reader
}

func newPemCommand(deps pemCmdDeps) *cobra.Command {
	if deps.out == nil {
		deps.out = os.Stdout
	}
	if deps.in == nil {
		deps.in = os.Stdin
	}
	cmd := &cobra.Command{
		Use:   "pem [file]",
		Short: "检视 PEM 文件（证书/CSR/私钥/公钥…离线，不打印私钥）",
		Long: `离线检视一个 PEM 文件：把每个 PEM 块拆出来、认出类型、给摘要。
不联网、绝不打印私钥内容（私钥块只给类型 + 位数）。0 新依赖。

跟 ssl cert（联网抓 host 证书）/ cert（生成自签证书）互补：pem 读本地文件。
正好 1 个叶子证书 + 1 个私钥时，还会比对公钥告诉你 key 跟 cert 是否匹配。

例：
  jdan pem fullchain.pem
  jdan pem server.key
  cat cert.pem | jdan pem
  jdan pem bundle.pem --json`,
		Args:          cobra.MaximumNArgs(1),
		SilenceUsage:  true,
		SilenceErrors: true,
		RunE: func(cmd *cobra.Command, args []string) error {
			var (
				data []byte
				err  error
			)
			if len(args) == 1 {
				data, err = os.ReadFile(args[0])
			} else {
				data, err = io.ReadAll(deps.in)
			}
			if err != nil {
				return err
			}

			res, err := pemx.Inspect(data)
			if err != nil {
				return err
			}

			asJSON, _ := cmd.Flags().GetBool("json")
			if asJSON {
				s, err := res.FormatJSON()
				if err != nil {
					return err
				}
				fmt.Fprintln(deps.out, s)
			} else {
				fmt.Fprint(deps.out, res.FormatText())
			}
			return nil
		},
	}
	cmd.Flags().Bool("json", false, "结构化输出")
	return cmd
}

func init() {
	rootCmd.AddCommand(newPemCommand(pemCmdDeps{}))
}
