// Package jwtverify 给 jdan jwt verify 提供 HMAC 校签。复用 internal/jwtdecode
// 解析结构与 alg，只补上签名校验。设计要点：
//   - 以 header.alg 为准；非 HMAC（none/RS*/ES*…）一律报错，绝不把它当 HMAC 验
//     （防 alg-confusion 攻击）
//   - HMAC 比对走 crypto/hmac.Equal（常量时间）
package jwtverify

import (
	"crypto/hmac"
	"crypto/sha256"
	"crypto/sha512"
	"encoding/base64"
	"fmt"
	"hash"
	"strings"

	"github.com/xunull/jdan/internal/jwtdecode"
)

// Verify 用 HMAC 密钥校验 JWT 签名，返回 (alg, 是否有效, error)。
// 仅支持 HS256/384/512；其余 alg 报错。
func Verify(token string, secret []byte) (string, bool, error) {
	token = strings.TrimSpace(token)
	for _, p := range []string{"Bearer ", "bearer "} {
		token = strings.TrimPrefix(token, p)
	}
	token = strings.TrimSpace(token)

	r, err := jwtdecode.Decode(token)
	if err != nil {
		return "", false, err
	}
	parts := strings.Split(token, ".")
	if len(parts) != 3 {
		return r.Algorithm, false, fmt.Errorf("不是合法 JWT：应为 3 段")
	}

	var h func() hash.Hash
	switch r.Algorithm {
	case "HS256":
		h = sha256.New
	case "HS384":
		h = sha512.New384
	case "HS512":
		h = sha512.New
	case "none", "":
		return r.Algorithm, false, fmt.Errorf("alg=%q：无签名可校验", r.Algorithm)
	default:
		return r.Algorithm, false, fmt.Errorf("暂只支持 HS256/384/512（HMAC）；%s 需要公钥校验", r.Algorithm)
	}

	sig, err := base64.RawURLEncoding.DecodeString(strings.TrimRight(parts[2], "="))
	if err != nil {
		return r.Algorithm, false, fmt.Errorf("signature base64url 解码失败: %w", err)
	}
	mac := hmac.New(h, secret)
	mac.Write([]byte(parts[0] + "." + parts[1]))
	return r.Algorithm, hmac.Equal(mac.Sum(nil), sig), nil
}
