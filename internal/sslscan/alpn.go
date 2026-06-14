package sslscan

import (
	"context"
	"crypto/tls"
	"net"
	"strconv"
	"time"
)

// ALPNSection 是 ALPN 协议探测结果。
type ALPNSection struct {
	Supported []string `json:"supported"` // 服务端 select 出的协议（一次一个）
	Tested    []string `json:"tested"`    // 我们 advertise 了哪些
}

// scanALPN 探测 server 支持哪些 ALPN 协议。Go TLS 客户端一次只能 select
// 一个协议，所以我们逐个 advertise + 看 NegotiatedProtocol。
//
// 注：HTTP/3 走 QUIC（UDP）不在 TCP+TLS 范围内，这里测不到。
func scanALPN(ctx context.Context, opts Options) ALPNSection {
	addr := net.JoinHostPort(opts.Host, strconv.Itoa(opts.Port))
	out := ALPNSection{
		Tested: []string{"h2", "http/1.1"},
	}
	for _, proto := range out.Tested {
		neg, err := tryHandshakeALPN(ctx, addr, opts.SNI, []string{proto})
		if err != nil {
			continue
		}
		if neg == proto {
			out.Supported = append(out.Supported, neg)
		}
	}
	return out
}

func tryHandshakeALPN(ctx context.Context, addr, sni string, protos []string) (string, error) {
	cctx, cancel := context.WithTimeout(ctx, 2*time.Second)
	defer cancel()
	dialer := &net.Dialer{Timeout: 2 * time.Second}
	tcpConn, err := dialer.DialContext(cctx, "tcp", addr)
	if err != nil {
		return "", err
	}
	defer tcpConn.Close()
	tlsConn := tls.Client(tcpConn, &tls.Config{
		ServerName:         sni,
		InsecureSkipVerify: true,
		NextProtos:         protos,
	})
	defer tlsConn.Close()
	if err := tlsConn.HandshakeContext(cctx); err != nil {
		return "", err
	}
	return tlsConn.ConnectionState().NegotiatedProtocol, nil
}

// HasH2 报告 server 是否支持 HTTP/2。
func (a ALPNSection) HasH2() bool {
	for _, p := range a.Supported {
		if p == "h2" {
			return true
		}
	}
	return false
}
