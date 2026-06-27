package cli

import (
	"github.com/spf13/cobra"

	"github.com/xunull/jdan/internal/dnslookup"
)

// 命令名 + flag 名的补全是 cobra 自带的（jdan completion <shell>）。这里给一批「值有
// 固定候选」的枚举 flag / 位置参数补上「值补全」，让 <Tab> 也能补出 sha256 / blocks 等。
// 集中在此一处，命令文件不动；在 Execute() 里调用（彼时所有 init() 已把子命令装齐）。

var httpMethods = []string{"GET", "HEAD", "POST", "PUT", "DELETE", "PATCH", "OPTIONS"}

// 常见 DNS 记录类型（dns lookup/trace --type）。
var dnsRecordTypes = []string{"A", "AAAA", "CNAME", "MX", "NS", "TXT", "SOA", "PTR", "SRV", "CAA", "all"}

func registerCompletions() {
	reg := func(path []string, flag string, choices ...string) {
		c, _, err := rootCmd.Find(path)
		if err != nil || c == nil {
			return
		}
		_ = c.RegisterFlagCompletionFunc(flag, cobra.FixedCompletions(choices, cobra.ShellCompDirectiveNoFileComp))
	}

	reg([]string{"hash"}, "algo", "md5", "sha1", "sha256", "sha512")
	reg([]string{"totp"}, "algo", "sha1", "sha256", "sha512")
	reg([]string{"qr"}, "ecc", "L", "M", "Q", "H")
	reg([]string{"figlet"}, "font", "standard", "block")
	reg([]string{"cert"}, "key-type", "ec", "rsa", "ed25519")
	reg([]string{"ascii-art"}, "ramp", "standard", "detailed", "blocks")
	reg([]string{"ssl", "pin"}, "format", "okhttp", "ios", "hpkp", "nss", "curl", "raw")
	reg([]string{"http", "headers"}, "method", httpMethods...)
	reg([]string{"net", "probe"}, "method", httpMethods...)
	reg([]string{"json", "merge"}, "arrays", "replace", "append")
	reg([]string{"dns", "lookup"}, "type", dnsRecordTypes...)
	reg([]string{"dns", "trace"}, "type", dnsRecordTypes...)

	doh := dnslookup.ProviderAliases()
	for _, p := range [][]string{{"dns", "lookup"}, {"dns", "trace"}, {"dns", "reverse"}, {"ping"}} {
		reg(p, "doh", doh...)
	}

	// cal 的位置参数补月份（数字 + 英文月名缩写）
	if c, _, err := rootCmd.Find([]string{"cal"}); err == nil && c != nil {
		c.ValidArgsFunction = monthComplete
	}
}

func monthComplete(_ *cobra.Command, args []string, _ string) ([]string, cobra.ShellCompDirective) {
	if len(args) > 0 { // 第二个参数是年份，不补
		return nil, cobra.ShellCompDirectiveNoFileComp
	}
	return []string{
		"1", "2", "3", "4", "5", "6", "7", "8", "9", "10", "11", "12",
		"Jan", "Feb", "Mar", "Apr", "May", "Jun", "Jul", "Aug", "Sep", "Oct", "Nov", "Dec",
	}, cobra.ShellCompDirectiveNoFileComp
}
