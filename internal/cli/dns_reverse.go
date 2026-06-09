package cli

import (
	"fmt"
	"net"
	"strings"
	"time"

	"github.com/miekg/dns"
	"github.com/spf13/cobra"

	"github.com/xunull/jdan/internal/dnslookup"
)

// newReverseCommand 构造 `jdan dns reverse <ip>` 子命令。
//
// 与 `dns lookup` 同款 flag 表面（共享 flag 都注册在这里），但**不**含 --type
// ——reverse 只查 PTR，type 写死。
func newReverseCommand(deps dnsCmdDeps) *cobra.Command {
	cmd := &cobra.Command{
		Use:   "reverse <ip>",
		Short: "把 IP 反向解析为域名（PTR 查询）",
		Long: `把 IP 反向解析为域名（PTR 查询）。

接受 IPv4 和 IPv6 字面量。自动转为 in-addr.arpa（v4）或 ip6.arpa（v6）域名后
查询 PTR 记录。

支持与 jdan dns lookup 完全相同的 flag：--server / --doh / --json / --short
/ --verbose / --strict / --timeout。--doh 别名（google / cloudflare / quad9
/ ...）依然走内置 IP 直连，劫持环境下也能拿到真实的 PTR。

输入只接受单一 IP；域名 / CIDR / host:port / link-local-with-zone-id 都会
被拒绝并提示正确用法。`,
		Args: cobra.ExactArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			return runDNSReverse(cmd, args, deps)
		},
	}

	cmd.Flags().StringP("server", "s", "", "DNS server (e.g. 8.8.8.8 或 8.8.8.8:53)；空表示读 /etc/resolv.conf")
	cmd.Flags().String("doh", "", "DoH endpoint：别名 ("+strings.Join(dnslookup.ProviderAliases(), "/")+")、主机名或完整 https:// URL；与 --server 互斥")
	cmd.Flags().BoolP("json", "j", false, "以 JSON 格式输出")
	cmd.Flags().Bool("short", false, "仅输出值，dig +short 风格")
	cmd.Flags().BoolP("verbose", "v", false, "输出 query time 等元数据，rcode 单独列")
	cmd.Flags().Bool("strict", false, "PTR 查询失败即 exit 1")
	cmd.Flags().Duration("timeout", 5*time.Second, "整体查询超时")

	return cmd
}

func runDNSReverse(cmd *cobra.Command, args []string, deps dnsCmdDeps) error {
	ip, err := parseReverseIP(args[0])
	if err != nil {
		return err
	}
	// dns.ReverseAddr 已处理 v4 → in-addr.arpa 和 v6 → ip6.arpa 两种格式。
	// 它只在输入非合法 IP 时返回错误——我们已通过 net.ParseIP 提前校验，这里
	// 仍 defensive check 一次。
	ptrDomain, err := dns.ReverseAddr(ip.String())
	if err != nil {
		return fmt.Errorf("无法构造 PTR 查询域名: %v", err)
	}
	// 把原始 IP 作为 DisplayName 传给 helper，让 formatter 顶部显示
	// "8.8.8.8 — via ..." 而不是 arpa 形式。
	return runDNSQuery(cmd, deps, ptrDomain, []uint16{dns.TypePTR}, ip.String())
}

// parseReverseIP 严格校验输入必须是单一 IP 字面量。按 design doc 拒绝清单：
//   - CIDR（含 "/"）→ 明确拒绝
//   - host:port（SplitHostPort 成功）→ 明确拒绝
//   - 域名（无 ":"、含 "." 且 ParseIP 失败）→ 提示用 jdan dns lookup
//   - 其他无法解析的形式（链路本地带 zone-id、纯垃圾）→ 通用"不是合法 IP"
//
// 0.0.0.0 / 127.0.0.1 / 私网 IP 等特殊但合法的 IP 不拦截，按 DNS 真相原则
// 透传查询（多数返回 NXDOMAIN）。
func parseReverseIP(input string) (net.IP, error) {
	input = strings.TrimSpace(input)
	if input == "" {
		return nil, fmt.Errorf("IP 不能为空")
	}
	if strings.Contains(input, "/") {
		return nil, fmt.Errorf("jdan dns reverse 不接受 CIDR: %q（请传单一 IP）", input)
	}
	if _, _, err := net.SplitHostPort(input); err == nil {
		return nil, fmt.Errorf("jdan dns reverse 不要传端口: %q（接受纯 IP，不含 :port）", input)
	}
	if ip := net.ParseIP(input); ip != nil {
		return ip, nil
	}
	// ParseIP 失败：根据形态给不同提示。
	// 没有冒号 + 含点 = 域名形态（如 google.com）→ 引导用户用 lookup
	if !strings.Contains(input, ":") && strings.Contains(input, ".") {
		return nil, fmt.Errorf("%q 不是合法 IP；反向查询请用 `jdan dns lookup`", input)
	}
	return nil, fmt.Errorf("%q 不是合法 IP", input)
}
