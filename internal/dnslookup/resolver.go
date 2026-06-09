// Package dnslookup 实现 jdan dns lookup 子命令的 DNS 查询、结果合并与格式化能力。
//
// 设计要点：
//   - Resolver 接口将查询能力与上层逻辑解耦，单元测试可注入 fakeResolver。
//   - 生产实现走 github.com/miekg/dns，可获取 TTL、rcode、flags 等完整 metadata。
//   - UDP 优先，响应被截断（TC 位）时自动切到 TCP 重试。
package dnslookup

import (
	"context"
	"net"
	"strings"
	"time"

	"github.com/miekg/dns"
)

// Resolver 抽象 DNS 查询能力。生产用 miekgResolver；测试用 mock。
type Resolver interface {
	Query(ctx context.Context, domain string, qtype uint16, server string) (*dns.Msg, error)
}

type miekgResolver struct {
	udpClient *dns.Client
	tcpClient *dns.Client
}

// NewResolver 构造一个走真实网络的 miekg/dns 解析器。timeout 同时作用于单个 type 查询。
func NewResolver(timeout time.Duration) Resolver {
	return &miekgResolver{
		udpClient: &dns.Client{Net: "udp", Timeout: timeout},
		tcpClient: &dns.Client{Net: "tcp", Timeout: timeout},
	}
}

func (r *miekgResolver) Query(ctx context.Context, domain string, qtype uint16, server string) (*dns.Msg, error) {
	msg := new(dns.Msg)
	msg.SetQuestion(dns.Fqdn(domain), qtype)
	msg.RecursionDesired = true

	addr := ensurePort(server)

	resp, _, err := r.udpClient.ExchangeContext(ctx, msg, addr)
	if err != nil {
		return nil, err
	}
	// TC 位表示 UDP 响应被截断，按 RFC 1035 切换 TCP 重试。
	if resp != nil && resp.Truncated {
		resp, _, err = r.tcpClient.ExchangeContext(ctx, msg, addr)
		if err != nil {
			return nil, err
		}
	}
	return resp, nil
}

// ensurePort 在缺省时补全 DNS 标准端口 53。
//
// 接受形式：空串、IPv4、IPv4:port、IPv6 字面量、[IPv6]:port、hostname、hostname:port。
// DoH URL（https:// 开头）原样返回——dohResolver 直接使用 URL，不需要 host:port 形式。
func ensurePort(server string) string {
	if server == "" {
		return "8.8.8.8:53"
	}
	if strings.HasPrefix(strings.ToLower(server), "https://") {
		return server
	}
	if _, _, err := net.SplitHostPort(server); err == nil {
		return server
	}
	if ip := net.ParseIP(server); ip != nil && ip.To4() == nil {
		return "[" + server + "]:53"
	}
	return server + ":53"
}
