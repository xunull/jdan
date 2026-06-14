package sslcert

import (
	"context"
	"crypto/tls"
	"errors"
	"fmt"
	"net"
	"strconv"
	"strings"
	"time"
)

// FetchOptions 控制一次 FetchFromHost 的行为。
type FetchOptions struct {
	// Host 是用于 TCP dial 的地址。
	Host string
	// Port 默认 443。
	Port int
	// SNI 是 TLS 握手发的 server_name；为空时用 Host。
	SNI string
	// Timeout 整体超时；默认 5s。
	Timeout time.Duration
}

// FetchFromHost 对 host:port 做 TLS 握手，拿 server 发的 cert chain。
//
// 关键设计：InsecureSkipVerify=true。我们要"看证书"，不能因为 cert 不可信
// 就直接拒绝；信任状态留给 verify.go 单独做并报告。
//
// 复用 internal/netprobe/tls.go 的概念但不直接 import 它——保持 sslcert
// 是独立单一职责的 package，netprobe 可以反过来用 sslcert 升级 TLS 输出。
func FetchFromHost(ctx context.Context, opts FetchOptions) (*Bundle, error) {
	if opts.Host == "" {
		return nil, errors.New("host required")
	}
	if opts.Port == 0 {
		opts.Port = 443
	}
	if opts.SNI == "" {
		opts.SNI = opts.Host
	}
	if opts.Timeout == 0 {
		opts.Timeout = 5 * time.Second
	}

	addr := net.JoinHostPort(opts.Host, strconv.Itoa(opts.Port))
	dialer := &net.Dialer{Timeout: opts.Timeout}

	cctx, cancel := context.WithTimeout(ctx, opts.Timeout)
	defer cancel()

	tcpConn, err := dialer.DialContext(cctx, "tcp", addr)
	if err != nil {
		return nil, fmt.Errorf("dial %s: %w", addr, err)
	}
	tlsConn := tls.Client(tcpConn, &tls.Config{
		ServerName:         opts.SNI,
		InsecureSkipVerify: true, // 见上面注释
	})
	if err := tlsConn.HandshakeContext(cctx); err != nil {
		_ = tcpConn.Close()
		return nil, fmt.Errorf("TLS handshake: %w", err)
	}
	chain := tlsConn.ConnectionState().PeerCertificates
	_ = tlsConn.Close()

	if len(chain) == 0 {
		return nil, errors.New("server returned no certificates")
	}
	return &Bundle{
		Source: addr,
		Host:   opts.Host,
		Chain:  chain,
	}, nil
}

// ParseTarget 拆 "host" / "host:port" / "https://host/..." 三种形态。
// 返回 (host, port)。port 默认 443。
func ParseTarget(s string) (string, int, error) {
	s = strings.TrimSpace(s)
	if s == "" {
		return "", 0, errors.New("empty target")
	}
	// 处理 https://host[:port]/path
	if strings.HasPrefix(s, "https://") {
		s = strings.TrimPrefix(s, "https://")
		if i := strings.Index(s, "/"); i >= 0 {
			s = s[:i]
		}
	} else if strings.HasPrefix(s, "http://") {
		return "", 0, errors.New("http:// is plaintext; jdan ssl cert needs HTTPS")
	}

	host, port, err := net.SplitHostPort(s)
	if err != nil {
		// 没 port，整个串就是 host
		return s, 443, nil
	}
	p, err := strconv.Atoi(port)
	if err != nil {
		return "", 0, fmt.Errorf("invalid port %q", port)
	}
	if p < 1 || p > 65535 {
		return "", 0, fmt.Errorf("port %d out of range", p)
	}
	return host, p, nil
}
