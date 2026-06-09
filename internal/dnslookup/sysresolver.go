package dnslookup

import (
	"net"

	"github.com/miekg/dns"
)

const (
	fallbackServer        = "8.8.8.8:53"
	defaultResolvConfPath = "/etc/resolv.conf"
)

// DetectSystemServer 返回当前 OS 配置的首选 DNS server（含端口）。
//
// macOS / Linux 上读 /etc/resolv.conf。文件不存在、无 nameserver 行、或解析失败时
// fallback 到 8.8.8.8:53；Windows 当前也走 fallback（plan D9：Windows not in scope）。
func DetectSystemServer() string {
	return detectFromFile(defaultResolvConfPath)
}

func detectFromFile(path string) string {
	cfg, err := dns.ClientConfigFromFile(path)
	if err != nil || cfg == nil || len(cfg.Servers) == 0 {
		return fallbackServer
	}
	port := cfg.Port
	if port == "" {
		port = "53"
	}
	host := cfg.Servers[0]
	// IPv6 字面量需用 brackets 包裹，否则 "host:port" 形式与 IPv6 address 自身的冒号
	// 在 net.SplitHostPort 下产生歧义。
	if ip := net.ParseIP(host); ip != nil && ip.To4() == nil {
		return "[" + host + "]:" + port
	}
	return host + ":" + port
}
