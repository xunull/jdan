package cli

import (
	"encoding/json"
	"fmt"
	"io"
	"os"

	"github.com/spf13/cobra"

	"github.com/xunull/jdan/internal/gitx"
)

type gitSummaryDeps struct {
	out io.Writer
	run gitx.Runner
}

func newGitSummaryCommand(deps gitSummaryDeps) *cobra.Command {
	if deps.out == nil {
		deps.out = os.Stdout
	}
	if deps.run == nil {
		deps.run = gitx.ExecRunner
	}
	cmd := &cobra.Command{
		Use:   "summary [path]",
		Short: "仓库一眼看：commit/分支/tag/年龄/贡献者/hotspots",
		Long: `仓库一眼看：总 commit 数、分支、tag、年龄、贡献者榜、改动最多的文件。
纯只读，底层调 git。

例：
  jdan git summary             # 当前目录
  jdan git summary /path/repo  # 指定仓库
  jdan git summary --top 10    # 各榜显示 10 条
  jdan git summary --json      # 结构化输出`,
		Args:          cobra.MaximumNArgs(1),
		SilenceUsage:  true,
		SilenceErrors: true,
		RunE: func(cmd *cobra.Command, args []string) error {
			dir := "."
			if len(args) == 1 {
				dir = args[0]
			}
			top, _ := cmd.Flags().GetInt("top")
			asJSON, _ := cmd.Flags().GetBool("json")

			s, err := gitx.Summarize(deps.run, dir, top)
			if err != nil {
				return err
			}
			if asJSON {
				return writeGitSummaryJSON(deps.out, s)
			}
			writeGitSummaryText(deps.out, s)
			return nil
		},
	}
	cmd.Flags().Int("top", 5, "贡献者 / hotspots 各显示几条")
	cmd.Flags().Bool("json", false, "JSON 输出")
	return cmd
}

func writeGitSummaryJSON(out io.Writer, s gitx.Summary) error {
	data, err := json.MarshalIndent(s, "", "  ")
	if err != nil {
		return err
	}
	fmt.Fprintln(out, string(data))
	return nil
}

func writeGitSummaryText(out io.Writer, s gitx.Summary) {
	fmt.Fprintf(out, "仓库: %s\n", s.Repo)
	fmt.Fprintf(out, "commit: %d   分支: %d   tag: %d\n", s.Commits, s.Branches, s.Tags)
	if s.FirstCommit != "" {
		fmt.Fprintf(out, "年龄: %s 起 (%s)\n", s.FirstCommit, s.Age)
	}

	if len(s.Contributors) > 0 {
		fmt.Fprintf(out, "\n贡献者 Top %d:\n", len(s.Contributors))
		w := 0
		for _, c := range s.Contributors {
			if len(c.Name) > w {
				w = len(c.Name)
			}
		}
		for _, c := range s.Contributors {
			fmt.Fprintf(out, "  %-*s  %d (%.1f%%)\n", w, c.Name, c.Commits, c.Percent)
		}
	}

	if len(s.Hotspots) > 0 {
		fmt.Fprintf(out, "\n改动最多的文件 (hotspots) Top %d:\n", len(s.Hotspots))
		w := 0
		for _, h := range s.Hotspots {
			if len(h.Path) > w {
				w = len(h.Path)
			}
		}
		for _, h := range s.Hotspots {
			fmt.Fprintf(out, "  %-*s  %d 次\n", w, h.Path, h.Changes)
		}
	}
}
