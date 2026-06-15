// Package totp 实现 TOTP（RFC 6238）+ HOTP（RFC 4226）+ base32 secret 解码 +
// otpauth:// URI 解析。全部基于 stdlib（crypto/hmac、encoding/base32、net/url），
// 0 新依赖。
//
// 设计要点：
//   - 默认参数对齐 Google Authenticator（SHA1 / 6 位 / 30s period）
//   - base32 secret 容忍小写、空格分组、缺 padding（用户复制时常带噪音）
//   - 跟 RFC 6238/4226 官方测试向量 byte-equal，能交叉验证
package totp

import (
	"encoding/base32"
	"fmt"
	"strings"
)

// DecodeSecret 把用户提供的 base32 secret 解码成原始 key bytes。
//
// 容错处理（实际 secret 复制时常见噪音）：
//   - 小写 → 转大写（base32 字母表是大写）
//   - 空格分组（"JBSW Y3DP" 是 Google 显示格式）→ 去掉
//   - 连字符 / 缺 padding → 去连字符，按需补 "=" padding
func DecodeSecret(s string) ([]byte, error) {
	cleaned := strings.ToUpper(strings.TrimSpace(s))
	cleaned = strings.ReplaceAll(cleaned, " ", "")
	cleaned = strings.ReplaceAll(cleaned, "-", "")
	if cleaned == "" {
		return nil, fmt.Errorf("empty secret")
	}
	// base32 标准要求长度是 8 的倍数（带 padding）；多数服务省略 padding，补回。
	if pad := len(cleaned) % 8; pad != 0 {
		cleaned += strings.Repeat("=", 8-pad)
	}
	key, err := base32.StdEncoding.DecodeString(cleaned)
	if err != nil {
		return nil, fmt.Errorf("invalid base32 secret: %w", err)
	}
	return key, nil
}
