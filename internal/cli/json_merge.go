package cli

import (
	"fmt"
	"io"
	"os"

	"github.com/spf13/cobra"

	"github.com/xunull/jdan/internal/jsonx"
)

func newJSONMergeCommand(deps jsonCmdDeps) *cobra.Command {
	cmd := &cobra.Command{
		Use:   "merge <file> <file> [file...]",
		Short: "深度合并多个 JSON（后者覆盖前者）",
		Long: `深度合并多个 JSON 文档，从左到右依次合（后者覆盖前者）。
对象递归合并、数组按 --arrays 策略、标量/类型不一致后者覆盖。0 新依赖。

例：
  jdan json merge base.json override.json
  jdan json merge a.json b.json --arrays append
  jdan json merge *.json -p
  cat base.json | jdan json merge - override.json   # - = stdin`,
		Args:          cobra.MinimumNArgs(2),
		SilenceUsage:  true,
		SilenceErrors: true,
		RunE: func(cmd *cobra.Command, args []string) error {
			arraysFlag, _ := cmd.Flags().GetString("arrays")
			strat, err := jsonx.ParseArrayStrategy(arraysFlag)
			if err != nil {
				return err
			}
			indent := indentFromPretty(cmd)

			docs, err := readMergeInputs(args, deps.in)
			if err != nil {
				return err
			}
			out, err := jsonx.MergeAll(docs, strat, indent)
			if err != nil {
				return err
			}
			fmt.Fprintln(deps.out, string(out))
			return nil
		},
	}
	cmd.Flags().String("arrays", "replace", "数组合并策略：replace（后者替换）/ append（拼接）")
	cmd.Flags().BoolP("pretty", "p", false, "缩进输出（默认 compact 一行）")
	return cmd
}

// readMergeInputs 读各输入，`-` 代表 stdin（最多一次）。
func readMergeInputs(args []string, stdin io.Reader) ([][]byte, error) {
	docs := make([][]byte, len(args))
	usedStdin := false
	for i, a := range args {
		if a == "-" {
			if usedStdin {
				return nil, fmt.Errorf("- (stdin) 只能用一次")
			}
			usedStdin = true
			d, err := io.ReadAll(stdin)
			if err != nil {
				return nil, err
			}
			docs[i] = d
			continue
		}
		d, err := os.ReadFile(a)
		if err != nil {
			return nil, err
		}
		docs[i] = d
	}
	return docs, nil
}
