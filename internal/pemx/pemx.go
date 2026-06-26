// Package pemx 实现 jdan pem：离线读一个 PEM 文件，认出每个块的类型并给摘要。
// 设计要点：
//   - 绝不输出私钥材料，私钥块只给「类型 + 位数」
//   - CERTIFICATE 块复用 internal/sslcert.Describe，不重写证书解析
//   - 单文件正好 1 个叶子证书 + 1 个私钥时，比对公钥给「key↔cert 是否匹配」
package pemx

import (
	"crypto"
	"crypto/ecdsa"
	"crypto/ed25519"
	"crypto/rsa"
	"crypto/x509"
	"encoding/json"
	"encoding/pem"
	"fmt"
	"strings"

	"github.com/xunull/jdan/internal/sslcert"
)

// KeyInfo 描述一个密钥块（私钥/公钥）——只含算法，绝无密钥字节。
type KeyInfo struct {
	Algorithm string `json:"algorithm"` // "RSA 2048" / "EC P-256" / "Ed25519"
	Public    bool   `json:"public"`
}

// CSRInfo 描述一个证书签名请求块。
type CSRInfo struct {
	Subject      string   `json:"subject"`
	SubjectCN    string   `json:"subject_cn"`
	SAN          []string `json:"san,omitempty"`
	KeyAlgorithm string   `json:"key_algorithm"`
	SigAlgorithm string   `json:"signature_algorithm"`
}

// Block 是一个解析过的 PEM 块。
type Block struct {
	Index int
	Type  string // PEM 头，如 "CERTIFICATE"
	Kind  string // 人类类别：certificate / csr / private-key / public-key / encrypted-private-key / other
	Bytes int    // DER 字节数
	Err   string // 解析失败时的原因（块保留、行内标注）
	Note  string // 如「私钥内容不打印」「已加密」
	Cert  *sslcert.Summary
	CSR   *CSRInfo
	Key   *KeyInfo
}

// Result 是整个文件的检视结果。
type Result struct {
	Blocks         []Block
	KeyMatchesCert *bool // 仅当正好 1 叶子证书 + 1 私钥时设置
}

type parsedItem struct {
	cert *x509.Certificate
	priv crypto.PrivateKey
}

// Inspect 遍历 data 里所有 PEM 块并解析。无 PEM 块时报错。
func Inspect(data []byte) (Result, error) {
	var (
		res   Result
		certs []*x509.Certificate
		privs []crypto.PrivateKey
		rest  = data
		idx   int
	)
	for {
		blk, r := pem.Decode(rest)
		if blk == nil {
			break
		}
		rest = r
		idx++
		b, item := classify(idx, blk)
		res.Blocks = append(res.Blocks, b)
		if item.cert != nil {
			certs = append(certs, item.cert)
		}
		if item.priv != nil {
			privs = append(privs, item.priv)
		}
	}
	if len(res.Blocks) == 0 {
		return Result{}, fmt.Errorf("没有发现 PEM 块（输入不是 PEM？）")
	}
	res.KeyMatchesCert = keyMatch(certs, privs)
	return res, nil
}

