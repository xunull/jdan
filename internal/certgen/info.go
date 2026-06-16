package certgen

import (
	"crypto/sha256"
	"crypto/x509"
	"encoding/base64"
	"strings"
)

// FingerprintSHA256 返回 "SHA256:base64nopad" 格式的证书指纹（跟 jdan ssl cert
// 以及 openssl 显示对齐）。
func FingerprintSHA256(cert *x509.Certificate) string {
	sum := sha256.Sum256(cert.Raw)
	return "SHA256:" + base64.RawStdEncoding.EncodeToString(sum[:])
}

// SANString 返回证书 SAN 的人类可读串（"DNS:localhost, IP:127.0.0.1"）。
func SANString(cert *x509.Certificate) string {
	var parts []string
	for _, d := range cert.DNSNames {
		parts = append(parts, "DNS:"+d)
	}
	for _, ip := range cert.IPAddresses {
		parts = append(parts, "IP:"+ip.String())
	}
	return strings.Join(parts, ", ")
}
