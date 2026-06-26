package cli

import (
	"fmt"
	"io"
	"os"

	"github.com/spf13/cobra"

	"github.com/xunull/jdan/internal/gitx"
)

type gitChangelogDeps struct {
	out io.Writer
	run gitx.Runner
}

func newGitChangelogCommand(deps gitChangelogDeps) *cobra.Command {
	if deps.out == nil {
		deps.out = os.Stdout
	}
	if deps.run == nil {
		deps.run = gitx.ExecRunner
	}
	cmd := &cobra.Command{
		Use:   "changelog",
		Short: "从最近 tag 到 HEAD 生成 changelog（Conventional Commits 分组）",
		Long: `从 from..to 范围的提交生成 changelog，按 Conventional Commits 分组
（feat→Features / fix→Bug Fixes / …，breaking 单独拎出）。默认输出 markdown。

from 默认最近的 tag（无 tag 则全部历史）；to 默认 HEAD。底层调 git，纯只读。

例：
  jdan git changelog                       # 最近 tag → HEAD
  jdan git changelog --from v0.4.0 --to v0.5.0
  jdan git changelog > RELEASE.md
  jdan git changelog --json`,
		Args:          cobra.NoArgs,
		SilenceUsage:  true,
		SilenceErrors: true,
		RunE: func(cmd *cobra.Command, args []string) error {
			from, _ := cmd.Flags().GetString("from")
			to, _ := cmd.Flags().GetString("to")
			asJSON, _ := cmd.Flags().GetBool("json")

			cl, err := gitx.BuildChangelog(deps.run, ".", from, to)
			if err != nil {
				return err
			}
			if asJSON {
				s, err := cl.JSON()
				if err != nil {
					return err
				}
				fmt.Fprintln(deps.out, s)
				return nil
			}
			fmt.Fprint(deps.out, cl.Markdown())
			return nil
		},
	}
	cmd.Flags().String("from", "", "起点 ref（默认最近 tag；无 tag 则全部历史）")
	cmd.Flags().String("to", "HEAD", "终点 ref")
	cmd.Flags().Bool("json", false, "JSON 输出")
	return cmd
}