func classify(idx int, blk *pem.Block) (Block, parsedItem) {
	b := Block{Index: idx, Type: blk.Type, Bytes: len(blk.Bytes)}

	// 旧式加密私钥（DEK-Info 头），不尝试解密
	if _, enc := blk.Headers["DEK-Info"]; enc {
		b.Kind = "encrypted-private-key"
		b.Note = "已加密（未解密，不读细节）"
		return b, parsedItem{}
	}

	switch blk.Type {
	case "CERTIFICATE":
		b.Kind = "certificate"
		c, err := x509.ParseCertificate(blk.Bytes)
		if err != nil {
			b.Err = err.Error()
			return b, parsedItem{}
		}
		s := sslcert.Describe(c)
		b.Cert = &s
		return b, parsedItem{cert: c}

	case "CERTIFICATE REQUEST", "NEW CERTIFICATE REQUEST":
		b.Kind = "csr"
		csr, err := x509.ParseCertificateRequest(blk.Bytes)
		if err != nil {
			b.Err = err.Error()
			return b, parsedItem{}
		}
		b.CSR = &CSRInfo{
			Subject:      csr.Subject.String(),
			SubjectCN:    csr.Subject.CommonName,
			SAN:          csr.DNSNames,
			KeyAlgorithm: pubKeyAlg(csr.PublicKey),
			SigAlgorithm: csr.SignatureAlgorithm.String(),
		}
		return b, parsedItem{}

	case "PUBLIC KEY":
		b.Kind = "public-key"
		pub, err := x509.ParsePKIXPublicKey(blk.Bytes)
		if err != nil {
			b.Err = err.Error()
			return b, parsedItem{}
		}
		b.Key = &KeyInfo{Algorithm: pubKeyAlg(pub), Public: true}
		return b, parsedItem{}

	case "RSA PRIVATE KEY":
		b.Kind = "private-key"
		b.Note = "私钥内容不打印"
		k, err := x509.ParsePKCS1PrivateKey(blk.Bytes)
		if err != nil {
			b.Err = err.Error()
			return b, parsedItem{}
		}
		b.Key = &KeyInfo{Algorithm: privKeyAlg(k)}
		return b, parsedItem{priv: k}

	case "EC PRIVATE KEY":
		b.Kind = "private-key"
		b.Note = "私钥内容不打印"
		k, err := x509.ParseECPrivateKey(blk.Bytes)
		if err != nil {
			b.Err = err.Error()
			return b, parsedItem{}
		}
		b.Key = &KeyInfo{Algorithm: privKeyAlg(k)}
		return b, parsedItem{priv: k}

	case "PRIVATE KEY": // PKCS#8
		b.Kind = "private-key"
		b.Note = "私钥内容不打印"
		k, err := x509.ParsePKCS8PrivateKey(blk.Bytes)
		if err != nil {
			b.Err = err.Error()
			return b, parsedItem{}
		}
		b.Key = &KeyInfo{Algorithm: privKeyAlg(k)}
		return b, parsedItem{priv: k}

	case "ENCRYPTED PRIVATE KEY": // PKCS#8 加密
		b.Kind = "encrypted-private-key"
		b.Note = "已加密（未解密，不读细节）"
		return b, parsedItem{}

	default: // EC PARAMETERS / DH PARAMETERS / X509 CRL / ...
		b.Kind = "other"
		return b, parsedItem{}
	}
}

func pubKeyAlg(pub any) string {
	switch k := pub.(type) {
	case *rsa.PublicKey:
		return fmt.Sprintf("RSA %d", k.N.BitLen())
	case *ecdsa.PublicKey:
		return "EC " + k.Curve.Params().Name
	case ed25519.PublicKey:
		return "Ed25519"
	default:
		return "unknown"
	}
}

func privKeyAlg(priv any) string {
	switch k := priv.(type) {
	case *rsa.PrivateKey:
		return fmt.Sprintf("RSA %d", k.N.BitLen())
	case *ecdsa.PrivateKey:
		return "EC " + k.Curve.Params().Name
	case ed25519.PrivateKey:
		return "Ed25519"
	default:
		return "unknown"
	}
}

// keyMatch 在「正好 1 叶子证书 + 1 私钥」时比对公钥，否则返回 nil。
func keyMatch(certs []*x509.Certificate, privs []crypto.PrivateKey) *bool {
	leaf := leafCert(certs)
	if leaf == nil || len(privs) != 1 {
		return nil
	}
	pwp, ok := privs[0].(interface{ Public() crypto.PublicKey })
	if !ok {
		return nil
	}
	eq := publicKeysEqual(leaf.PublicKey, pwp.Public())
	return &eq
}

func leafCert(certs []*x509.Certificate) *x509.Certificate {
	var leaves []*x509.Certificate
	for _, c := range certs {
		if !c.IsCA {
			leaves = append(leaves, c)
		}
	}
	if len(leaves) == 1 {
		return leaves[0]
	}
	if len(certs) == 1 {
		return certs[0]
	}
	return nil
}

func publicKeysEqual(a, b crypto.PublicKey) bool {
	type equaler interface{ Equal(crypto.PublicKey) bool }
	if e, ok := a.(equaler); ok {
		return e.Equal(b)
	}
	return false
}

