package cli

import (
	"fmt"
	"io"
	"os"

	"github.com/spf13/cobra"

	"github.com/xunull/jdan/internal/toc"
)

type tocCmdDeps struct {
	out io.Writer
	in  io.Reader
}

func newTocCommand(deps tocCmdDeps) *cobra.Command {
	if deps.out == nil {
		deps.out = os.Stdout
	}
	if deps.in == nil {
		deps.in = os.Stdin
	}
	cmd := &cobra.Command{
		Use:   "toc [file]",
		Short: "从 Markdown 标题生成目录（GitHub 风格 anchor）",
		Long: `从 Markdown 标题生成目录（TOC），anchor 跟 GitHub 渲染规则一致。
0 新依赖（纯 stdlib）。

例：
  jdan toc README.md                 # 输出 TOC 到 stdout
  jdan toc README.md --min 2 --max 3 # 只要某几级
  jdan toc README.md --inplace       # 回填到文件
  cat README.md | jdan toc           # stdin

--inplace 在文件的 <!-- toc --> 和 <!-- /toc --> 标记之间回填（缺标记报错）。`,
		Args:          cobra.MaximumNArgs(1),
		SilenceUsage:  true,
		SilenceErrors: true,
		RunE: func(cmd *cobra.Command, args []string) error {
			min, _ := cmd.Flags().GetInt("min")
			max, _ := cmd.Flags().GetInt("max")
			inplace, _ := cmd.Flags().GetBool("inplace")
			if min < 1 || max > 6 || min > max {
				return fmt.Errorf("级别范围非法：--min %d --max %d（须 1<=min<=max<=6）", min, max)
			}

			var path string
			if len(args) == 1 {
				path = args[0]
			}
			if inplace && path == "" {
				return fmt.Errorf("--inplace 需要一个文件参数（不能用于 stdin）")
			}

			var content string
			if path != "" {
				data, err := os.ReadFile(path)
				if err != nil {
					return err
				}
				content = string(data)
			} else {
				data, err := io.ReadAll(deps.in)
				if err != nil {
					return err
				}
				content = string(data)
			}

			tocText := toc.Render(toc.ParseHeadings(content), min, max)

			if inplace {
				updated, err := toc.Insert(content, tocText)
				if err != nil {
					return err
				}
				return os.WriteFile(path, []byte(updated), 0o644)
			}

			if tocText != "" {
				fmt.Fprintln(deps.out, tocText)
			}
			return nil
		},
	}
	cmd.Flags().Int("min", 2, "最小标题级别（默认从 h2 起，跳过文档大标题）")
	cmd.Flags().Int("max", 6, "最大标题级别")
	cmd.Flags().Bool("inplace", false, "回填到文件的 <!-- toc --> 标记之间")
	return cmd
}

func init() {
	rootCmd.AddCommand(newTocCommand(tocCmdDeps{}))
}
