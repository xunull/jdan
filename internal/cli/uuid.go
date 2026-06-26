package cli

import (
	"crypto/rand"
	"fmt"
	"io"
	"os"
	"strings"

	"github.com/spf13/cobra"

	"github.com/xunull/jdan/internal/randgen"
	"github.com/xunull/jdan/internal/uuidx"
)

type uuidCmdDeps struct {
	out        io.Writer
	in         io.Reader
	randReader io.Reader
}

func newUUIDCommand(deps uuidCmdDeps) *cobra.Command {
	if deps.out == nil {
		deps.out = os.Stdout
	}
	if deps.in == nil {
		deps.in = os.Stdin
	}
	if deps.randReader == nil {
		deps.randReader = rand.Reader
	}

	cmd := &cobra.Command{
		Use:   "uuid [uuid]",
		Short: "检视 UUID（版本/variant/时间戳/字节）；生成见 jdan uuid new",
		Long: `检视一个 UUID：版本、variant、v1/v7 内嵌时间戳、字节、URN 形式、nil/max。
输入容错：urn:uuid: 前缀 / {花括号} / 无连字符 / 大小写都行。0 新依赖（纯 stdlib）。

生成走子命令 jdan uuid new（复用 jdan rand uuid 的实现，不重复造）。

例：
  jdan uuid 0190a1b2-c3d4-7e5f-8a9b-1c2d3e4f5a6b
  echo "$U" | jdan uuid
  jdan uuid "$U" --json
  jdan uuid new --v7 -n 3`,
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
				return fmt.Errorf("没有 UUID（给个参数或 stdin；生成用 jdan uuid new）")
			}
			info, err := uuidx.Parse(raw)
			if err != nil {
				return err
			}
			asJSON, _ := cmd.Flags().GetBool("json")
			if asJSON {
				s, err := info.FormatJSON()
				if err != nil {
					return err
				}
				fmt.Fprintln(deps.out, s)
			} else {
				fmt.Fprint(deps.out, info.FormatText())
			}
			return nil
		},
	}
	cmd.Flags().Bool("json", false, "结构化输出")
	cmd.AddCommand(newUUIDNewCommand(deps))
	return cmd
}

func newUUIDNewCommand(deps uuidCmdDeps) *cobra.Command {
	var (
		v7 bool
		n  int
	)
	cmd := &cobra.Command{
		Use:           "new",
		Short:         "生成 UUID（默认 v4；--v7 为时间排序的 v7）",
		Long:          "生成 UUID。复用 jdan rand uuid 的实现（internal/randgen，CSPRNG）。",
		Args:          cobra.NoArgs,
		SilenceUsage:  true,
		SilenceErrors: true,
		RunE: func(cmd *cobra.Command, args []string) error {
			if n < 1 {
				n = 1
			}
			for range n {
				var (
					u   string
					err error
				)
				if v7 {
					u, err = randgen.GenerateUUIDv7(deps.randReader)
				} else {
					u, err = randgen.GenerateUUIDv4(deps.randReader)
				}
				if err != nil {
					return err
				}
				fmt.Fprintln(deps.out, u)
			}
			return nil
		},
	}
	cmd.Flags().BoolVar(&v7, "v7", false, "生成 v7（时间排序）而非 v4")
	cmd.Flags().IntVarP(&n, "count", "n", 1, "生成数量")
	return cmd
}

func init() {
	rootCmd.AddCommand(newUUIDCommand(uuidCmdDeps{}))
}