// FormatText 渲染成文本。
func (r Result) FormatText() string {
	var b strings.Builder
	for _, blk := range r.Blocks {
		fmt.Fprintf(&b, "[%d] %s  (%d bytes)\n", blk.Index, blk.Type, blk.Bytes)
		if blk.Err != "" {
			fmt.Fprintf(&b, "    解析失败: %s\n", blk.Err)
		}
		switch {
		case blk.Cert != nil:
			writeCert(&b, blk.Cert)
		case blk.CSR != nil:
			writeCSR(&b, blk.CSR)
		case blk.Key != nil:
			fmt.Fprintf(&b, "    key:      %s", blk.Key.Algorithm)
			if blk.Note != "" {
				fmt.Fprintf(&b, "   （%s）", blk.Note)
			}
			b.WriteString("\n")
		case blk.Note != "":
			fmt.Fprintf(&b, "    %s\n", blk.Note)
		}
		b.WriteString("\n")
	}
	if r.KeyMatchesCert != nil {
		if *r.KeyMatchesCert {
			b.WriteString("✓ 私钥与证书匹配\n")
		} else {
			b.WriteString("✗ 私钥与证书不匹配\n")
		}
	}
	return b.String()
}

func writeCert(b *strings.Builder, s *sslcert.Summary) {
	fmt.Fprintf(b, "    subject:  %s\n", s.Subject)
	fmt.Fprintf(b, "    issuer:   %s\n", s.Issuer)
	validity := fmt.Sprintf("%s → %s", s.NotBefore.Format("2006-01-02"), s.NotAfter.Format("2006-01-02"))
	switch {
	case s.Expired:
		validity += "  (已过期)"
	case s.NotYetValid:
		validity += "  (尚未生效)"
	default:
		validity += fmt.Sprintf("  (剩余 %dd)", s.DaysLeft)
	}
	fmt.Fprintf(b, "    validity: %s\n", validity)
	if len(s.SAN) > 0 {
		fmt.Fprintf(b, "    SAN:      %s\n", strings.Join(s.SAN, ", "))
	}
	fmt.Fprintf(b, "    key:      %s\n", s.KeyAlgorithm)
	fmt.Fprintf(b, "    CA:       %v\n", s.IsCA)
	fmt.Fprintf(b, "    sha256:   %s\n", s.SHA256)
}

func writeCSR(b *strings.Builder, c *CSRInfo) {
	fmt.Fprintf(b, "    subject:  %s\n", c.Subject)
	if len(c.SAN) > 0 {
		fmt.Fprintf(b, "    SAN:      %s\n", strings.Join(c.SAN, ", "))
	}
	fmt.Fprintf(b, "    key:      %s\n", c.KeyAlgorithm)
	fmt.Fprintf(b, "    sigalg:   %s\n", c.SigAlgorithm)
}

// FormatJSON 渲染成结构化输出。
func (r Result) FormatJSON() (string, error) {
	type blockJSON struct {
		Index int              `json:"index"`
		Type  string           `json:"type"`
		Kind  string           `json:"kind"`
		Bytes int              `json:"bytes"`
		Error string           `json:"error,omitempty"`
		Note  string           `json:"note,omitempty"`
		Cert  *sslcert.Summary `json:"certificate,omitempty"`
		CSR   *CSRInfo         `json:"csr,omitempty"`
		Key   *KeyInfo         `json:"key,omitempty"`
	}
	out := struct {
		Blocks         []blockJSON `json:"blocks"`
		KeyMatchesCert *bool       `json:"key_matches_cert,omitempty"`
	}{KeyMatchesCert: r.KeyMatchesCert}
	for _, blk := range r.Blocks {
		out.Blocks = append(out.Blocks, blockJSON{
			Index: blk.Index, Type: blk.Type, Kind: blk.Kind, Bytes: blk.Bytes,
			Error: blk.Err, Note: blk.Note, Cert: blk.Cert, CSR: blk.CSR, Key: blk.Key,
		})
	}
	data, err := json.MarshalIndent(out, "", "  ")
	if err != nil {
		return "", err
	}
	return string(data), nil
}
