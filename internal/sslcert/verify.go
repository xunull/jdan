package sslcert

import (
	"crypto/x509"
	"fmt"
)

// VerificationReport 是对一个 Bundle 的 3 项验证结果。
type VerificationReport struct {
	Trusted         bool   `json:"trusted"` // 系统 trust store 能验出可信链
	TrustErr        string `json:"trust_error,omitempty"`
	HostnameOK      bool   `json:"hostname_ok"` // leaf SAN 包含目标 hostname
	HostnameErr     string `json:"hostname_error,omitempty"`
	Expired         bool   `json:"expired"`
	NotYetValid     bool   `json:"not_yet_valid"`
	HostnameSkipped bool   `json:"hostname_skipped"` // 本地 PEM 文件场景没 host 可比
}

// Verify 验证一个 Bundle。改 bundle.VerifiedChains 字段（让 RootFromTrust 能找到 root）。
func Verify(b *Bundle, hostname string) *VerificationReport {
	r := &VerificationReport{}
	if b == nil || b.Leaf() == nil {
		r.TrustErr = "empty bundle"
		return r
	}
	leaf := b.Leaf()
	now := Now()

	// 过期检查
	if now.After(leaf.NotAfter) {
		r.Expired = true
	}
	if now.Before(leaf.NotBefore) {
		r.NotYetValid = true
	}

	// 信任链验证（系统 trust store + bundle 里其他 cert 作为 intermediate）
	pool := x509.NewCertPool()
	intermediates := x509.NewCertPool()
	for i, c := range b.Chain {
		if i == 0 {
			continue
		}
		intermediates.AddCert(c)
		_ = pool // unused; trust store via VerifyOptions.Roots=nil 走系统 default
	}
	chains, err := leaf.Verify(x509.VerifyOptions{
		Intermediates: intermediates,
		CurrentTime:   now,
	})
	if err != nil {
		r.TrustErr = err.Error()
	} else {
		r.Trusted = true
		b.VerifiedChains = chains
	}

	// Hostname 检查（DNSName 验证）
	if hostname == "" {
		r.HostnameSkipped = true
	} else {
		if err := leaf.VerifyHostname(hostname); err != nil {
			r.HostnameErr = err.Error()
		} else {
			r.HostnameOK = true
		}
	}
	return r
}

// FailureSummary 把 verification 失败汇总成单行人话，给 cli 顶部的 ✗ 用。
func (r *VerificationReport) FailureSummary() string {
	if r == nil {
		return "no verification report"
	}
	var failures []string
	if !r.Trusted {
		failures = append(failures, "chain not trusted")
	}
	if r.HostnameErr != "" {
		failures = append(failures, "hostname mismatch")
	}
	if r.Expired {
		failures = append(failures, "expired")
	}
	if r.NotYetValid {
		failures = append(failures, "not yet valid")
	}
	if len(failures) == 0 {
		return ""
	}
	return fmt.Sprintf("%d issue(s): %v", len(failures), failures)
}
