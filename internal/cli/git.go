package cli

import (
	"github.com/spf13/cobra"
)

// newGitCommand 是 jdan git 父命令，挂载各 git 子命令（summary…）。
func newGitCommand() *cobra.Command {
	cmd := &cobra.Command{
		Use:   "git",
		Short: "git 辅助工具（summary/changelog…）",
		Long:  "git 辅助工具。底层调用 git，0 新依赖（只要环境里有 git）。",
	}
	cmd.AddCommand(newGitSummaryCommand(gitSummaryDeps{}))
	cmd.AddCommand(newGitChangelogCommand(gitChangelogDeps{}))
	return cmd
}

func init() {
	rootCmd.AddCommand(newGitCommand())
}
