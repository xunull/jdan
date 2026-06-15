package sshkey

import (
	"strings"

	"golang.org/x/crypto/ssh"
)

// PublicKeyLine 从私钥 signer 导出 authorized_keys 格式的公钥行（= ssh-keygen -y）。
// comment 非空时附在末尾。
func PublicKeyLine(signer ssh.Signer, comment string) string {
	line := strings.TrimSpace(string(ssh.MarshalAuthorizedKey(signer.PublicKey())))
	if comment != "" {
		line += " " + comment
	}
	return line
}
