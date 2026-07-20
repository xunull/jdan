package sslscan

import (
	"context"
	"crypto/tls"
	"fmt"
	"net"
	"strconv"
	"time"
)

// VersionResult 是单个 TLS 版本探测结果。
type VersionResult struct {
	Version    string `json:"version"` // "TLS 1.0" / "TLS 1.1" / "TLS 1.2" / "TLS 1.3"
	Supported  bool   `json:"supported"`
	Err        string `json:"error,omitempty"`
	Deprecated bool   `json:"deprecated"` // TLS 1.0 / 1.1 是 deprecated
}

// VersionsSection 是版本探测的整体结果。
type VersionsSection struct {
	Results []VersionResult `json:"results"`
}

// tlsVersionsToScan 是我们要探测的版本列表。SSL 3.0 不做（Go stdlib 已移除）。
var tlsVersionsToScan = []struct {
	name       string
	id         uint16
	deprecated bool
}{
	{"TLS 1.0", tls.VersionTLS10, true},
	{"TLS 1.1", tls.VersionTLS11, true},
	{"TLS 1.2", tls.VersionTLS12, false},
	{"TLS 1.3", tls.VersionTLS13, false},
}

func scanVersions(ctx context.Context, opts Options) VersionsSection {
	out := VersionsSection{}
	addr := net.JoinHostPort(opts.Host, strconv.Itoa(opts.Port))

	for _, v := range tlsVersionsToScan {
		r := VersionResult{
			Version:    v.name,
			Deprecated: v.deprecated,
		}
		err := tryHandshake(ctx, addr, opts.SNI, v.id)
		if err == nil {
			r.Supported = true
		} else {
			r.Err = err.Error()
		}
		out.Results = append(out.Results, r)
	}
	return out
}

// tryHandshake 用 MinVersion=MaxVersion=version 强制单一版本做 TLS 握手。
// 成功 = server 支持这个版本。
//
// CipherSuites 故意留空让 Go 用默认（兼容性最大）。
func tryHandshake(ctx context.Context, addr, sni string, version uint16) error {
	cctx, cancel := context.WithTimeout(ctx, 3*time.Second)
	defer cancel()

	dialer := &net.Dialer{Timeout: 3 * time.Second}
	tcpConn, err := dialer.DialContext(cctx, "tcp", addr)
	if err != nil {
		return fmt.Errorf("dial: %w", err)
	}
	defer tcpConn.Close()

	tlsConn := tls.Client(tcpConn, &tls.Config{
		ServerName:         sni,
		InsecureSkipVerify: true, // scan 不关心 cert 是否可信
		MinVersion:         version,
		MaxVersion:         version,
	})
	defer tlsConn.Close()

	return tlsConn.HandshakeContext(cctx)
}

// supportedVersions 给 grade.go 用的辅助：返回支持的最高 TLS 版本。
// 找不到（全失败）返回 0。
func (s VersionsSection) HighestSupported() uint16 {
	highest := uint16(0)
	for _, r := range s.Results {
		if !r.Supported {
			continue
		}
		v := versionStringToID(r.Version)
		if v > highest {
			highest = v
		}
	}
	return highest
}

// SupportedSet 返回支持的版本集合，给 grade 评判用。
func (s VersionsSection) SupportedSet() map[uint16]bool {
	out := make(map[uint16]bool)
	for _, r := range s.Results {
		if r.Supported {
			out[versionStringToID(r.Version)] = true
		}
	}
	return out
}

func versionStringToID(s string) uint16 {
	switch s {
	case "TLS 1.0":
		return tls.VersionTLS10
	case "TLS 1.1":
		return tls.VersionTLS11
	case "TLS 1.2":
		return tls.VersionTLS12
	case "TLS 1.3":
		return tls.VersionTLS13
	}
	return 0
}
