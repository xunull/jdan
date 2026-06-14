package sslscan

import (
	"context"
	"crypto/tls"
	"fmt"
	"net"
	"strconv"
	"time"
)

// CipherStrength 是 cipher 强度分类。
type CipherStrength string

const (
	StrengthStrong     CipherStrength = "strong"     // AES-GCM / ChaCha20 + ECDHE forward secrecy
	StrengthAcceptable CipherStrength = "acceptable" // AES-CBC / 无 forward secrecy 但算法仍强
	StrengthWeak       CipherStrength = "weak"       // RC4 / DES / 3DES / NULL / EXPORT
)

// CipherResult 是单个 cipher 的探测结果。
type CipherResult struct {
	Name      string         `json:"name"`            // e.g. "ECDHE-RSA-AES256-GCM-SHA384"
	Hex       string         `json:"id"`              // 0xc02f
	Supported bool           `json:"supported"`
	Strength  CipherStrength `json:"strength"`
}

// CiphersSection 是 TLS 1.2 cipher 枚举的整体结果。
type CiphersSection struct {
	TLS12 []CipherResult `json:"tls12"`
	// TLS 1.3 cipher 是固定 mandatory 5 个，不枚举
	TLS13Note string `json:"tls13_note,omitempty"`
}

// 16 个常见 TLS 1.2 cipher。覆盖 ECDHE-RSA / ECDHE-ECDSA / AES-128/256 GCM/CBC /
// ChaCha20 + 几个 weak 例子（让用户能看到 server 是否接受弱密）。
var commonCipherSuites = []uint16{
	// 强：ECDHE 提供前向安全
	tls.TLS_ECDHE_RSA_WITH_AES_256_GCM_SHA384,
	tls.TLS_ECDHE_RSA_WITH_AES_128_GCM_SHA256,
	tls.TLS_ECDHE_RSA_WITH_CHACHA20_POLY1305,
	tls.TLS_ECDHE_ECDSA_WITH_AES_256_GCM_SHA384,
	tls.TLS_ECDHE_ECDSA_WITH_AES_128_GCM_SHA256,
	tls.TLS_ECDHE_ECDSA_WITH_CHACHA20_POLY1305,
	// acceptable：CBC mode
	tls.TLS_ECDHE_RSA_WITH_AES_256_CBC_SHA,
	tls.TLS_ECDHE_RSA_WITH_AES_128_CBC_SHA,
	tls.TLS_ECDHE_ECDSA_WITH_AES_256_CBC_SHA,
	tls.TLS_ECDHE_ECDSA_WITH_AES_128_CBC_SHA,
	// 无前向安全：RSA key exchange
	tls.TLS_RSA_WITH_AES_256_GCM_SHA384,
	tls.TLS_RSA_WITH_AES_128_GCM_SHA256,
	tls.TLS_RSA_WITH_AES_256_CBC_SHA,
	tls.TLS_RSA_WITH_AES_128_CBC_SHA,
	// 弱密
	tls.TLS_RSA_WITH_3DES_EDE_CBC_SHA,
	tls.TLS_ECDHE_RSA_WITH_3DES_EDE_CBC_SHA,
}

// fullCipherSuites 是 Go stdlib 暴露的所有 TLS 1.2 cipher（含 stdlib 已废
// 但仍能尝试的）。--full-cipher 时用这个。
var fullCipherSuites = []uint16{
	tls.TLS_RSA_WITH_AES_128_CBC_SHA,
	tls.TLS_RSA_WITH_AES_256_CBC_SHA,
	tls.TLS_RSA_WITH_AES_128_GCM_SHA256,
	tls.TLS_RSA_WITH_AES_256_GCM_SHA384,
	tls.TLS_RSA_WITH_AES_128_CBC_SHA256,
	tls.TLS_RSA_WITH_3DES_EDE_CBC_SHA,
	tls.TLS_ECDHE_ECDSA_WITH_AES_128_CBC_SHA,
	tls.TLS_ECDHE_ECDSA_WITH_AES_256_CBC_SHA,
	tls.TLS_ECDHE_RSA_WITH_AES_128_CBC_SHA,
	tls.TLS_ECDHE_RSA_WITH_AES_256_CBC_SHA,
	tls.TLS_ECDHE_RSA_WITH_AES_128_CBC_SHA256,
	tls.TLS_ECDHE_ECDSA_WITH_AES_128_CBC_SHA256,
	tls.TLS_ECDHE_RSA_WITH_AES_128_GCM_SHA256,
	tls.TLS_ECDHE_ECDSA_WITH_AES_128_GCM_SHA256,
	tls.TLS_ECDHE_RSA_WITH_AES_256_GCM_SHA384,
	tls.TLS_ECDHE_ECDSA_WITH_AES_256_GCM_SHA384,
	tls.TLS_ECDHE_RSA_WITH_CHACHA20_POLY1305,
	tls.TLS_ECDHE_ECDSA_WITH_CHACHA20_POLY1305,
	tls.TLS_ECDHE_RSA_WITH_3DES_EDE_CBC_SHA,
}

