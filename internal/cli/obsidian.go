package cli

import (
	"fmt"
	"os"
	"strings"

	"github.com/spf13/cobra"

	"github.com/xunull/jdan/internal/obsidian"
)

var obsidianCmd = &cobra.Command{
	Use:   "obsidian",
	Short: "Obsidian 相关子命令",
}

var obsidianInstallClaudianCmd = &cobra.Command{
	Use:   "install-claudian [vault-path]",
	Short: "从 GitHub 最新 Release 安装 Claudian 插件到 Obsidian Vault",
	Args:  cobra.MaximumNArgs(1),
	RunE: func(cmd *cobra.Command, args []string) error {
		force, _ := cmd.Flags().GetBool("force")

		var vaultPath string
		if len(args) > 0 {
			vaultPath = expandHome(args[0])
		} else {
			cwd, err := os.Getwd()
			if err != nil {
				return fmt.Errorf("无法获取当前目录: %w", err)
			}
			vaultPath = cwd
		}

		return obsidian.NewInstaller().Install(vaultPath, force)
	},
}

// expandHome replaces a leading ~ with the user's home directory.
func expandHome(path string) string {
	if path == "~" || strings.HasPrefix(path, "~/") {
		if home, err := os.UserHomeDir(); err == nil {
			path = home + path[1:]
		}
	}
	return path
}

func init() {
	obsidianInstallClaudianCmd.Flags().BoolP("force", "f", false, "覆盖已安装的插件")
	obsidianCmd.AddCommand(obsidianInstallClaudianCmd)
	rootCmd.AddCommand(obsidianCmd)
}
