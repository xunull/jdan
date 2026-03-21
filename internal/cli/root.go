package cli

import (
	"strings"

	"github.com/spf13/cobra"
	"github.com/spf13/viper"
)

// Execute runs the root command.
func Execute() error {
	return rootCmd.Execute()
}

var cfgFile string

var rootCmd = &cobra.Command{
	Use:   "jdan",
	Short: "常用小工具集合",
	PersistentPreRunE: func(cmd *cobra.Command, args []string) error {
		viper.SetEnvPrefix("JDAN")
		viper.AutomaticEnv()
		viper.SetEnvKeyReplacer(strings.NewReplacer(".", "_"))
		if cfgFile != "" {
			viper.SetConfigFile(cfgFile)
		} else {
			viper.SetConfigName("config")
			viper.AddConfigPath(".")
		}
		_ = viper.ReadInConfig()
		if err := viper.BindPFlags(cmd.PersistentFlags()); err != nil {
			return err
		}
		return viper.BindPFlags(cmd.Flags())
	},
}

func init() {
	rootCmd.PersistentFlags().StringVar(&cfgFile, "config", "", "配置文件路径（可选）")
}
