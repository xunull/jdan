// Package pwned 实现 HIBP Pwned Passwords 的 k-匿名查询逻辑（纯函数，不联网）。
//
// 原理：算 SHA1(password)，只把【前 5 位十六进制】发给 api.pwnedpasswords.com/range/<prefix>，
// 服务器返回所有以该 prefix 开头的哈希后缀 + 出现次数，本地再比对后 35 位。服务器只看到 5 位
// 前缀（对应几十万个可能密码），明文和完整哈希都不离开本机。
package pwned

import (
	"crypto/sha1"
	"encoding/hex"
	"strconv"
	"strings"
)

// SHA1Hex 返回密码的大写十六进制 SHA1（HIBP 数据集用 SHA1）。
func SHA1Hex(password string) string {
	sum := sha1.Sum([]byte(password))
	return strings.ToUpper(hex.EncodeToString(sum[:]))
}

// SplitRange 把 40 位 SHA1 拆成 前 5 位 prefix + 后 35 位 suffix（k-匿名）。
func SplitRange(sha1hex string) (prefix, suffix string) {
	if len(sha1hex) < 5 {
		return sha1hex, ""
	}
	return sha1hex[:5], sha1hex[5:]
}

// Lookup 在 range API 返回体里查 suffix，返回出现次数（0 = 未出现 / 未收录）。
// body 每行 "SUFFIX:COUNT"，SUFFIX 是 35 位十六进制。
// Add-Padding 开启时 HIBP 会塞入 count=0 的假条目，它们自然返回 0（=未泄露）。
func Lookup(body, suffix string) int {
	want := strings.ToUpper(strings.TrimSpace(suffix))
	for line := range strings.SplitSeq(body, "\n") {
		s, c, ok := strings.Cut(strings.TrimSpace(line), ":")
		if !ok {
			continue
		}
		if strings.ToUpper(s) == want {
			n, _ := strconv.Atoi(strings.TrimSpace(c))
			return n
		}
	}
	return 0
}
