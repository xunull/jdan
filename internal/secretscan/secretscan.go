// Package secretscan 扫文本里疑似硬编码的密钥/token：正则引擎（已知格式，高精度）
// + 高熵引擎（未知 token，复用 entropyx）。0 新依赖（regexp stdlib + entropyx）。
//
// 安全铁律：输出永不含完整 secret，只给脱敏预览（前 4…后 4）。降噪靠 allowlist、
// 行内 pragma 豁免、保守的高熵阈值（高熵命中标低置信度）。
package secretscan

import (
	"regexp"
	"strings"

	"github.com/xunull/jdan/internal/entropyx"
)

// Confidence 级别。
const (
	High   = "high"
	Medium = "medium"
	Low    = "low"
)

// Rule 是一条正则规则。ValueGroup 指明哪个子组是「真正的密钥值」（0 = 整个匹配）。
type Rule struct {
	Name       string
	Re         *regexp.Regexp
	Confidence string
	ValueGroup int
}

// 内嵌规则表：高精度优先。每条 regexp 在 init 编译一次。
var rules = buildRules()

func buildRules() []Rule {
	type spec struct {
		name string
		pat  string
		conf string
		grp  int
	}
	specs := []spec{
		{"aws-access-key", `\b(?:AKIA|ASIA|AGPA|AIDA|AROA|AIPA|ANPA|ANVA)[0-9A-Z]{16}\b`, High, 0},
		{"github-pat", `\bgh[pousr]_[0-9A-Za-z]{36}\b`, High, 0},
		{"github-fine-grained", `\bgithub_pat_[0-9A-Za-z_]{82}\b`, High, 0},
		{"gitlab-pat", `\bglpat-[0-9A-Za-z_-]{20}\b`, High, 0},
		{"slack-token", `\bxox[baprs]-[0-9A-Za-z-]{10,48}\b`, High, 0},
		{"slack-webhook", `https://hooks\.slack\.com/services/[A-Za-z0-9/+_-]+`, High, 0},
		{"stripe-secret", `\b(?:sk|rk)_(?:live|test)_[0-9A-Za-z]{24,}\b`, High, 0},
		{"google-api-key", `\bAIza[0-9A-Za-z_-]{35}\b`, High, 0},
		{"google-oauth-client", `\b[0-9]+-[0-9A-Za-z_]{32}\.apps\.googleusercontent\.com\b`, Medium, 0},
		{"twilio-key", `\bSK[0-9a-fA-F]{32}\b`, Medium, 0},
		{"sendgrid-key", `\bSG\.[0-9A-Za-z_-]{22}\.[0-9A-Za-z_-]{43}\b`, High, 0},
		{"npm-token", `\bnpm_[0-9A-Za-z]{36}\b`, High, 0},
		{"openai-key", `\bsk-(?:proj-)?[A-Za-z0-9_-]{20,}\b`, High, 0},
		{"anthropic-key", `\bsk-ant-[A-Za-z0-9_-]{20,}\b`, High, 0},
		{"private-key", `-----BEGIN (?:RSA |EC |OPENSSH |DSA |PGP |ENCRYPTED )?PRIVATE KEY-----`, High, 0},
		{"jwt", `\beyJ[A-Za-z0-9_-]{10,}\.eyJ[A-Za-z0-9_-]{10,}\.[A-Za-z0-9_-]{10,}\b`, Medium, 0},
		{"url-basic-auth", `\b[a-z][a-z0-9+.-]*://[^/\s:@]+:([^/\s:@]{3,})@`, Medium, 1},
		{"generic-assign", `(?i)\b(?:password|passwd|pwd|secret|secret[_-]?key|token|api[_-]?key|access[_-]?key|auth[_-]?token|client[_-]?secret)\b\s*[:=]\s*["']([^"']{8,})["']`, Medium, 1},
	}
	out := make([]Rule, 0, len(specs))
	for _, s := range specs {
		out = append(out, Rule{Name: s.name, Re: regexp.MustCompile(s.pat), Confidence: s.conf, ValueGroup: s.grp})
	}
	return out
}

// Finding 是一处命中。Redacted 永远是脱敏后的预览，绝不含完整 secret。
type Finding struct {
	Rule       string
	Line       int // 1-based
	Col        int // 1-based 字节列
	Redacted   string
	Entropy    float64 // 0 表示正则命中（非高熵）
	Confidence string
}

// Options 控制扫描。
type Options struct {
	NoEntropy  bool    // 关闭高熵引擎
	MinEntropy float64 // 高熵阈值（bits/byte），0 → 默认 4.0
	MinLen     int     // 高熵 token 最小长度，0 → 默认 20
}

// ScanBytes 逐行扫一段文本，返回命中（纯函数）。
func ScanBytes(data []byte, opts Options) []Finding {
	if opts.MinEntropy == 0 {
		opts.MinEntropy = 4.0
	}
	if opts.MinLen == 0 {
		opts.MinLen = 20
	}

	var out []Finding
	lineNo := 0
	for _, raw := range splitLines(data) {
		lineNo++
		line := string(raw)
		if strings.Contains(line, "pragma: allowlist secret") {
			continue // 行内豁免
		}

		matched := scanPatterns(line, lineNo, &out)
		if !opts.NoEntropy {
			scanEntropy(line, lineNo, matched, opts, &out)
		}
	}
	return out
}

