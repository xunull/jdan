package cli

import "github.com/spf13/cobra"

// sslCmd 是 jdan ssl 子命令的命名空间。
var sslCmd = &cobra.Command{
	Use:   "ssl",
	Short: "TLS / SSL 证书工具",
	Long: `TLS / SSL 证书相关子命令。

  jdan ssl cert <host>          看一个 host 的 TLS 证书详情（chain + verification + OCSP）

例：
  jdan ssl cert github.com
  jdan ssl cert example.com:8443
  jdan ssl cert -f cert.pem
  jdan ssl cert github.com --expires-in 30d   # 用于监控脚本`,
}

func init() {
	rootCmd.AddCommand(sslCmd)
}
