package cli

import (
	"fmt"
	"os"
	"strings"

	"github.com/spf13/cobra"
	"github.com/spf13/viper"
)

// Execute 运行根命令，并负责把错误打印出来。main 只负责退出码。
//
// 打印责任必须收在一个地方。之前是两处各打一遍：cobra 按 SilenceErrors
// 打一次，main 再用 zerolog 打一次，于是 `jdan sizz` 会先看到有用的
// 「Did you mean this? size」，紧接着又来一行把同样内容转义成机器格式的
// FTL。而 56 个设了 SilenceErrors 的子命令则相反 —— cobra 不打，全靠 main
// 那行 FTL，错误还写在 stdout 上。
func Execute() error {
	registerCompletions() // 给枚举 flag / 位置参数挂值补全（命令名/flag 名补全 cobra 自带）

	cmd, err := rootCmd.ExecuteC()
	if err == nil {
		return nil
	}

	// err 本身已经带了 cobra 的建议（「unknown command "sizz" … Did you mean
	// this? size」整段都在 err.Error() 里），直接打出来即可。
	fmt.Fprintln(os.Stderr, "Error:", err)
	if isUsageError(err) {
		// cmd 是真正出错的那一级，所以 flag 写错时提示的是
		// `jdan size --help` 而不是 `jdan --help`。
		fmt.Fprintf(os.Stderr, "Run '%s --help' for usage.\n", cmd.CommandPath())
	}
	return err
}

// usageErrorPrefixes 是 cobra 在「命令行写错了」时产生的错误消息前缀。
//
// 只对这类错误提示 --help：用户把命令名或 flag 敲错了，--help 能帮上忙。
// 运行时错误（文件不存在、网络超时、权限不足）提示 --help 纯属噪音 ——
// 这也正是那 56 个子命令设 SilenceUsage 的原因。
//
// cobra 没有为这类错误导出哨兵值，只能按消息格式匹配。TestIsUsageError
// 用真实的 cobra 命令跑出每一种错误再断言分类，cobra 改了措辞会立刻失败。
var usageErrorPrefixes = []string{
	"unknown command",
	"unknown flag:",
	"unknown shorthand flag:",
	"flag needs an argument:",
	"invalid argument",
	"accepts ",
	"requires at least",
	"unknown help topic",
}

func isUsageError(err error) bool {
	msg := err.Error()
	for _, p := range usageErrorPrefixes {
		if strings.HasPrefix(msg, p) {
			return true
		}
	}
	return false
}

var cfgFile string

var rootCmd = &cobra.Command{
	Use:   "jdan",
	Short: "常用小工具集合",
	// 错误由 Execute 统一打印，不让 cobra 自己打（否则和 Execute 里那次重复）。
	SilenceErrors: true,
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
