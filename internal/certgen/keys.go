// Package certgen 实现 jdan cert 命令的核心：生成本地开发用的自签名 TLS 证书。
//
// 全部基于 stdlib crypto（x509 / ecdsa / rsa / ed25519），0 新依赖。
// 仅限本地开发 / 测试，不要用于生产（生产证书走 ACME / certbot）。
package certgen

import (
	"crypto"
	"crypto/ecdsa"
	"crypto/ed25519"
	"crypto/elliptic"
	"crypto/rand"
	"crypto/rsa"
	"crypto/x509"
	"encoding/pem"
	"fmt"
	"strings"
)

// KeyType 是私钥算法。
type KeyType string

const (
	KeyEC      KeyType = "ec"      // P-256，快/小，默认
	KeyRSA     KeyType = "rsa"     // RSA 2048
	KeyEd25519 KeyType = "ed25519" // Ed25519
)

// ParseKeyType 解析 key type（大小写不敏感）。空字符串默认 EC。
func ParseKeyType(s string) (KeyType, error) {
	switch strings.ToLower(strings.TrimSpace(s)) {
	case "", "ec", "ecdsa", "p256":
		return KeyEC, nil
	case "rsa":
		return KeyRSA, nil
	case "ed25519":
		return KeyEd25519, nil
	default:
		return "", fmt.Errorf("unsupported key type %q (want ec/rsa/ed25519)", s)
	}
}

// Label 返回人类可读的算法名（用于输出）。
func (k KeyType) Label() string {
	switch k {
	case KeyRSA:
		return "RSA (2048)"
	case KeyEd25519:
		return "Ed25519"
	default:
		return "EC (P-256)"
	}
}

// generateKey 按 KeyType 生成一对新私钥。返回 crypto.Signer（统一接口）。
func generateKey(kt KeyType) (crypto.Signer, error) {
	switch kt {
	case KeyRSA:
		return rsa.GenerateKey(rand.Reader, 2048)
	case KeyEd25519:
		_, priv, err := ed25519.GenerateKey(rand.Reader)
		return priv, err
	default: // EC
		return ecdsa.GenerateKey(elliptic.P256(), rand.Reader)
	}
}

// encodePrivateKeyPEM 把私钥编码成 PKCS#8 PEM。
func encodePrivateKeyPEM(key crypto.Signer) ([]byte, error) {
	der, err := x509.MarshalPKCS8PrivateKey(key)
	if err != nil {
		return nil, fmt.Errorf("marshal private key: %w", err)
	}
	return pem.EncodeToMemory(&pem.Block{Type: "PRIVATE KEY", Bytes: der}), nil
}

// encodeCertPEM 把证书 DER 编码成 PEM。
func encodeCertPEM(der []byte) []byte {
	return pem.EncodeToMemory(&pem.Block{Type: "CERTIFICATE", Bytes: der})
}