func scanCiphers(ctx context.Context, opts Options) CiphersSection {
	addr := net.JoinHostPort(opts.Host, strconv.Itoa(opts.Port))
	list := commonCipherSuites
	if opts.FullCipher {
		list = fullCipherSuites
	}
	out := CiphersSection{
		TLS13Note: "TLS 1.3 ciphers are mandatory (5 fixed suites); not enumerated",
	}
	for _, c := range list {
		r := CipherResult{
			Name:     tls.CipherSuiteName(c),
			Hex:      fmt.Sprintf("0x%04x", c),
			Strength: classifyCipher(c),
		}
		err := tryHandshakeCipher(ctx, addr, opts.SNI, c)
		if err == nil {
			r.Supported = true
		}
		out.TLS12 = append(out.TLS12, r)
	}
	return out
}

func tryHandshakeCipher(ctx context.Context, addr, sni string, cipher uint16) error {
	cctx, cancel := context.WithTimeout(ctx, 2*time.Second)
	defer cancel()
	dialer := &net.Dialer{Timeout: 2 * time.Second}
	tcpConn, err := dialer.DialContext(cctx, "tcp", addr)
	if err != nil {
		return err
	}
	defer tcpConn.Close()
	tlsConn := tls.Client(tcpConn, &tls.Config{
		ServerName:         sni,
		InsecureSkipVerify: true,
		MinVersion:         tls.VersionTLS12,
		MaxVersion:         tls.VersionTLS12,
		CipherSuites:       []uint16{cipher},
	})
	defer tlsConn.Close()
	return tlsConn.HandshakeContext(cctx)
}

// classifyCipher 把 cipher ID 分到强 / 可接受 / 弱 三档。
//
// 规则：
//   - 含 RC4 / DES / 3DES / EXPORT / NULL / MD5 → weak
//   - ECDHE + GCM / ChaCha20 → strong（forward secrecy + AEAD）
//   - ECDHE + CBC → acceptable（forward secrecy 但 CBC 老）
//   - 无 ECDHE → acceptable（无前向安全，但算法本身不弱）
func classifyCipher(c uint16) CipherStrength {
	name := tls.CipherSuiteName(c)
	if containsCipherWeak(name) {
		return StrengthWeak
	}
	hasECDHE := contains(name, "ECDHE")
	hasAEAD := contains(name, "GCM") || contains(name, "CHACHA20")
	if hasECDHE && hasAEAD {
		return StrengthStrong
	}
	return StrengthAcceptable
}

func containsCipherWeak(name string) bool {
	for _, w := range []string{"RC4", "_DES_", "3DES", "EXPORT", "NULL", "MD5", "EXPORT40"} {
		if contains(name, w) {
			return true
		}
	}
	return false
}

// SupportedStrong 返回支持的 strong cipher 数。给 grade 用。
func (c CiphersSection) SupportedStrong() int {
	n := 0
	for _, r := range c.TLS12 {
		if r.Supported && r.Strength == StrengthStrong {
			n++
		}
	}
	return n
}

// SupportedWeak 返回支持的 weak cipher 数。给 grade 减分。
func (c CiphersSection) SupportedWeak() int {
	n := 0
	for _, r := range c.TLS12 {
		if r.Supported && r.Strength == StrengthWeak {
			n++
		}
	}
	return n
}

// SupportedNonFS 返回不带前向安全（无 ECDHE）的支持 cipher 数。
func (c CiphersSection) SupportedNonFS() int {
	n := 0
	for _, r := range c.TLS12 {
		if !r.Supported {
			continue
		}
		if !contains(r.Name, "ECDHE") && !contains(r.Name, "DHE") {
			n++
		}
	}
	return n
}
