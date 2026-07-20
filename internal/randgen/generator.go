// Package randgen 实现 jdan rand 子命令族的底层随机生成器。
//
// 安全约束：
//   - 所有生成器使用 crypto/rand（CSPRNG），永远不用 math/rand
//   - 从字符集抽字符必须用 crypto/rand.Int(len(charset))，禁止
//     b[i] % len(charset) 这种 mod 操作——charset 不整除 256 时频率不均匀
//     是密码学常见 footgun
//   - reader 通过参数传入，允许测试注入 fake 覆盖 crypto/rand 失败路径
package randgen

import (
	crypto_rand "crypto/rand"
	"encoding/base32"
	"encoding/base64"
	"encoding/hex"
	"errors"
	"fmt"
	"io"
	"math/big"
	"strings"
)

// 字符集常量。默认排除歧义字符（I/l/1, O/0），减少抄写时混淆。
//
// 排歧义后大小：
//   - 大写 26 - {I,O}    = 24
//   - 小写 26 - {l}      = 25
//   - 数字 10 - {0,1}    = 8
//   - 标准 symbols       = 14
//   - 合计含 symbols     = 71
//   - 合计 alnum 无 symbols = 57
const (
	charsLowerNoAmbig = "abcdefghijkmnopqrstuvwxyz" // 25, no 'l'
	charsUpperNoAmbig = "ABCDEFGHJKLMNPQRSTUVWXYZ"  // 24, no 'I', 'O'
	charsDigitNoAmbig = "23456789"                  // 8, no '0', '1'
	charsSymbols      = "!@#$%^&*()-_=+"            // 14 标准 symbols

	charsLowerFull = "abcdefghijklmnopqrstuvwxyz" // 26
	charsUpperFull = "ABCDEFGHIJKLMNOPQRSTUVWXYZ" // 26
	charsDigitFull = "0123456789"                 // 10
)

// PasswordOptions 控制 GeneratePassword 行为。
type PasswordOptions struct {
	Length           int  // 默认 20；< 类数（3 或 4）时报错
	NoSymbols        bool // true → 仅 alnum + 必含 3 类
	IncludeAmbiguous bool // true → 不排除 I/l/1/O/0
}

// randIndex 返回 [0, n) 内均匀随机整数。
// 这是字符集随机选取的唯一允许方式——禁止 b[0] % byte(n) 这种 mod 偏差写法。
func randIndex(reader io.Reader, n int) (int, error) {
	if n <= 0 {
		return 0, fmt.Errorf("randIndex: n must be > 0, got %d", n)
	}
	idx, err := crypto_rand.Int(reader, big.NewInt(int64(n)))
	if err != nil {
		return 0, err
	}
	return int(idx.Int64()), nil
}

// shuffle 对 byte slice 做 Fisher-Yates 洗牌（用 CSPRNG）。无偏差。
func shuffle(reader io.Reader, b []byte) error {
	for i := len(b) - 1; i > 0; i-- {
		j, err := randIndex(reader, i+1)
		if err != nil {
			return err
		}
		b[i], b[j] = b[j], b[i]
	}
	return nil
}

// pickChar 从 charset 中均匀抽一个字符。
func pickChar(reader io.Reader, charset string) (byte, error) {
	idx, err := randIndex(reader, len(charset))
	if err != nil {
		return 0, err
	}
	return charset[idx], nil
}

// pickN 抽 n 个独立字符。
func pickN(reader io.Reader, charset string, n int) ([]byte, error) {
	b := make([]byte, n)
	for i := 0; i < n; i++ {
		c, err := pickChar(reader, charset)
		if err != nil {
			return nil, err
		}
		b[i] = c
	}
	return b, nil
}

// passwordCharsets 根据 options 返回 4 类字符集。symbol 在 NoSymbols 时为空。
func passwordCharsets(opts PasswordOptions) (lower, upper, digit, symbol string) {
	if opts.IncludeAmbiguous {
		lower = charsLowerFull
		upper = charsUpperFull
		digit = charsDigitFull
	} else {
		lower = charsLowerNoAmbig
		upper = charsUpperNoAmbig
		digit = charsDigitNoAmbig
	}
	if !opts.NoSymbols {
		symbol = charsSymbols
	}
	return
}

