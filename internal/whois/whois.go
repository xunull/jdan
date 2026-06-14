package whois

import (
	"context"
	"fmt"
	"io"
	"net"
	"strings"
	"time"
)

// DefaultPort 是 WHOIS 协议端口（RFC 3912）。
const DefaultPort = "43"

// DefaultTimeout 一次查询的默认超时。WHOIS 服务器有快有慢，5s 兜得住大多数。
const DefaultTimeout = 5 * time.Second

// maxReferralHops 限制 referral 跟随次数，防止恶意 server 卡死循环。
const maxReferralHops = 3

// Query 向 server 发送 target 查询并返回 raw 响应。
//   - server 不带端口时自动补 :43
//   - 单次 TCP 连接、单次 query、读到 EOF
//   - 用 context 控制取消；timeout 同时作为 IO deadline
func Query(ctx context.Context, server, target string, timeout time.Duration) (string, error) {
	if timeout <= 0 {
		timeout = DefaultTimeout
	}
	if !strings.Contains(server, ":") {
		server = server + ":" + DefaultPort
	}
	d := net.Dialer{Timeout: timeout}
	conn, err := d.DialContext(ctx, "tcp", server)
	if err != nil {
		return "", fmt.Errorf("dial %s: %w", server, err)
	}
	defer conn.Close()
	_ = conn.SetDeadline(time.Now().Add(timeout))
	if _, err := io.WriteString(conn, target+"\r\n"); err != nil {
		return "", fmt.Errorf("write query: %w", err)
	}
	body, err := io.ReadAll(conn)
	if err != nil {
		return "", fmt.Errorf("read response: %w", err)
	}
	return string(body), nil
}

// Lookup 高层 entry：自动路由 + 跟随 referral，返回完整 Result。
func Lookup(ctx context.Context, target string, timeout time.Duration) (*Result, error) {
	server, kind, err := RoutingFor(target)
	if err != nil {
		return nil, err
	}
	return lookupChain(ctx, target, kind, server, timeout)
}

// LookupWithServer 绕过路由表，直接用指定 server 查（--server flag 走这里）。
func LookupWithServer(ctx context.Context, target, server string, timeout time.Duration) (*Result, error) {
	kind, err := detectKind(target)
	if err != nil {
		return nil, err
	}
	raw, err := Query(ctx, server, target, timeout)
	if err != nil {
		return nil, err
	}
	return &Result{Target: target, Kind: kind, Server: server, RawText: raw}, nil
}

// lookupChain 跟随 referral 链。
//   - IANA root → tld server（一跳）
//   - ARIN → RIPE/APNIC/LACNIC/AFRINIC（一跳）
//   - 最多 maxReferralHops 跳防止 server 故意制造环
func lookupChain(ctx context.Context, target string, kind Kind, server string, timeout time.Duration) (*Result, error) {
	res := &Result{Target: target, Kind: kind}
	cur := server
	for hop := range maxReferralHops {
		raw, err := Query(ctx, cur, target, timeout)
		if err != nil {
			return nil, err
		}
		// IANA root 一定要跟到真实 TLD server
		if cur == IANARoot {
			if next := ParseIANAReferral(raw); next != "" {
				res.Hops = append(res.Hops, Hop{Server: cur})
				cur = next
				continue
			}
			// IANA 没给 referral（罕见）：返回 IANA 的响应
			res.Server = cur
			res.RawText = raw
			return res, nil
		}
		// IP 跨 RIR referral（ARIN → RIPE 等）
		if kind == KindIPv4 || kind == KindIPv6 {
			next := ParseReferral(raw)
			if next != "" && next != cur && hop < maxReferralHops-1 {
				res.Hops = append(res.Hops, Hop{Server: cur})
				cur = next
				continue
			}
		}
		// 终态
		res.Server = cur
		res.RawText = raw
		return res, nil
	}
	return nil, fmt.Errorf("too many WHOIS referrals (>%d) starting from %s", maxReferralHops, server)
}
