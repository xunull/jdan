package sshkey

import (
	"golang.org/x/crypto/ssh"
)

// FingerprintSHA256 返回 "SHA256:base64nopad" 格式，跟 ssh-keygen -lf 默认输出
// 以及 GitHub SSH key 页面显示的 byte-equal。
func FingerprintSHA256(pub ssh.PublicKey) string {
	return ssh.FingerprintSHA256(pub)
}

// FingerprintMD5 返回 "MD5:xx:xx:..." colon-hex 格式，跟 ssh-keygen -lf -E md5
// 以及老 server 显示的 legacy 指纹 byte-equal。
func FingerprintMD5(pub ssh.PublicKey) string {
	return "MD5:" + ssh.FingerprintLegacyMD5(pub)
}
