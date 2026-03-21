package cli

import (
	"errors"
	"time"

	"github.com/rs/zerolog/log"
	"github.com/spf13/cobra"

	"github.com/xunull/jdan/internal/filebak"
)
var fileCmd = &cobra.Command{
	Use:   "file",
	Short: "文件相关子命令",
}

var fileBakCmd = &cobra.Command{
	Use:   "bak [path]",
	Short: "将文件复制为同目录下的 .bak 备份",
	Args:  cobra.ExactArgs(1),
	RunE: func(cmd *cobra.Command, args []string) error {
		src := args[0]
		desc, _ := cmd.Flags().GetString("desc")
		err := filebak.BackupFile(src, desc, time.Now())
		if errors.Is(err, filebak.ErrInvalidDesc) {
			log.Warn().Msg("描述仅允许英文字母、汉字、数字与空格")
		}
		return err
	},
}
func init() {
	fileBakCmd.Flags().String("desc", "", "可选描述（仅字母、汉字、数字与空格；空格会变为下划线）")
	fileCmd.AddCommand(fileBakCmd)
	rootCmd.AddCommand(fileCmd)
}
