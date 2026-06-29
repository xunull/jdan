// Package htpasswdx 生成 / 校验 Apache/nginx Basic Auth 的密码哈希行（0 新依赖）。
//
// 三种格式（靠前缀区分）：
//   - bcrypt  $2y$...           Apache 推荐，加盐慢哈希，最安全（x/crypto/bcrypt，已在依赖图）
//   - apr1    $apr1$salt$...     Apache 的 MD5-crypt（1000 轮 MD5），老但通吃（crypto/md5 手写）
//   - {SHA}   {SHA}base64(sha1)  无盐，不安全，仅兼容老系统（crypto/sha1）
package htpasswdx

import (
	"crypto/md5"
	"crypto/sha1"
	"crypto/subtle"
	"encoding/base64"
	"errors"
	"fmt"
	"strings"

	"golang.org/x/crypto/bcrypt"
)

// DefaultCost 是 bcrypt 默认 cost。
const DefaultCost = 10

// Bcrypt 用 bcrypt 哈希密码。Go 生成 $2a$，改写成 Apache htpasswd 用的 $2y$
// （算法完全相同，仅版本标记不同）。
func Bcrypt(pw string, cost int) (string, error) {
	h, err := bcrypt.GenerateFromPassword([]byte(pw), cost)
	if err != nil {
		return "", err
	}
	s := string(h)
	if strings.HasPrefix(s, "$2a$") {
		s = "$2y$" + s[len("$2a$"):]
	}
	return s, nil
}

// SHA1 生成 {SHA}base64(sha1(pw))。无盐，不安全，仅为兼容老系统。
func SHA1(pw string) string {
	sum := sha1.Sum([]byte(pw))
	return "{SHA}" + base64.StdEncoding.EncodeToString(sum[:])
}

// APR1 生成 Apache MD5-crypt 哈希：$apr1$salt$digest。salt 最长 8 字符。
func APR1(pw, salt string) string {
	return md5crypt([]byte(pw), salt, "$apr1$")
}

// Verify 按前缀自动识别哈希格式，校验密码是否匹配。
func Verify(hash, pw string) (bool, error) {
	switch {
	case strings.HasPrefix(hash, "$2"): // bcrypt：$2a$ / $2b$ / $2y$
		h := hash
		// Go 的 bcrypt 不认 $2y$/$2x$，规整为 $2a$（同算法）再比对
		if strings.HasPrefix(h, "$2y$") || strings.HasPrefix(h, "$2x$") {
			h = "$2a$" + h[len("$2y$"):]
		}
		err := bcrypt.CompareHashAndPassword([]byte(h), []byte(pw))
		if err == nil {
			return true, nil
		}
		if errors.Is(err, bcrypt.ErrMismatchedHashAndPassword) {
			return false, nil
		}
		return false, err
	case strings.HasPrefix(hash, "$apr1$"):
		parts := strings.SplitN(hash, "$", 4) // ["", "apr1", salt, digest]
		if len(parts) != 4 {
			return false, fmt.Errorf("apr1 哈希格式错")
		}
		return constEq(APR1(pw, parts[2]), hash), nil
	case strings.HasPrefix(hash, "{SHA}"):
		return constEq(SHA1(pw), hash), nil
	default:
		return false, fmt.Errorf("无法识别的哈希格式（支持 bcrypt / apr1 / {SHA}）")
	}
}

// Upsert 把 "user:hash" 行写入 htpasswd 文件内容：同名用户替换那行，新用户追加，
// 其余行（含注释 / 空行）原样保留。
func Upsert(content, user, line string) string {
	prefix := user + ":"
	body := strings.TrimRight(content, "\n")
	var out []string
	replaced := false
	if body != "" {
		for l := range strings.SplitSeq(body, "\n") {
			if strings.HasPrefix(l, prefix) {
				out = append(out, line)
				replaced = true
			} else {
				out = append(out, l)
			}
		}
	}
	if !replaced {
		out = append(out, line)
	}
	return strings.Join(out, "\n") + "\n"
}

func constEq(a, b string) bool {
	return subtle.ConstantTimeCompare([]byte(a), []byte(b)) == 1
}

// itoa64 是 crypt(3) 系列的自定义 base64 字母表。
const itoa64 = "./0123456789ABCDEFGHIJKLMNOPQRSTUVWXYZabcdefghijklmnopqrstuvwxyz"

func to64(v uint32, n int) string {
	var b strings.Builder
	for ; n > 0; n-- {
		b.WriteByte(itoa64[v&0x3f])
		v >>= 6
	}
	return b.String()
}

// md5crypt 是 Poul-Henning Kamp 的 MD5-crypt 算法，magic 决定方言（$apr1$ = Apache）。
func md5crypt(password []byte, salt, magic string) string {
	if len(salt) > 8 {
		salt = salt[:8]
	}
	sb := []byte(salt)

	d := md5.New()
	d.Write(password)
	d.Write([]byte(magic))
	d.Write(sb)

	alt := md5.New()
	alt.Write(password)
	alt.Write(sb)
	alt.Write(password)
	altSum := alt.Sum(nil)

	for i := len(password); i > 0; i -= 16 {
		if i > 16 {
			d.Write(altSum[:16])
		} else {
			d.Write(altSum[:i])
		}
	}
	for i := len(password); i > 0; i >>= 1 {
		if i&1 != 0 {
			d.Write([]byte{0})
		} else {
			d.Write(password[:1])
		}
	}
	final := d.Sum(nil)

	for i := range 1000 {
		c := md5.New()
		if i&1 != 0 {
			c.Write(password)
		} else {
			c.Write(final[:16])
		}
		if i%3 != 0 {
			c.Write(sb)
		}
		if i%7 != 0 {
			c.Write(password)
		}
		if i&1 != 0 {
			c.Write(final[:16])
		} else {
			c.Write(password)
		}
		final = c.Sum(nil)
	}

	p := final
	enc := to64(uint32(p[0])<<16|uint32(p[6])<<8|uint32(p[12]), 4) +
		to64(uint32(p[1])<<16|uint32(p[7])<<8|uint32(p[13]), 4) +
		to64(uint32(p[2])<<16|uint32(p[8])<<8|uint32(p[14]), 4) +
		to64(uint32(p[3])<<16|uint32(p[9])<<8|uint32(p[15]), 4) +
		to64(uint32(p[4])<<16|uint32(p[10])<<8|uint32(p[5]), 4) +
		to64(uint32(p[11]), 2)
	return magic + salt + "$" + enc
}
