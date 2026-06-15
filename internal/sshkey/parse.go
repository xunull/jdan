// Package sshkey 实现 jdan ssh-key 命令的核心：SSH 公钥/私钥解析、fingerprint
// 计算（SHA256 base64 + legacy MD5 colon-hex）、从私钥提取公钥。
//
// 全部基于 golang.org/x/crypto/ssh（已经是 jdan 的 direct dep，0 新依赖）。
// 设计要点：
//   - 自动识别公钥（单行 "ssh-ed25519 AAAA..."）vs 私钥（PEM "-----BEGIN..."）
//   - fingerprint 跟 ssh-keygen -lf 的输出 byte-equal，能交叉验证
//   - 加密私钥（passphrase 保护）识别但不强制解密
package sshkey

import (
	"bytes"
	"errors"
	"fmt"

	"golang.org/x/crypto/ssh"
)

// pemPrivPrefix 是 OpenSSH / PEM 私钥的起始标记。
var pemPrivPrefix = []byte("-----BEGIN")

// IsPrivateKey 通过内容前缀判断是公钥还是私钥。
// 私钥都是 PEM（"-----BEGIN ... PRIVATE KEY-----"）；公钥是单行 base64。
func IsPrivateKey(data []byte) bool {
	return bytes.HasPrefix(bytes.TrimSpace(data), pemPrivPrefix)
}

// ParsePublicKey 解析 OpenSSH authorized_keys 格式的公钥。
// 返回 key + comment（comment 可能为空）。
func ParsePublicKey(data []byte) (ssh.PublicKey, string, error) {
	pub, comment, _, _, err := ssh.ParseAuthorizedKey(bytes.TrimSpace(data))
	if err != nil {
		return nil, "", fmt.Errorf("parse public key: %w", err)
	}
	return pub, comment, nil
}

// ErrEncrypted 表示私钥被 passphrase 保护，无 passphrase 无法提取公钥。
var ErrEncrypted = errors.New("private key is passphrase-protected")

// ParsePrivateKey 解析私钥。
//   - passphrase == "" 时尝试无密码解析；遇到加密私钥返回 ErrEncrypted
//   - passphrase != "" 时用它解密
//
// 返回 signer（可导出公钥）+ comment。
func ParsePrivateKey(data []byte, passphrase string) (ssh.Signer, string, error) {
	if passphrase != "" {
		signer, err := ssh.ParsePrivateKeyWithPassphrase(data, []byte(passphrase))
		if err != nil {
			return nil, "", fmt.Errorf("parse encrypted private key: %w", err)
		}
		return signer, privateKeyComment(data), nil
	}
	signer, err := ssh.ParsePrivateKey(data)
	if err != nil {
		// x/crypto 对加密私钥返回 *ssh.PassphraseMissingError
		var missing *ssh.PassphraseMissingError
		if errors.As(err, &missing) {
			return nil, "", ErrEncrypted
		}
		return nil, "", fmt.Errorf("parse private key: %w", err)
	}
	return signer, privateKeyComment(data), nil
}

// IsEncryptedPrivateKey 判断私钥是否被 passphrase 保护（不解密）。
func IsEncryptedPrivateKey(data []byte) bool {
	_, err := ssh.ParsePrivateKey(data)
	var missing *ssh.PassphraseMissingError
	return errors.As(err, &missing)
}

// privateKeyComment 从私钥里提取 comment。
//
// OpenSSH 私钥 blob 内嵌 public key + comment，但 x/crypto/ssh 不暴露解析
// comment 的 API（ParseRawPrivateKey 只给 key material）。所以这里始终返回
// 空，让 CLI 层在有同名 .pub 文件时回退读取 comment。保留独立函数是为了
// 将来 x/crypto 若暴露 comment 解析能在一处替换。
func privateKeyComment(_ []byte) string {
	return ""
}