func scanPatterns(line string, lineNo int, out *[]Finding) []string {
	var matched []string
	for _, rule := range rules {
		for _, sm := range rule.Re.FindAllStringSubmatchIndex(line, -1) {
			g := rule.ValueGroup
			vs, ve := sm[2*g], sm[2*g+1]
			if vs < 0 {
				continue
			}
			val := line[vs:ve]
			if isAllowlisted(val) {
				continue
			}
			if rule.Name == "generic-assign" && looksWeak(val) {
				continue // 全小写/无数字的赋值多半不是真密钥
			}
			matched = append(matched, line[sm[0]:sm[1]])
			*out = append(*out, Finding{
				Rule: rule.Name, Line: lineNo, Col: vs + 1,
				Redacted: Redact(val), Confidence: rule.Confidence,
			})
		}
	}
	return matched
}

func scanEntropy(line string, lineNo int, matched []string, opts Options, out *[]Finding) {
	for _, tp := range splitTokens(line) {
		if len(tp.tok) < opts.MinLen || !looksLikeSecret(tp.tok) || isAllowlisted(tp.tok) {
			continue
		}
		if containedIn(matched, tp.tok) {
			continue // 已被正则命中，别重复报
		}
		if e := entropyx.Shannon([]byte(tp.tok)); e >= opts.MinEntropy {
			*out = append(*out, Finding{
				Rule: "high-entropy", Line: lineNo, Col: tp.pos + 1,
				Redacted: Redact(tp.tok), Entropy: e, Confidence: Low,
			})
		}
	}
}

// Redact 把密钥脱敏成「前 4…后 4」；太短则整体打码。绝不返回完整原文。
func Redact(s string) string {
	r := []rune(s)
	if len(r) <= 8 {
		return strings.Repeat("•", len(r))
	}
	return string(r[:4]) + "…" + string(r[len(r)-4:])
}

// ---- 降噪 ----

var allowlistRe = []*regexp.Regexp{
	regexp.MustCompile(`(?i)^[0-9a-f]{8}-[0-9a-f]{4}-[0-9a-f]{4}-[0-9a-f]{4}-[0-9a-f]{12}$`), // UUID
	regexp.MustCompile(`(?i)example|changeme|change[_-]?me|your[_-]?(api[_-]?)?key|placeholder|dummy|sample|test[_-]?key|xxxx+|\.\.\.|<[^>]+>`),
}

var allowlistExact = map[string]bool{
	"AKIAIOSFODNN7EXAMPLE": true, // AWS 文档示例
}

func isAllowlisted(s string) bool {
	if allowlistExact[s] {
		return true
	}
	for _, re := range allowlistRe {
		if re.MatchString(s) {
			return true
		}
	}
	return false
}

// looksWeak：全小写、无数字无大写 → 多半是占位文案而非真密钥。
func looksWeak(s string) bool {
	hasDigit, hasUpper := false, false
	for _, r := range s {
		switch {
		case r >= '0' && r <= '9':
			hasDigit = true
		case r >= 'A' && r <= 'Z':
			hasUpper = true
		}
	}
	return !hasDigit && !hasUpper
}

// looksLikeSecret：高熵候选必须字母+数字都有（排除纯单词、纯数字、版本号、IP）。
func looksLikeSecret(s string) bool {
	hasDigit, hasAlpha := false, false
	for _, r := range s {
		switch {
		case r >= '0' && r <= '9':
			hasDigit = true
		case (r >= 'a' && r <= 'z') || (r >= 'A' && r <= 'Z'):
			hasAlpha = true
		}
	}
	return hasDigit && hasAlpha
}

type tokenPos struct {
	tok string
	pos int
}

func splitTokens(line string) []tokenPos {
	var toks []tokenPos
	start := -1
	for i := 0; i <= len(line); i++ {
		if i < len(line) && isTokenChar(line[i]) {
			if start < 0 {
				start = i
			}
		} else if start >= 0 {
			toks = append(toks, tokenPos{line[start:i], start})
			start = -1
		}
	}
	return toks
}

// '=' 不算 token 字符（它是赋值分隔符；base64 尾部 padding 丢掉不影响熵），
// 否则 KEY=secret 会被并成一个 token，既噪音又绕过与正则命中的去重。
func isTokenChar(b byte) bool {
	return b >= 'A' && b <= 'Z' || b >= 'a' && b <= 'z' || b >= '0' && b <= '9' ||
		b == '+' || b == '/' || b == '_' || b == '-' || b == '.'
}

func containedIn(matched []string, tok string) bool {
	for _, m := range matched {
		if strings.Contains(m, tok) {
			return true
		}
	}
	return false
}

func splitLines(data []byte) [][]byte {
	var out [][]byte
	start := 0
	for i := range data {
		if data[i] == '\n' {
			line := data[start:i]
			if n := len(line); n > 0 && line[n-1] == '\r' {
				line = line[:n-1]
			}
			out = append(out, line)
			start = i + 1
		}
	}
	out = append(out, data[start:])
	return out
}
