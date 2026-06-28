// Package grabx 从任意文本里捞 URL / email / IP（纯函数，0 依赖）。
//
// 原理：纯正则永远做不完美（URL/email/IPv6 语法太复杂），所以两步走——
// 用一个【松】正则把"长得像"的候选都抓出来（宁可多抓），再用 stdlib 校验器把候选过一遍
// 留真去假并归一化：netip.ParseAddr（IP）、url.Parse（URL）、mail.ParseAddress（email）。
package grabx

import (
	"net/mail"
	"net/netip"
	"net/url"
	"regexp"
	"strings"
)

var (
	urlRe   = regexp.MustCompile("(?i)\\b(?:https?|ftp)://[^\\s<>\"'`)\\]}]+")
	emailRe = regexp.MustCompile(`(?i)[a-z0-9._%+\-]+@[a-z0-9.\-]+\.[a-z]{2,}`)
	ipv4Re  = regexp.MustCompile(`\b(?:\d{1,3}\.){3}\d{1,3}\b`)
	ipv6Re  = regexp.MustCompile(`(?i)\b(?:[0-9a-f]{0,4}:){2,7}[0-9a-f]{0,4}\b`)
)

// URLs 返回文本里所有合法 URL（含重复，保持出现顺序）。
func URLs(text string) []string {
	return collect(urlRe.FindAllString(text, -1), func(s string) (string, bool) {
		s = strings.TrimRight(s, `.,;:!?'"`)
		u, err := url.Parse(s)
		if err != nil || u.Host == "" {
			return "", false
		}
		return s, true
	})
}

// Emails 返回文本里所有合法邮箱（含重复，保持出现顺序）。
func Emails(text string) []string {
	return collect(emailRe.FindAllString(text, -1), func(s string) (string, bool) {
		a, err := mail.ParseAddress(s)
		if err != nil {
			return "", false
		}
		return a.Address, true
	})
}

// IPs 返回文本里所有合法 IP（IPv4 + IPv6，含重复；先 v4 后 v6）。归一化形式（netip.String）。
func IPs(text string) []string {
	cands := ipv4Re.FindAllString(text, -1)
	cands = append(cands, ipv6Re.FindAllString(text, -1)...)
	return collect(cands, func(s string) (string, bool) {
		addr, err := netip.ParseAddr(s)
		if err != nil {
			return "", false
		}
		return addr.String(), true
	})
}

// Dedup 去重并保留首次出现顺序。
func Dedup(in []string) []string {
	seen := make(map[string]struct{}, len(in))
	out := make([]string, 0, len(in))
	for _, s := range in {
		if _, ok := seen[s]; ok {
			continue
		}
		seen[s] = struct{}{}
		out = append(out, s)
	}
	return out
}

// collect 对每个候选跑校验器，留下通过的（归一化值），保持顺序、含重复。
func collect(cands []string, validate func(string) (string, bool)) []string {
	out := make([]string, 0, len(cands))
	for _, c := range cands {
		if v, ok := validate(c); ok {
			out = append(out, v)
		}
	}
	return out
}
