// Package jwtdecode 解析 JWT 的 header 和 payload 段。
//
// 设计取舍：
//   - 不引入 jwt 库（github.com/golang-jwt/jwt 等），避免暴露 secret/key API
//     表面让用户误以为本工具会做签名验证。本包**只 decode，不 verify**。
//   - 不发起任何网络请求（不会 fetch jwks_uri、不查 Issuer 元数据）。
//   - 严格遵守 JWT base64url 规则（无 padding，只用 url-safe 字母表）。
//
// 验签是单独的能力，应当由 jdan jwt verify 子命令处理（带 --key、--jwks-url 等
// 显式参数），不应混在 decode 里。
package jwtdecode

import (
	"encoding/base64"
	"encoding/json"
	"errors"
	"fmt"
	"strings"
	"time"
)

// Result 是 Decode 的输出。Header 和 Payload 是原始 JSON 字符串（已 pretty-print），
// 适合直接打印；HeaderMap / PayloadMap 是解析后的 map，方便结构化访问。
type Result struct {
	Header        string         `json:"header"`
	Payload       string         `json:"payload"`
	HeaderMap     map[string]any `json:"header_map"`
	PayloadMap    map[string]any `json:"payload_map"`
	Signature     string         `json:"signature"`     // 原始 base64url，未解码
	Algorithm     string         `json:"alg"`           // header.alg
	KeyID         string         `json:"kid,omitempty"` // header.kid
	IssuedAt      *time.Time     `json:"issued_at,omitempty"`
	NotBefore     *time.Time     `json:"not_before,omitempty"`
	ExpiresAt     *time.Time     `json:"expires_at,omitempty"`
	Expired       bool           `json:"expired"`
	TimeRemaining string         `json:"time_remaining,omitempty"` // 人类可读，仅未过期时填充
	Subject       string         `json:"sub,omitempty"`
	Issuer        string         `json:"iss,omitempty"`
	Audience      []string       `json:"aud,omitempty"`
	Extra         map[string]any `json:"-"` // 留作未来扩展
}

// Decode 解码 token 字符串。**不**验证签名。
//
// 输入要求：标准 JWT 三段格式 header.payload.signature，每段为 base64url（无 padding）。
//
// 错误场景：
//   - token 不是 3 段 → "not a valid JWT structure"
//   - header / payload base64url decode 失败 → "invalid base64url in <header|payload>"
//   - header / payload 不是有效 JSON → "<header|payload> is not valid JSON"
//
// 时间字段（iat/nbf/exp）按 RFC 7519 解读为 NumericDate（unix 秒）。non-numeric 或负值
// 会被忽略，不当作错误（行为与 jwt.io 一致）。
func Decode(token string) (*Result, error) {
	token = strings.TrimSpace(token)
	if token == "" {
		return nil, errors.New("empty token")
	}
	parts := strings.Split(token, ".")
	if len(parts) != 3 {
		return nil, fmt.Errorf("not a valid JWT structure: expected 3 dot-separated segments, got %d", len(parts))
	}
	if parts[0] == "" || parts[1] == "" {
		return nil, errors.New("not a valid JWT structure: empty header or payload segment")
	}

	headerJSON, err := decodeSegment(parts[0])
	if err != nil {
		return nil, fmt.Errorf("invalid base64url in header: %w", err)
	}
	payloadJSON, err := decodeSegment(parts[1])
	if err != nil {
		return nil, fmt.Errorf("invalid base64url in payload: %w", err)
	}

	var headerMap, payloadMap map[string]any
	if err := json.Unmarshal(headerJSON, &headerMap); err != nil {
		return nil, fmt.Errorf("header is not valid JSON: %w", err)
	}
	if err := json.Unmarshal(payloadJSON, &payloadMap); err != nil {
		return nil, fmt.Errorf("payload is not valid JSON: %w", err)
	}

	headerPretty, _ := json.MarshalIndent(headerMap, "", "  ")
	payloadPretty, _ := json.MarshalIndent(payloadMap, "", "  ")

	r := &Result{
		Header:     string(headerPretty),
		Payload:    string(payloadPretty),
		HeaderMap:  headerMap,
		PayloadMap: payloadMap,
		Signature:  parts[2],
	}
	enrichFromHeader(r, headerMap)
	enrichFromPayload(r, payloadMap)
	return r, nil
}

// decodeSegment 解码不带 padding 的 base64url。
// RFC 7515 §3 强制 JWT 段必须无 padding；但有些实现会带（错误地），所以我们用 raw 模式
// 但允许 `=` 出现并去掉。
func decodeSegment(s string) ([]byte, error) {
	s = strings.TrimRight(s, "=")
	return base64.RawURLEncoding.DecodeString(s)
}

func enrichFromHeader(r *Result, h map[string]any) {
	if v, ok := h["alg"].(string); ok {
		r.Algorithm = v
	}
	if v, ok := h["kid"].(string); ok {
		r.KeyID = v
	}
}

func enrichFromPayload(r *Result, p map[string]any) {
	now := time.Now()
	if t := parseEpoch(p["iat"]); t != nil {
		r.IssuedAt = t
	}
	if t := parseEpoch(p["nbf"]); t != nil {
		r.NotBefore = t
	}
	if t := parseEpoch(p["exp"]); t != nil {
		r.ExpiresAt = t
		if now.After(*t) {
			r.Expired = true
		} else {
			r.TimeRemaining = humanDuration(t.Sub(now))
		}
	}
	if v, ok := p["sub"].(string); ok {
		r.Subject = v
	}
	if v, ok := p["iss"].(string); ok {
		r.Issuer = v
	}
	// aud 按 RFC 7519 可以是 string 或 []string
	switch v := p["aud"].(type) {
	case string:
		r.Audience = []string{v}
	case []any:
		for _, x := range v {
			if s, ok := x.(string); ok {
				r.Audience = append(r.Audience, s)
			}
		}
	}
}

// parseEpoch 把 JWT 时间字段（应为 NumericDate，秒级 unix 时间）解析成 time.Time。
// JSON unmarshal 会把数字读成 float64；负值或非数字返回 nil（不当作错误）。
func parseEpoch(v any) *time.Time {
	var secs float64
	switch x := v.(type) {
	case float64:
		secs = x
	case json.Number:
		f, err := x.Float64()
		if err != nil {
			return nil
		}
		secs = f
	default:
		return nil
	}
	if secs <= 0 {
		return nil
	}
	t := time.Unix(int64(secs), 0).UTC()
	return &t
}

// humanDuration 把一段时间转成 "3d 4h" / "12m" 这种紧凑写法。
// 用于 jdan jwt decode 显示"还有多久过期"，不需要毫秒精度。
func humanDuration(d time.Duration) string {
	if d < 0 {
		return "expired"
	}
	if d < time.Minute {
		return fmt.Sprintf("%ds", int(d.Seconds()))
	}
	if d < time.Hour {
		return fmt.Sprintf("%dm", int(d.Minutes()))
	}
	if d < 24*time.Hour {
		h := int(d.Hours())
		m := int(d.Minutes()) - h*60
		if m == 0 {
			return fmt.Sprintf("%dh", h)
		}
		return fmt.Sprintf("%dh %dm", h, m)
	}
	days := int(d.Hours()) / 24
	h := int(d.Hours()) - days*24
	if h == 0 {
		return fmt.Sprintf("%dd", days)
	}
	return fmt.Sprintf("%dd %dh", days, h)
}
