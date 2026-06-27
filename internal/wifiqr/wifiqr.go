// Package wifiqr 构造 WiFi 入网二维码的标准 payload（WIFI: 格式）。
//
//	WIFI:T:<auth>;S:<ssid>;P:<password>;H:<hidden>;;
//
// 核心是把 SSID / 密码里的特殊字符 `\ ; , " :` 正确反斜杠转义——手搓 payload 最容易
// 漏这一步，漏了二维码就是错的、手机扫了静默入不了网。纯函数，不依赖渲染。
package wifiqr

import (
	"fmt"
	"strings"
)

// Auth 是 WiFi 认证类型。
type Auth string

const (
	AuthWPA    Auth = "WPA"    // 含 WPA / WPA2 / WPA3（de-facto 标准无单独 WPA3 token）
	AuthWEP    Auth = "WEP"    // 老旧 WEP
	AuthNopass Auth = "nopass" // 开放网络，无密码
)

// ParseAuth 把用户输入（大小写随意）解析成 Auth。
func ParseAuth(s string) (Auth, error) {
	switch strings.ToLower(strings.TrimSpace(s)) {
	case "wpa", "wpa2", "wpa3", "":
		return AuthWPA, nil
	case "wep":
		return AuthWEP, nil
	case "nopass", "open", "none":
		return AuthNopass, nil
	default:
		return "", fmt.Errorf("未知认证类型 %q（用 wpa / wep / nopass）", s)
	}
}

// Config 描述一个 WiFi 网络。
type Config struct {
	SSID     string
	Password string
	Auth     Auth
	Hidden   bool
}

// Payload 按 WIFI: 标准拼出 payload 字符串，SSID/密码已转义。
func Payload(c Config) (string, error) {
	if strings.TrimSpace(c.SSID) == "" {
		return "", fmt.Errorf("SSID 不能为空")
	}
	auth := c.Auth
	if auth == "" {
		auth = AuthWPA
	}

	var b strings.Builder
	b.WriteString("WIFI:")
	b.WriteString("T:")
	b.WriteString(string(auth))
	b.WriteString(";S:")
	b.WriteString(escape(c.SSID))
	b.WriteString(";")
	if auth != AuthNopass {
		// 开放网络省略 P:；WPA/WEP 即便空密码也写出 P: 以保持字段完整
		b.WriteString("P:")
		b.WriteString(escape(c.Password))
		b.WriteString(";")
	}
	if c.Hidden {
		b.WriteString("H:true;")
	}
	b.WriteString(";")
	return b.String(), nil
}

// escape 反斜杠转义 WIFI: payload 中的保留字符：`\ ; , " :`。
func escape(s string) string {
	var b strings.Builder
	b.Grow(len(s))
	for _, r := range s {
		switch r {
		case '\\', ';', ',', '"', ':':
			b.WriteByte('\\')
		}
		b.WriteRune(r)
	}
	return b.String()
}
