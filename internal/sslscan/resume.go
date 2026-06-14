package sslscan

import (
	"context"
	"crypto/tls"
	"net"
	"strconv"
	"time"
)

// ResumeSection 是 session resumption 测试结果。
type ResumeSection struct {
	TLS12TicketSupported bool   `json:"tls12_ticket_supported"`
	TLS13PSKSupported    bool   `json:"tls13_psk_supported"`
	Err                  string `json:"error,omitempty"`
}

// scanResume 跑两次握手，用 ClientSessionCache 看第二次是否能 resume。
// TLS 1.2 用 session ticket，TLS 1.3 用 PSK——同一个 cache 接口透明处理。
func scanResume(ctx context.Context, opts Options) ResumeSection {
	out := ResumeSection{}
	addr := net.JoinHostPort(opts.Host, strconv.Itoa(opts.Port))

	// 先测 TLS 1.2 session ticket
	if ok, err := tryResume(ctx, addr, opts.SNI, tls.VersionTLS12); err == nil {
		out.TLS12TicketSupported = ok
	}

	// 再测 TLS 1.3 PSK
	if ok, err := tryResume(ctx, addr, opts.SNI, tls.VersionTLS13); err == nil {
		out.TLS13PSKSupported = ok
	}

	return out
}

// tryResume 用一个新 ClientSessionCache 跑两次握手。第二次的 ConnectionState.DidResume
// 告诉我们 server 是否接受了 session 重用。
func tryResume(ctx context.Context, addr, sni string, version uint16) (bool, error) {
	cache := tls.NewLRUClientSessionCache(8)
	cfg := &tls.Config{
		ServerName:         sni,
		InsecureSkipVerify: true,
		MinVersion:         version,
		MaxVersion:         version,
		ClientSessionCache: cache,
	}

	// 第一次握手——用来 cache session ticket / PSK
	if err := doHandshake(ctx, addr, cfg); err != nil {
		return false, err
	}

	// 第二次——看是否 resume
	cctx, cancel := context.WithTimeout(ctx, 3*time.Second)
	defer cancel()
	dialer := &net.Dialer{Timeout: 3 * time.Second}
	tcpConn, err := dialer.DialContext(cctx, "tcp", addr)
	if err != nil {
		return false, err
	}
	defer tcpConn.Close()
	tlsConn := tls.Client(tcpConn, cfg)
	defer tlsConn.Close()
	if err := tlsConn.HandshakeContext(cctx); err != nil {
		return false, err
	}
	return tlsConn.ConnectionState().DidResume, nil
}

func doHandshake(ctx context.Context, addr string, cfg *tls.Config) error {
	cctx, cancel := context.WithTimeout(ctx, 3*time.Second)
	defer cancel()
	dialer := &net.Dialer{Timeout: 3 * time.Second}
	tcpConn, err := dialer.DialContext(cctx, "tcp", addr)
	if err != nil {
		return err
	}
	defer tcpConn.Close()
	tlsConn := tls.Client(tcpConn, cfg)
	defer tlsConn.Close()
	if err := tlsConn.HandshakeContext(cctx); err != nil {
		return err
	}
	// TLS 1.3 PSK 是握手"之后"才发的 NewSessionTicket，需要读一下 conn 让 ticket 入 cache
	if cfg.MaxVersion == tls.VersionTLS13 {
		_ = tlsConn.SetReadDeadline(time.Now().Add(500 * time.Millisecond))
		buf := make([]byte, 1)
		_, _ = tlsConn.Read(buf) // 期望 timeout——给 ticket 一点时间
	}
	return nil
}

// AnySupported 如果至少一种 resumption 工作，返回 true。
func (r ResumeSection) AnySupported() bool {
	return r.TLS12TicketSupported || r.TLS13PSKSupported
}