// GeneratePassword 生成符合 1Password 风格的密码（design doc D3 锁定）。
//
// 算法：固定位置 + Fisher-Yates 洗牌（无偏差，对短长度也高效）：
//  1. 从每个必含类（lower/upper/digit/[symbol]）各抽 1 字符，放入前 K 位
//  2. 剩余 Length-K 位用全字符集填充
//  3. Fisher-Yates 洗牌
//
// 错误：Length < K（含 symbols 时 K=4，否则 K=3）。
func GeneratePassword(reader io.Reader, opts PasswordOptions) (string, error) {
	if opts.Length <= 0 {
		return "", errors.New("password length must be > 0")
	}
	lower, upper, digit, symbol := passwordCharsets(opts)
	classes := []string{lower, upper, digit}
	if symbol != "" {
		classes = append(classes, symbol)
	}
	minLen := len(classes)
	if opts.Length < minLen {
		return "", fmt.Errorf("password length must be >= %d (one of each class)", minLen)
	}

	full := lower + upper + digit + symbol
	out := make([]byte, opts.Length)

	// 1. 每类各抽 1 字符放固定位置
	for i, c := range classes {
		ch, err := pickChar(reader, c)
		if err != nil {
			return "", err
		}
		out[i] = ch
	}
	// 2. 剩余位置用全字符集填充
	for i := minLen; i < opts.Length; i++ {
		ch, err := pickChar(reader, full)
		if err != nil {
			return "", err
		}
		out[i] = ch
	}
	// 3. Fisher-Yates 洗牌
	if err := shuffle(reader, out); err != nil {
		return "", err
	}
	return string(out), nil
}

// GenerateHex 返回 N 字节的 hex 编码（输出长度 = 2N）。
func GenerateHex(reader io.Reader, byteLen int) (string, error) {
	if byteLen <= 0 {
		return "", errors.New("byte length must be > 0")
	}
	buf := make([]byte, byteLen)
	if _, err := io.ReadFull(reader, buf); err != nil {
		return "", err
	}
	return hex.EncodeToString(buf), nil
}

// GenerateBase64 返回 N 字节的标准 base64 编码（含 + / = padding）。
func GenerateBase64(reader io.Reader, byteLen int) (string, error) {
	if byteLen <= 0 {
		return "", errors.New("byte length must be > 0")
	}
	buf := make([]byte, byteLen)
	if _, err := io.ReadFull(reader, buf); err != nil {
		return "", err
	}
	return base64.StdEncoding.EncodeToString(buf), nil
}

// GenerateBase64URL 返回 N 字节的 URL-safe base64（用 - _ 代替 + /，无 padding）。
func GenerateBase64URL(reader io.Reader, byteLen int) (string, error) {
	if byteLen <= 0 {
		return "", errors.New("byte length must be > 0")
	}
	buf := make([]byte, byteLen)
	if _, err := io.ReadFull(reader, buf); err != nil {
		return "", err
	}
	return base64.RawURLEncoding.EncodeToString(buf), nil
}

// GenerateBase32 返回 N 字节的 RFC 4648 标准 base32（大写 A-Z + 2-7 + padding）。
func GenerateBase32(reader io.Reader, byteLen int) (string, error) {
	if byteLen <= 0 {
		return "", errors.New("byte length must be > 0")
	}
	buf := make([]byte, byteLen)
	if _, err := io.ReadFull(reader, buf); err != nil {
		return "", err
	}
	return base32.StdEncoding.EncodeToString(buf), nil
}

// GenerateAlnum 返回 length 个字母数字字符。**无类约束**——这是与
// `password --no-symbols` 的关键区别（后者仍要求每类至少一个）。
func GenerateAlnum(reader io.Reader, length int, includeAmbiguous bool) (string, error) {
	if length <= 0 {
		return "", errors.New("length must be > 0")
	}
	var charset string
	if includeAmbiguous {
		charset = charsLowerFull + charsUpperFull + charsDigitFull
	} else {
		charset = charsLowerNoAmbig + charsUpperNoAmbig + charsDigitNoAmbig
	}
	b, err := pickN(reader, charset, length)
	if err != nil {
		return "", err
	}
	return string(b), nil
}

// GenerateInt 返回 [min, max] 闭区间内的均匀随机整数。支持负数；min == max 也合法。
func GenerateInt(reader io.Reader, min, max int64) (int64, error) {
	if max < min {
		return 0, fmt.Errorf("max must be >= min: got min=%d max=%d", min, max)
	}
	// rangeSize = max - min + 1。用 big.Int 防 int64 溢出（虽然实践极少触发）。
	rangeSize := new(big.Int).Sub(big.NewInt(max), big.NewInt(min))
	rangeSize.Add(rangeSize, big.NewInt(1))
	if rangeSize.Sign() <= 0 {
		return 0, fmt.Errorf("range overflow: min=%d max=%d", min, max)
	}
	n, err := crypto_rand.Int(reader, rangeSize)
	if err != nil {
		return 0, err
	}
	return n.Int64() + min, nil
}

// charsetsForTesting 暴露字符集常量给同包测试使用，避免测试里硬编码。
func charsetsForTesting() (lo, up, di, sy, loFull, upFull, diFull string) {
	return charsLowerNoAmbig, charsUpperNoAmbig, charsDigitNoAmbig, charsSymbols,
		charsLowerFull, charsUpperFull, charsDigitFull
}

// charsetContains 是同包测试 helper：判断 s 中是否至少有一个字符属于 charset。
func charsetContains(s, charset string) bool {
	for _, c := range s {
		if strings.ContainsRune(charset, c) {
			return true
		}
	}
	return false
}
