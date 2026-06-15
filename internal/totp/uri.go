package totp

import (
	"fmt"
	"net/url"
	"strconv"
	"strings"
)

// OtpauthParams 是从 otpauth:// URI 解析出的全部字段。
type OtpauthParams struct {
	Type      string // "totp"（"hotp" 不支持，TOTP 是 99% 场景）
	Issuer    string // 服务商，如 "GitHub"
	Account   string // 账号，如 "quincy@example.com"
	Secret    string // base32 secret（未解码）
	Algorithm Algorithm
	Digits    int
	Period    int
}

// Config 把解析出的参数转成 TOTP Config。
func (p OtpauthParams) Config() Config {
	return Config{
		Digits:    p.Digits,
		Period:    p.Period,
		Algorithm: p.Algorithm,
	}.normalize()
}

// ParseOtpauthURI 解析标准 otpauth:// URI（扫二维码得到的格式）。
//
// 格式：otpauth://totp/LABEL?secret=...&issuer=...&algorithm=...&digits=...&period=...
// LABEL 通常是 "Issuer:Account"，issuer 也可能在 query 里（query 优先）。
func ParseOtpauthURI(raw string) (OtpauthParams, error) {
	u, err := url.Parse(strings.TrimSpace(raw))
	if err != nil {
		return OtpauthParams{}, fmt.Errorf("parse otpauth URI: %w", err)
	}
	if u.Scheme != "otpauth" {
		return OtpauthParams{}, fmt.Errorf("not an otpauth:// URI (scheme %q)", u.Scheme)
	}
	typ := strings.ToLower(u.Host)
	if typ != "totp" {
		return OtpauthParams{}, fmt.Errorf("only totp supported, got %q", typ)
	}

	p := OtpauthParams{Type: typ}

	// LABEL = path 去掉前导 "/"，可能是 "Issuer:Account"
	label := strings.TrimPrefix(u.Path, "/")
	if label != "" {
		if iss, acct, found := strings.Cut(label, ":"); found {
			p.Issuer = strings.TrimSpace(iss)
			p.Account = strings.TrimSpace(acct)
		} else {
			p.Account = label
		}
	}

	q := u.Query()
	p.Secret = q.Get("secret")
	if p.Secret == "" {
		return OtpauthParams{}, fmt.Errorf("otpauth URI missing secret")
	}
	// query 的 issuer 优先于 label 里的
	if iss := q.Get("issuer"); iss != "" {
		p.Issuer = iss
	}

	algo, err := ParseAlgorithm(q.Get("algorithm"))
	if err != nil {
		return OtpauthParams{}, err
	}
	p.Algorithm = algo

	p.Digits = atoiDefault(q.Get("digits"), 6)
	p.Period = atoiDefault(q.Get("period"), 30)
	return p, nil
}

func atoiDefault(s string, def int) int {
	if s == "" {
		return def
	}
	n, err := strconv.Atoi(s)
	if err != nil || n <= 0 {
		return def
	}
	return n
}
