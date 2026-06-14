package sslcert

import (
	"crypto/ecdsa"
	"crypto/ed25519"
	"crypto/rsa"
	"crypto/sha1"
	"crypto/sha256"
	"crypto/x509"
	"fmt"
	"strings"
	"time"
)

// Summary 是 *x509.Certificate 的人类友好摘要，用于 cli render 和 JSON 输出。
type Summary struct {
	Subject       string    `json:"subject"`        // CN=foo, O=bar,...
	SubjectCN     string    `json:"subject_cn"`
	Issuer        string    `json:"issuer"`
	IssuerCN      string    `json:"issuer_cn"`
	NotBefore     time.Time `json:"not_before"`
	NotAfter      time.Time `json:"not_after"`
	ValidDays     int       `json:"valid_days"`     // NotAfter - NotBefore 总天数
	DaysLeft      int       `json:"days_left"`      // NotAfter - now
	Expired       bool      `json:"expired"`
	NotYetValid   bool      `json:"not_yet_valid"`

	SAN           []string  `json:"san"`            // DNS names from SAN extension
	IPAddresses   []string  `json:"ip_addresses,omitempty"`
	EmailSANs     []string  `json:"email_sans,omitempty"`
	URISANs       []string  `json:"uri_sans,omitempty"`

	KeyAlgorithm  string    `json:"key_algorithm"`  // RSA 2048 / EC P-256 / Ed25519
	SigAlgorithm  string    `json:"signature_algorithm"`

	Serial        string    `json:"serial"`         // hex, colon-separated
	SHA256        string    `json:"sha256"`         // fingerprint
	SHA1          string    `json:"sha1"`           // 仍然有人用 SHA1 pin

	IsCA          bool      `json:"is_ca"`
	IsSelfSigned  bool      `json:"is_self_signed"`
	KeyUsage      []string  `json:"key_usage,omitempty"`
	ExtKeyUsage   []string  `json:"ext_key_usage,omitempty"`

	OCSPServer    []string  `json:"ocsp_server,omitempty"`
	IssuingCertURL []string `json:"issuing_cert_url,omitempty"`
	CRLDistribution []string `json:"crl_distribution_points,omitempty"`
}

// Describe 把 x509.Certificate 转成 Summary。
func Describe(c *x509.Certificate) Summary {
	if c == nil {
		return Summary{}
	}
	now := Now()
	s := Summary{
		Subject:        c.Subject.String(),
		SubjectCN:      c.Subject.CommonName,
		Issuer:         c.Issuer.String(),
		IssuerCN:       c.Issuer.CommonName,
		NotBefore:      c.NotBefore,
		NotAfter:       c.NotAfter,
		ValidDays:      int(c.NotAfter.Sub(c.NotBefore).Hours() / 24),
		DaysLeft:       int(c.NotAfter.Sub(now).Hours() / 24),
		Expired:        now.After(c.NotAfter),
		NotYetValid:    now.Before(c.NotBefore),
		SAN:            c.DNSNames,
		KeyAlgorithm:   keyAlgorithmString(c),
		SigAlgorithm:   c.SignatureAlgorithm.String(),
		Serial:         hexColon(c.SerialNumber.Bytes()),
		SHA256:         fingerprintSHA256(c),
		SHA1:           fingerprintSHA1(c),
		IsCA:           c.IsCA,
		IsSelfSigned:   isSelfSigned(c),
		KeyUsage:       keyUsageStrings(c.KeyUsage),
		ExtKeyUsage:    extKeyUsageStrings(c.ExtKeyUsage),
		OCSPServer:     c.OCSPServer,
		IssuingCertURL: c.IssuingCertificateURL,
		CRLDistribution: c.CRLDistributionPoints,
	}
	for _, ip := range c.IPAddresses {
		s.IPAddresses = append(s.IPAddresses, ip.String())
	}
	s.EmailSANs = append(s.EmailSANs, c.EmailAddresses...)
	for _, u := range c.URIs {
		s.URISANs = append(s.URISANs, u.String())
	}
	return s
}

func keyAlgorithmString(c *x509.Certificate) string {
	switch pub := c.PublicKey.(type) {
	case *rsa.PublicKey:
		return fmt.Sprintf("RSA %d", pub.N.BitLen())
	case *ecdsa.PublicKey:
		return fmt.Sprintf("EC %s", pub.Curve.Params().Name)
	case ed25519.PublicKey:
		return "Ed25519"
	}
	// DSA / 其他算法 → 走 PublicKeyAlgorithm fallback
	return c.PublicKeyAlgorithm.String()
}

func hexColon(b []byte) string {
	if len(b) == 0 {
		return ""
	}
	parts := make([]string, len(b))
	for i, x := range b {
		parts[i] = fmt.Sprintf("%02x", x)
	}
	return strings.Join(parts, ":")
}

func fingerprintSHA256(c *x509.Certificate) string {
	h := sha256.Sum256(c.Raw)
	return hexColon(h[:])
}

func fingerprintSHA1(c *x509.Certificate) string {
	// SHA1 已被 NIST/Browser 弃用，但 cert pinning 历史 fingerprint 仍是 SHA1，保留显示
	h := sha1.Sum(c.Raw)
	return hexColon(h[:])
}

func keyUsageStrings(ku x509.KeyUsage) []string {
	var out []string
	if ku&x509.KeyUsageDigitalSignature != 0 {
		out = append(out, "DigitalSignature")
	}
	if ku&x509.KeyUsageContentCommitment != 0 {
		out = append(out, "ContentCommitment")
	}
	if ku&x509.KeyUsageKeyEncipherment != 0 {
		out = append(out, "KeyEncipherment")
	}
	if ku&x509.KeyUsageDataEncipherment != 0 {
		out = append(out, "DataEncipherment")
	}
	if ku&x509.KeyUsageKeyAgreement != 0 {
		out = append(out, "KeyAgreement")
	}
	if ku&x509.KeyUsageCertSign != 0 {
		out = append(out, "CertSign")
	}
	if ku&x509.KeyUsageCRLSign != 0 {
		out = append(out, "CRLSign")
	}
	return out
}

func extKeyUsageStrings(eku []x509.ExtKeyUsage) []string {
	var out []string
	for _, u := range eku {
		out = append(out, ekuName(u))
	}
	return out
}

func ekuName(u x509.ExtKeyUsage) string {
	switch u {
	case x509.ExtKeyUsageServerAuth:
		return "ServerAuth"
	case x509.ExtKeyUsageClientAuth:
		return "ClientAuth"
	case x509.ExtKeyUsageCodeSigning:
		return "CodeSigning"
	case x509.ExtKeyUsageEmailProtection:
		return "EmailProtection"
	case x509.ExtKeyUsageTimeStamping:
		return "TimeStamping"
	case x509.ExtKeyUsageOCSPSigning:
		return "OCSPSigning"
	}
	return fmt.Sprintf("EKU(%d)", u)
}

// ShortName 截短一个 cert 在 chain 列表里显示用的短名：优先 CN，其次 O，
// 都没有再用完整 Subject string。
func ShortName(c *x509.Certificate) string {
	if c == nil {
		return ""
	}
	if c.Subject.CommonName != "" {
		return "CN=" + c.Subject.CommonName
	}
	if len(c.Subject.Organization) > 0 {
		return "O=" + c.Subject.Organization[0]
	}
	return c.Subject.String()
}
