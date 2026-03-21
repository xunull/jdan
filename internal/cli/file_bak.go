package cli

import (
	"github.com/rs/zerolog/log"
	"github.com/spf13/cobra"
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
		log.Info().Str("src", src).Str("desc", desc).Msg("file bak (skeleton)")
		return nil
	},
}

func init() {
	fileBakCmd.Flags().String("desc", "", "可选描述（仅字母、汉字、数字与空格；空格会变为下划线）")
	fileCmd.AddCommand(fileBakCmd)
	rootCmd.AddCommand(fileCmd)
}
