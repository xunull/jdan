package sslscan

import (
	"context"
	"time"

	"github.com/xunull/jdan/internal/sslcert"
)

// CertSection 是 scan 输出里的 cert 概要（只保留评分需要的字段，详细信息
// 用户可以单独跑 jdan ssl cert）。
type CertSection struct {
	SubjectCN    string `json:"subject_cn"`
	IssuerCN     string `json:"issuer_cn"`
	KeyAlgorithm string `json:"key_algorithm"`
	SigAlgorithm string `json:"signature_algorithm"`
	DaysLeft     int    `json:"days_left"`
	Expired      bool   `json:"expired"`
	Trusted      bool   `json:"trusted"`
	HostnameOK   bool   `json:"hostname_ok"`
	KeySizeBits  int    `json:"key_size_bits"` // RSA bits 或 EC curve bits
	IsWeakSig    bool   `json:"is_weak_sig"`   // SHA1 等已废算法
	IsWeakKey    bool   `json:"is_weak_key"`   // RSA < 2048 / DH < 2048
}

func scanCert(ctx context.Context, opts Options) *CertSection {
	cctx, cancel := context.WithTimeout(ctx, 5*time.Second)
	defer cancel()
	b, err := sslcert.FetchFromHost(cctx, sslcert.FetchOptions{
		Host: opts.Host, Port: opts.Port, SNI: opts.SNI, Timeout: 5 * time.Second,
	})
	if err != nil {
		return nil
	}
	report := sslcert.Verify(b, opts.Host)
	leaf := b.Leaf()
	if leaf == nil {
		return nil
	}
	summary := sslcert.Describe(leaf)
	s := &CertSection{
		SubjectCN:    summary.SubjectCN,
		IssuerCN:     summary.IssuerCN,
		KeyAlgorithm: summary.KeyAlgorithm,
		SigAlgorithm: summary.SigAlgorithm,
		DaysLeft:     summary.DaysLeft,
		Expired:      summary.Expired,
		Trusted:      report.Trusted,
		HostnameOK:   report.HostnameOK,
	}
	s.KeySizeBits = keyBitsFromAlgo(summary.KeyAlgorithm)
	s.IsWeakSig = isWeakSig(summary.SigAlgorithm)
	s.IsWeakKey = isWeakKey(summary.KeyAlgorithm, s.KeySizeBits)
	return s
}

// keyBitsFromAlgo 从 "RSA 2048" / "EC P-256" 这类字符串里抽 bit 数。
// 找不到返回 0。
func keyBitsFromAlgo(algo string) int {
	if algo == "" {
		return 0
	}
	// RSA N → N
	for _, prefix := range []string{"RSA "} {
		if len(algo) > len(prefix) && algo[:len(prefix)] == prefix {
			n := 0
			for i := len(prefix); i < len(algo); i++ {
				c := algo[i]
				if c < '0' || c > '9' {
					break
				}
				n = n*10 + int(c-'0')
			}
			return n
		}
	}
	// EC P-256 / P-384 / P-521 → 256 / 384 / 521
	if len(algo) > 5 && algo[:3] == "EC " {
		rest := algo[3:]
		if len(rest) > 2 && rest[:2] == "P-" {
			n := 0
			for i := 2; i < len(rest); i++ {
				c := rest[i]
				if c < '0' || c > '9' {
					break
				}
				n = n*10 + int(c-'0')
			}
			return n
		}
	}
	if algo == "Ed25519" {
		return 256
	}
	return 0
}

func isWeakSig(sig string) bool {
	for _, w := range []string{"SHA1", "MD5"} {
		if contains(sig, w) {
			return true
		}
	}
	return false
}

func isWeakKey(algo string, bits int) bool {
	if bits == 0 {
		return false
	}
	// RSA / DH < 2048 = weak
	if len(algo) > 3 && (algo[:3] == "RSA" || algo[:2] == "DH") {
		return bits < 2048
	}
	// EC < 256 = weak（P-192 / P-224）
	if len(algo) > 2 && algo[:2] == "EC" {
		return bits < 256
	}
	return false
}

func contains(s, sub string) bool {
	if len(sub) == 0 {
		return true
	}
	for i := 0; i+len(sub) <= len(s); i++ {
		if s[i:i+len(sub)] == sub {
			return true
		}
	}
	return false
}
