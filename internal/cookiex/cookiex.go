// Package cookiex 解析 Set-Cookie / Cookie 头并做安全体检（纯函数）。
//
// 解析直接复用 stdlib http.ParseSetCookie / http.ParseCookie（Go 1.23+），这里只在上面加
// 审计层：缺 Secure / HttpOnly、SameSite=None 无 Secure、__Host-/__Secure- 前缀规则、过宽 Domain。
package cookiex

import (
	"net/http"
	"strings"
)

// Issue 是一条体检结果。
type Issue struct {
	Level string `json:"level"` // warn | info
	Msg   string `json:"msg"`
}

// ParseSetCookie 包一层 stdlib，解析一条 Set-Cookie 行。
func ParseSetCookie(line string) (*http.Cookie, error) {
	return http.ParseSetCookie(line)
}

// ParseCookie 解析一条请求 Cookie 头（仅 name=value 对，无属性）。
func ParseCookie(line string) ([]*http.Cookie, error) {
	return http.ParseCookie(line)
}

// SameSiteName 把 enum 转可读名。
func SameSiteName(s http.SameSite) string {
	switch s {
	case http.SameSiteDefaultMode:
		return "Default"
	case http.SameSiteLaxMode:
		return "Lax"
	case http.SameSiteStrictMode:
		return "Strict"
	case http.SameSiteNoneMode:
		return "None"
	default:
		return "(未设置)"
	}
}

// Audit 体检一条已解析的 Set-Cookie。
func Audit(c *http.Cookie) []Issue {
	var out []Issue

	if !c.Secure {
		out = append(out, Issue{"warn", "缺 Secure → 可能经明文 HTTP 传输被窃听"})
	}
	if !c.HttpOnly {
		out = append(out, Issue{"warn", "缺 HttpOnly → JS 可读，XSS 能偷 cookie"})
	}

	switch c.SameSite {
	case http.SameSiteNoneMode:
		if !c.Secure {
			out = append(out, Issue{"warn", "SameSite=None 必须配 Secure，否则浏览器拒收"})
		}
	case http.SameSiteLaxMode, http.SameSiteStrictMode, http.SameSiteDefaultMode:
		// ok
	default: // 0 = 未设置
		out = append(out, Issue{"info", "未设 SameSite（现代浏览器默认 Lax，建议显式声明）"})
	}

	switch {
	case strings.HasPrefix(c.Name, "__Host-"):
		if !c.Secure || c.Path != "/" || c.Domain != "" {
			out = append(out, Issue{"warn", "__Host- 前缀要求 Secure + Path=/ + 不设 Domain，当前不满足"})
		}
	case strings.HasPrefix(c.Name, "__Secure-"):
		if !c.Secure {
			out = append(out, Issue{"warn", "__Secure- 前缀要求 Secure，当前缺失"})
		}
	}

	if strings.HasPrefix(c.Domain, ".") {
		out = append(out, Issue{"info", "Domain 以 . 开头 → 作用于所有子域，范围偏大"})
	}

	return out
}
