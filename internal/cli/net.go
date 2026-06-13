package cli

import "github.com/spf13/cobra"

// netCmd 是 jdan net 子命令的命名空间。具体子命令在各自文件里 AddCommand 进来。
var netCmd = &cobra.Command{
	Use:   "net",
	Short: "网络探查 & 排查工具",
	Long: `网络层面的诊断工具集合。

  jdan net probe <target>       从客户端视角探查目标，逐阶段（DNS/TCP/TLS/HTTP）报告
  jdan net selfcheck [:port]    服务端自检，回答"我作为 server 该不该被外部访问"`,
}

func init() {
	rootCmd.AddCommand(netCmd)
}
