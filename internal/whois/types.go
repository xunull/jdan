// Package whois 实现 WHOIS 协议（RFC 3912）+ TLD/IP 路由 + 响应解析。
//
// WHOIS 协议本身极简（TCP:43，发送 query+CRLF，读所有响应），复杂度
// 在路由：每个 TLD 一个 server，IP 跨 RIR 要 referral，且不同 registrar
// 输出 schema 不统一。本包正面回应：路由层做 TLD 映射 + IANA fallback +
// referral 跟随；输出层保留原文（--raw 永远可用）+ 通用 line-prefix parser
// 做 best-effort 字段提取（在 Commit 2 引入）。
package whois

// Result 一次 WHOIS 查询的完整结果。
type Result struct {
	Target  string `json:"target"`         // 用户输入（domain or IP）
	Kind    Kind   `json:"kind"`           // domain / ipv4 / ipv6
	Server  string `json:"server"`         // 最终查询用的 server（跟 referral 后）
	Hops    []Hop  `json:"hops,omitempty"` // referral 链（IP 跨 RIR / IANA root → TLD server）
	RawText string `json:"raw"`            // 最终 server 的原始响应文本
}

// Hop 是 referral 链中的一跳。中间跳的 raw 一般不展示（避免吵），
// 但 server 名记录下来便于 debug。
type Hop struct {
	Server string `json:"server"`
}

// Kind 表征 target 类型。
type Kind string

const (
	KindDomain Kind = "domain"
	KindIPv4   Kind = "ipv4"
	KindIPv6   Kind = "ipv6"
)
