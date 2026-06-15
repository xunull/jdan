package totp

import (
	"crypto/hmac"
	"crypto/sha1"
	"crypto/sha256"
	"crypto/sha512"
	"encoding/binary"
	"fmt"
	"hash"
	"strings"
)

// Algorithm 是 HMAC 用的哈希算法。
type Algorithm string

const (
	AlgoSHA1   Algorithm = "SHA1"
	AlgoSHA256 Algorithm = "SHA256"
	AlgoSHA512 Algorithm = "SHA512"
)

// ParseAlgorithm 解析算法名（大小写不敏感）。空字符串默认 SHA1。
func ParseAlgorithm(s string) (Algorithm, error) {
	switch strings.ToUpper(strings.TrimSpace(s)) {
	case "", "SHA1":
		return AlgoSHA1, nil
	case "SHA256":
		return AlgoSHA256, nil
	case "SHA512":
		return AlgoSHA512, nil
	default:
		return "", fmt.Errorf("unsupported algorithm %q (want SHA1/SHA256/SHA512)", s)
	}
}

func (a Algorithm) newHash() func() hash.Hash {
	switch a {
	case AlgoSHA256:
		return sha256.New
	case AlgoSHA512:
		return sha512.New
	default:
		return sha1.New
	}
}

// pow10 是 10^n 表（digits 一般 6 或 8）。
var pow10 = [...]uint32{1, 10, 100, 1000, 10000, 100000, 1000000, 10000000, 100000000}

// HOTP 计算 RFC 4226 的 HMAC-based one-time password。
//   - key: 解码后的 secret bytes
//   - counter: 8-byte big-endian 计数器（TOTP 里是 time/period）
//   - digits: 输出位数（6 或 8）
//   - algo: HMAC 哈希算法
//
// 返回零填充到 digits 位的十进制字符串。
func HOTP(key []byte, counter uint64, digits int, algo Algorithm) string {
	var buf [8]byte
	binary.BigEndian.PutUint64(buf[:], counter)

	mac := hmac.New(algo.newHash(), key)
	mac.Write(buf[:])
	sum := mac.Sum(nil)

	// RFC 4226 §5.3 dynamic truncation
	offset := sum[len(sum)-1] & 0x0F
	code := (uint32(sum[offset]&0x7F) << 24) |
		(uint32(sum[offset+1]) << 16) |
		(uint32(sum[offset+2]) << 8) |
		uint32(sum[offset+3])

	if digits < 1 || digits >= len(pow10) {
		digits = 6
	}
	code %= pow10[digits]
	return fmt.Sprintf("%0*d", digits, code)
}
