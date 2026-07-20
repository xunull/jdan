package sshkey

import (
	"crypto"
	"crypto/ecdsa"
	"crypto/ed25519"
	"crypto/rsa"
	"fmt"
	"strings"

	"golang.org/x/crypto/ssh"
)

// KeyInfo 是一个 SSH key（公钥或私钥）的综合信息。
type KeyInfo struct {
	Kind           string `json:"kind"`              // "public" | "private"
	Type           string `json:"type"`              // ssh-ed25519 / ssh-rsa / ecdsa-sha2-nistp256 ...
	Algorithm      string `json:"algorithm"`         // Ed25519 / RSA / ECDSA
	Bits           int    `json:"bits"`              // 密钥位数
	Comment        string `json:"comment,omitempty"` // key comment
	FingerprintSHA string `json:"fingerprint_sha256"`
	FingerprintMD5 string `json:"fingerprint_md5"`
	Encrypted      bool   `json:"encrypted,omitempty"`       // 仅私钥
	PublicKeyLine  string `json:"public_key_line,omitempty"` // 私钥导出的 authorized_keys 行
	SecurityKey    bool   `json:"security_key,omitempty"`    // sk-* (FIDO/U2F 硬件密钥)
}

// InfoFromPublicKey 从 ssh.PublicKey + comment 组装 KeyInfo。
func InfoFromPublicKey(pub ssh.PublicKey, comment string) KeyInfo {
	keyType := pub.Type()
	info := KeyInfo{
		Kind:           "public",
		Type:           keyType,
		Algorithm:      algorithmName(keyType),
		Bits:           keyBits(pub),
		Comment:        comment,
		FingerprintSHA: FingerprintSHA256(pub),
		FingerprintMD5: FingerprintMD5(pub),
		SecurityKey:    strings.HasPrefix(keyType, "sk-"),
	}
	return info
}

// InfoFromSigner 从私钥 signer 组装 KeyInfo（导出公钥后复用公钥逻辑）。
func InfoFromSigner(signer ssh.Signer, comment string) KeyInfo {
	pub := signer.PublicKey()
	info := InfoFromPublicKey(pub, comment)
	info.Kind = "private"
	info.PublicKeyLine = strings.TrimSpace(string(ssh.MarshalAuthorizedKey(pub)))
	if comment != "" {
		info.PublicKeyLine += " " + comment
	}
	return info
}

// algorithmName 把 SSH key type 映射到人类可读的算法名。
func algorithmName(keyType string) string {
	switch {
	case strings.Contains(keyType, "ed25519"):
		return "Ed25519"
	case strings.Contains(keyType, "rsa"):
		return "RSA"
	case strings.Contains(keyType, "ecdsa"):
		return "ECDSA"
	case strings.Contains(keyType, "dss"):
		return "DSA"
	default:
		return keyType
	}
}

// keyBits 计算密钥位数。
//   - Ed25519: 固定 256
//   - RSA: modulus 位数
//   - ECDSA: 曲线位数（P-256 → 256 等）
//
// 走 ssh.CryptoPublicKey 接口拿到底层 crypto.PublicKey 再判类型。
func keyBits(pub ssh.PublicKey) int {
	// sk-* 硬件密钥也基于 ed25519/ecdsa，名字带曲线信息
	cpk, ok := pub.(ssh.CryptoPublicKey)
	if !ok {
		return bitsFromTypeName(pub.Type())
	}
	switch k := cpk.CryptoPublicKey().(type) {
	case *rsa.PublicKey:
		return k.N.BitLen()
	case *ecdsa.PublicKey:
		return k.Curve.Params().BitSize
	case ed25519.PublicKey:
		return 256
	case crypto.PublicKey:
		return bitsFromTypeName(pub.Type())
	}
	return bitsFromTypeName(pub.Type())
}

// bitsFromTypeName 是 fallback：从 key type 名字推位数（ecdsa-sha2-nistp384 → 384）。
func bitsFromTypeName(keyType string) int {
	switch {
	case strings.Contains(keyType, "ed25519"):
		return 256
	case strings.Contains(keyType, "nistp256"):
		return 256
	case strings.Contains(keyType, "nistp384"):
		return 384
	case strings.Contains(keyType, "nistp521"):
		return 521
	}
	return 0
}

// String 渲染人类可读的 info 表（CLI 默认输出）。
func (info KeyInfo) String() string {
	var sb strings.Builder
	row := func(label, val string) {
		if val != "" {
			fmt.Fprintf(&sb, "%-13s %s\n", label+":", val)
		}
	}
	if info.Kind == "private" {
		row("Type", "OpenSSH private key")
	} else {
		row("Type", info.Type)
	}
	if info.Encrypted {
		row("Encrypted", "yes (passphrase-protected; cannot derive public key without it)")
		return sb.String()
	}
	row("Algorithm", info.Algorithm)
	if info.Bits > 0 {
		row("Bits", fmt.Sprintf("%d", info.Bits))
	}
	if info.SecurityKey {
		row("Hardware", "yes (FIDO/U2F security key)")
	}
	row("Comment", info.Comment)
	row("Fingerprint", info.FingerprintSHA)
	row("MD5", info.FingerprintMD5)
	if info.PublicKeyLine != "" {
		row("Public key", info.PublicKeyLine)
	}
	return sb.String()
}
