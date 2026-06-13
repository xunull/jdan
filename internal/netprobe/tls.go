package netprobe

import (
	"context"
	"crypto/tls"
	"crypto/x509"
	"fmt"
	"net"
	"time"
)

// TLSDetail 是 TLS 阶段的特有字段。connState 是非导出字段，给后续 HTTP 阶段复用。
type TLSDetail struct {
	Version     string    `json:"version"`
	CipherSuite string    `json:"cipher_suite"`
	ALPN        string    `json:"alpn,omitempty"`
	SNI         string    `json:"sni"`
	LeafSubject string    `json:"leaf_subject"`
	LeafIssuer  string    `json:"leaf_issuer"`
	NotAfter    time.Time `json:"not_after"`
	NotBefore   time.Time `json:"not_before"`
	ChainDepth  int       `json:"chain_depth"`
	Insecure    bool      `json:"insecure"` // true 表示用了 InsecureSkipVerify

	connState *tls.ConnectionState // 给 HTTP 阶段复用
}

func runTLS(ctx context.Context, t *Target, ip net.IP, opts Options) *StageResult {
	stageStart := time.Now()
	r := &StageResult{Stage: StageTLS}

	cfg := &tls.Config{
		ServerName:         t.Host, // SNI
		InsecureSkipVerify: opts.Insecure,
		NextProtos:         []string{"h2", "http/1.1"},
	}

	addr := dialAddr(ip, t.Port)
	dialer := &net.Dialer{Timeout: opts.Timeout}

	// 用 tls.Dialer 在 TCP 已建好之后做 handshake；但为了准确测 TLS 单独耗时，
	// 我们手动两步：TCP connect 一次 + TLS handshake 一次。
	tcpConn, err := dialer.DialContext(ctx, "tcp", addr)
	if err != nil {
		// TCP 阶段已经成功，这里再失败说明短时间内连接已断；保留原错误
		r.Success = false
		r.Err = err.Error()
		r.Hint = "TCP succeeded earlier but failed on re-dial; transient network issue?"
		r.Duration = time.Since(stageStart)
		return r
	}

	tlsConn := tls.Client(tcpConn, cfg)
	hsCtx, cancel := context.WithTimeout(ctx, opts.Timeout)
	defer cancel()
	if err := tlsConn.HandshakeContext(hsCtx); err != nil {
		_ = tcpConn.Close()
		r.Success = false
		r.Class = ClassifyTLSError(err)
		r.Err = err.Error()
		r.Explanation = WhatItMeans(r.Class)
		r.Hint = HintForClass(r.Class)
		r.Duration = time.Since(stageStart)
		return r
	}

	state := tlsConn.ConnectionState()
	_ = tlsConn.Close()
	r.Duration = time.Since(stageStart)
	r.Success = true

	d := &TLSDetail{
		Version:     tlsVersionString(state.Version),
		CipherSuite: tls.CipherSuiteName(state.CipherSuite),
		ALPN:        state.NegotiatedProtocol,
		SNI:         cfg.ServerName,
		ChainDepth:  len(state.PeerCertificates),
		Insecure:    opts.Insecure,
		connState:   &state,
	}
	if len(state.PeerCertificates) > 0 {
		leaf := state.PeerCertificates[0]
		d.LeafSubject = subjectShort(leaf.Subject)
		d.LeafIssuer = subjectShort(leaf.Issuer)
		d.NotAfter = leaf.NotAfter
		d.NotBefore = leaf.NotBefore
	}
	r.TLS = d
	r.Detail = fmt.Sprintf("%s, cert: %s (issued by %s, exp %s)",
		d.Version, d.LeafSubject, d.LeafIssuer, d.NotAfter.Format("2006-01-02"))
	return r
}

func tlsVersionString(v uint16) string {
	switch v {
	case tls.VersionTLS10:
		return "TLS 1.0"
	case tls.VersionTLS11:
		return "TLS 1.1"
	case tls.VersionTLS12:
		return "TLS 1.2"
	case tls.VersionTLS13:
		return "TLS 1.3"
	}
	return fmt.Sprintf("TLS 0x%04x", v)
}

// subjectShort 把 x509 subject 收成 "CN=..." 或 "O=..."；找不到给完整 String()
func subjectShort(s interface {
	String() string
}) string {
	if name, ok := s.(interface {
		String() string
	}); ok {
		full := name.String()
		// pkix.Name.String() 形如 "CN=github.com,O=GitHub\\, Inc.,..."
		if cn := extractRDN(full, "CN="); cn != "" {
			return cn
		}
		if o := extractRDN(full, "O="); o != "" {
			return o
		}
		return full
	}
	return ""
}

// 直接处理 x509.Certificate 的 Subject (pkix.Name)
var _ = x509.Certificate{}

func extractRDN(s, prefix string) string {
	// 找 "CN=xxx,..." 的 xxx 部分。简单的 prefix + 逗号 split。
	idx := indexOf(s, prefix)
	if idx == -1 {
		return ""
	}
	rest := s[idx+len(prefix):]
	end := indexOf(rest, ",")
	if end == -1 {
		return rest
	}
	return rest[:end]
}

func indexOf(s, sub string) int {
	for i := 0; i+len(sub) <= len(s); i++ {
		if s[i:i+len(sub)] == sub {
			return i
		}
	}
	return -1
}
