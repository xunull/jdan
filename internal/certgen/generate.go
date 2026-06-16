package certgen

import (
	"crypto"
	"crypto/rand"
	"crypto/x509"
	"crypto/x509/pkix"
	"fmt"
	"math/big"
	"time"
)

// Options 是生成一张证书的参数。
type Options struct {
	CommonName string
	SANs       SANs
	Days       int
	KeyType    KeyType
	// now 注入便于测试有效期；零值时用 time.Now
	now func() time.Time
}

// Result 是生成结果（PEM 编码 + 解析后的证书供 info）。
type Result struct {
	CertPEM []byte
	KeyPEM  []byte
	Cert    *x509.Certificate
}

func (o Options) clock() time.Time {
	if o.now != nil {
		return o.now()
	}
	return time.Now()
}

func (o Options) days() int {
	if o.Days <= 0 {
		return 825 // 浏览器接受的 leaf 上限
	}
	return o.Days
}

// randSerial 生成一个随机 128-bit 序列号。
func randSerial() (*big.Int, error) {
	limit := new(big.Int).Lsh(big.NewInt(1), 128)
	return rand.Int(rand.Reader, limit)
}

// GenerateSelfSigned 生成一张自签名 leaf 证书。
func GenerateSelfSigned(o Options) (*Result, error) {
	key, err := generateKey(o.KeyType)
	if err != nil {
		return nil, err
	}
	tmpl, err := leafTemplate(o)
	if err != nil {
		return nil, err
	}
	der, err := x509.CreateCertificate(rand.Reader, tmpl, tmpl, key.Public(), key)
	if err != nil {
		return nil, fmt.Errorf("create certificate: %w", err)
	}
	return buildResult(der, key)
}

// CA 是一个生成出来的 CA（用于签发 leaf）。
type CA struct {
	CertPEM []byte
	KeyPEM  []byte
	cert    *x509.Certificate
	key     crypto.Signer
}

// GenerateCA 生成一个本地 CA（可签发任意 leaf）。
func GenerateCA(o Options) (*CA, error) {
	key, err := generateKey(o.KeyType)
	if err != nil {
		return nil, err
	}
	serial, err := randSerial()
	if err != nil {
		return nil, err
	}
	now := o.clock()
	cn := o.CommonName
	if cn == "" {
		cn = "jdan local CA"
	}
	tmpl := &x509.Certificate{
		SerialNumber:          serial,
		Subject:               pkix.Name{CommonName: cn + " (jdan local dev CA)"},
		NotBefore:             now.Add(-time.Hour),
		NotAfter:              now.AddDate(0, 0, o.days()),
		KeyUsage:              x509.KeyUsageCertSign | x509.KeyUsageCRLSign | x509.KeyUsageDigitalSignature,
		BasicConstraintsValid: true,
		IsCA:                  true,
		MaxPathLenZero:        true,
	}
	der, err := x509.CreateCertificate(rand.Reader, tmpl, tmpl, key.Public(), key)
	if err != nil {
		return nil, fmt.Errorf("create CA certificate: %w", err)
	}
	cert, err := x509.ParseCertificate(der)
	if err != nil {
		return nil, err
	}
	certPEM := encodeCertPEM(der)
	keyPEM, err := encodePrivateKeyPEM(key)
	if err != nil {
		return nil, err
	}
	return &CA{CertPEM: certPEM, KeyPEM: keyPEM, cert: cert, key: key}, nil
}

// SignLeaf 用 CA 签发一张 leaf 证书。
func (ca *CA) SignLeaf(o Options) (*Result, error) {
	key, err := generateKey(o.KeyType)
	if err != nil {
		return nil, err
	}
	tmpl, err := leafTemplate(o)
	if err != nil {
		return nil, err
	}
	der, err := x509.CreateCertificate(rand.Reader, tmpl, ca.cert, key.Public(), ca.key)
	if err != nil {
		return nil, fmt.Errorf("sign leaf with CA: %w", err)
	}
	return buildResult(der, key)
}

// leafTemplate 构造 leaf 证书模板（server auth + SAN）。
func leafTemplate(o Options) (*x509.Certificate, error) {
	serial, err := randSerial()
	if err != nil {
		return nil, err
	}
	now := o.clock()
	cn := o.CommonName
	if cn == "" && len(o.SANs.DNS) > 0 {
		cn = o.SANs.DNS[0]
	}
	return &x509.Certificate{
		SerialNumber: serial,
		Subject:      pkix.Name{CommonName: cn},
		NotBefore:    now.Add(-time.Hour),
		NotAfter:     now.AddDate(0, 0, o.days()),
		KeyUsage:     x509.KeyUsageDigitalSignature | x509.KeyUsageKeyEncipherment,
		ExtKeyUsage:  []x509.ExtKeyUsage{x509.ExtKeyUsageServerAuth},
		DNSNames:     o.SANs.DNS,
		IPAddresses:  o.SANs.IPs,
	}, nil
}

func buildResult(der []byte, key crypto.Signer) (*Result, error) {
	cert, err := x509.ParseCertificate(der)
	if err != nil {
		return nil, err
	}
	keyPEM, err := encodePrivateKeyPEM(key)
	if err != nil {
		return nil, err
	}
	return &Result{
		CertPEM: encodeCertPEM(der),
		KeyPEM:  keyPEM,
		Cert:    cert,
	}, nil
}
