package cli

import (
	"encoding/json"
	"fmt"
	"io"
	"os"

	"github.com/spf13/cobra"

	"github.com/xunull/jdan/internal/fake"
)

type fakeCmdDeps struct {
	out io.Writer
}

func newFakeCommand(deps fakeCmdDeps) *cobra.Command {
	if deps.out == nil {
		deps.out = os.Stdout
	}
	cmd := &cobra.Command{
		Use:   "fake [type]",
		Short: "生成像真实数据的假值（测试 fixture / 填库 / 示例）",
		Long: `生成像真实数据的结构化假值。0 新依赖，内置词库。

类型：name email uuid sentence word int date ip
全部取自示例词库或 RFC 保留段，不对应真实个人/主机，仅供测试。

例：
  jdan fake name                 # 一个姓名
  jdan fake email -n 3           # 3 个邮箱
  jdan fake int --min 1 --max 6  # 骰子
  jdan fake name --seed 42 -n 2  # 可复现（同 seed 同输出）
  jdan fake uuid --json -n 5     # JSON 数组
  jdan fake --json -n 2          # 复合记录（name+email+age+ip）
  jdan fake --list               # 列出类型`,
		Args:          cobra.MaximumNArgs(1),
		SilenceUsage:  true,
		SilenceErrors: true,
		RunE: func(cmd *cobra.Command, args []string) error {
			if list, _ := cmd.Flags().GetBool("list"); list {
				for _, t := range fake.SupportedTypes {
					fmt.Fprintln(deps.out, t)
				}
				return nil
			}

			n, _ := cmd.Flags().GetInt("count")
			if n < 1 {
				return fmt.Errorf("--count 必须 >= 1")
			}
			asJSON, _ := cmd.Flags().GetBool("json")
			opts := fake.Options{}
			opts.Min, _ = cmd.Flags().GetInt("min")
			opts.Max, _ = cmd.Flags().GetInt("max")
			opts.Words, _ = cmd.Flags().GetInt("words")
			opts.DateFormat, _ = cmd.Flags().GetString("format")

			g, err := newFakeGenerator(cmd)
			if err != nil {
				return err
			}

			// 无 type + --json → 复合记录
			if len(args) == 0 {
				if !asJSON {
					return fmt.Errorf("需要一个类型参数，或用 --json 生成复合记录（--list 查看类型）")
				}
				people := make([]fake.Person, n)
				for i := range people {
					people[i] = g.Person()
				}
				return writeJSON(deps.out, people)
			}

			typ := args[0]
			vals := make([]string, n)
			for i := range vals {
				v, err := g.Value(typ, opts)
				if err != nil {
					return err
				}
				vals[i] = v
			}
			if asJSON {
				return writeJSON(deps.out, vals)
			}
			for _, v := range vals {
				fmt.Fprintln(deps.out, v)
			}
			return nil
		},
	}
	cmd.Flags().IntP("count", "n", 1, "生成几个")
	cmd.Flags().Int64("seed", 0, "可复现种子（不设则用真随机熵）")
	cmd.Flags().Bool("json", false, "JSON 数组（有 type）或复合记录（无 type）")
	cmd.Flags().Int("min", 0, "int 类型最小值（闭区间）")
	cmd.Flags().Int("max", 9999, "int 类型最大值（闭区间）")
	cmd.Flags().Int("words", 6, "sentence 类型词数")
	cmd.Flags().String("format", "2006-01-02", "date 类型格式（Go 参考时间布局）")
	cmd.Flags().Bool("list", false, "列出支持的类型")
	return cmd
}

// newFakeGenerator 根据 --seed 是否显式设置选择确定性或真随机生成器。
func newFakeGenerator(cmd *cobra.Command) (*fake.Generator, error) {
	if cmd.Flags().Changed("seed") {
		seed, _ := cmd.Flags().GetInt64("seed")
		return fake.New(seed), nil
	}
	return fake.NewRandom()
}

func writeJSON(out io.Writer, v any) error {
	data, err := json.MarshalIndent(v, "", "  ")
	if err != nil {
		return err
	}
	fmt.Fprintln(out, string(data))
	return nil
}

func init() {
	rootCmd.AddCommand(newFakeCommand(fakeCmdDeps{}))
}
