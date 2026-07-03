// Package sechdrx 给一组 HTTP 响应头的「安全性」打分（securityheaders.com 风格）：
// 看核心安全头有没有、配得好不好，给字母等级 A+~F + 分项 pass/warn/fail + 修复建议。
//
// 纯函数、纯 stdlib、0 新依赖。只评估传进来的响应头，不发请求、不做任何主动探测。
package sechdrx

import (
	"encoding/json"
	"fmt"
	"net/http"
	"regexp"
	"strconv"
	"strings"
)

// Check 状态。
const (
	Pass = "pass"
	Warn = "warn"
	Fail = "fail"
	Info = "info"
)

// Check 是一项检查结果。
type Check struct {
	Header string `json:"header"`
	Status string `json:"status"` // pass/warn/fail/info
	Detail string `json:"detail"`
	Advice string `json:"advice,omitempty"`
}

// Report 是完整评级。
type Report struct {
	URL    string  `json:"url,omitempty"`
	Grade  string  `json:"grade"` // A+ A B C D F
	Score  int     `json:"score"` // 0..100
	Checks []Check `json:"checks"`
}

// Options 控制评分。
type Options struct {
	Strict bool // 把 COOP/COEP/CORP 跨源隔离纳入评级（默认只提示不扣分）
}

// HSTS 合格的最小 max-age（180 天，和主流实践一致）。
const hstsMinMaxAge = 15552000

var maxAgeRe = regexp.MustCompile(`(?i)max-age\s*=\s*(\d+)`)
var versionRe = regexp.MustCompile(`\d+\.\d`)

// Grade 给一组响应头打分。isHTTPS 表示最终响应是否走 HTTPS（影响 HSTS 判定）。
func Grade(h http.Header, isHTTPS bool, opts Options) Report {
	var checks []Check
	score := 0

	// 核心 6 项（满分合计 100）。
	c, p := gradeHSTS(h.Get("Strict-Transport-Security"), isHTTPS)
	checks = append(checks, c)
	score += p

	c, p = gradeCSP(h.Get("Content-Security-Policy"))
	checks = append(checks, c)
	score += p
	cspHasFrameAncestors := strings.Contains(strings.ToLower(h.Get("Content-Security-Policy")), "frame-ancestors")

	c, p = gradeNosniff(h.Get("X-Content-Type-Options"))
	checks = append(checks, c)
	score += p

	c, p = gradeFrame(h.Get("X-Frame-Options"), cspHasFrameAncestors)
	checks = append(checks, c)
	score += p

	c, p = gradeReferrer(h.Get("Referrer-Policy"))
	checks = append(checks, c)
	score += p

	c, p = gradePermissions(h.Get("Permissions-Policy"), h.Get("Feature-Policy"))
	checks = append(checks, c)
	score += p

	// 跨源隔离 COOP/COEP/CORP：默认只提示（Info，不计分）；--strict 时缺失各扣 5。
	for _, x := range []struct{ name, val string }{
		{"Cross-Origin-Opener-Policy", h.Get("Cross-Origin-Opener-Policy")},
		{"Cross-Origin-Embedder-Policy", h.Get("Cross-Origin-Embedder-Policy")},
		{"Cross-Origin-Resource-Policy", h.Get("Cross-Origin-Resource-Policy")},
	} {
		checks = append(checks, gradeCrossOrigin(x.name, x.val, opts.Strict, &score))
	}

	// 信息泄露头：反向扣分（不低于 0）。
	checks = append(checks, leakServer(h.Get("Server"), &score))
	if v := h.Get("X-Powered-By"); v != "" {
		score -= 5
		checks = append(checks, Check{"X-Powered-By", Warn, v + "（暴露技术栈）", "去掉 X-Powered-By 响应头"})
	}
	for _, name := range []string{"X-AspNet-Version", "X-AspNetMvc-Version"} {
		if v := h.Get(name); v != "" {
			score -= 3
			checks = append(checks, Check{name, Warn, v + "（暴露框架版本）", "去掉 " + name + " 响应头"})
		}
	}

	if score < 0 {
		score = 0
	}
	if score > 100 {
		score = 100
	}
	return Report{Grade: gradeLetter(score), Score: score, Checks: checks}
}

func gradeHSTS(v string, isHTTPS bool) (Check, int) {
	const name = "Strict-Transport-Security"
	if !isHTTPS {
		return Check{name, Fail, "站点未走 HTTPS，HSTS 无意义", "先启用 HTTPS，再加 HSTS"}, 0
	}
	if v == "" {
		return Check{name, Fail, "缺失", "加 Strict-Transport-Security: max-age=31536000; includeSubDomains"}, 0
	}
	low := strings.ToLower(v)
	maxAge := 0
	if m := maxAgeRe.FindStringSubmatch(low); m != nil {
		maxAge, _ = strconv.Atoi(m[1])
	}
	subdomains := strings.Contains(low, "includesubdomains")
	switch {
	case maxAge >= hstsMinMaxAge && subdomains:
		return Check{name, Pass, v, ""}, 20
	case maxAge >= hstsMinMaxAge:
		return Check{name, Warn, v + "（建议加 includeSubDomains）", "加 includeSubDomains 覆盖子域"}, 15
	case maxAge > 0:
		return Check{name, Warn, v + "（max-age 太短，<180 天）", "把 max-age 提到至少 15552000（180 天）"}, 10
	default:
		return Check{name, Fail, v + "（无有效 max-age）", "设 max-age=31536000"}, 0
	}
}

func gradeCSP(v string) (Check, int) {
	const name = "Content-Security-Policy"
	if v == "" {
		return Check{name, Fail, "缺失", "加 Content-Security-Policy 限制资源来源（至少 default-src 'self'）"}, 0
	}
	low := strings.ToLower(v)
	var weak []string
	if strings.Contains(low, "unsafe-inline") {
		weak = append(weak, "unsafe-inline")
	}
	if strings.Contains(low, "unsafe-eval") {
		weak = append(weak, "unsafe-eval")
	}
	if len(weak) > 0 {
		return Check{name, Warn, "含 " + strings.Join(weak, "/") + "（削弱了防护，等于给内联脚本开口子）",
			"去掉 " + strings.Join(weak, "/") + "，改用 nonce/hash"}, 12
	}
	return Check{name, Pass, truncate(v, 80), ""}, 25
}

func gradeNosniff(v string) (Check, int) {
	const name = "X-Content-Type-Options"
	if strings.EqualFold(strings.TrimSpace(v), "nosniff") {
		return Check{name, Pass, "nosniff", ""}, 15
	}
	if v == "" {
		return Check{name, Fail, "缺失", "加 X-Content-Type-Options: nosniff"}, 0
	}
	return Check{name, Warn, v + "（应为 nosniff）", "设 X-Content-Type-Options: nosniff"}, 0
}

func gradeFrame(v string, cspFrameAncestors bool) (Check, int) {
	const name = "X-Frame-Options"
	if cspFrameAncestors {
		return Check{name, Pass, "由 CSP frame-ancestors 覆盖", ""}, 15
	}
	switch strings.ToUpper(strings.TrimSpace(v)) {
	case "DENY", "SAMEORIGIN":
		return Check{name, Pass, v, ""}, 15
	case "":
		return Check{name, Fail, "缺失", "加 X-Frame-Options: SAMEORIGIN（或 CSP frame-ancestors）防点击劫持"}, 0
	default:
		return Check{name, Warn, v + "（非标准/已废弃）", "改用 DENY 或 SAMEORIGIN"}, 7
	}
}

func gradeReferrer(v string) (Check, int) {
	const name = "Referrer-Policy"
	if v == "" {
		return Check{name, Fail, "缺失", "加 Referrer-Policy: strict-origin-when-cross-origin"}, 0
	}
	if strings.Contains(strings.ToLower(v), "unsafe-url") {
		return Check{name, Warn, v + "（unsafe-url 会把完整 URL 泄给第三方）", "改用 strict-origin-when-cross-origin"}, 6
	}
	return Check{name, Pass, v, ""}, 12
}

func gradePermissions(v, legacy string) (Check, int) {
	const name = "Permissions-Policy"
	if v != "" {
		return Check{name, Pass, truncate(v, 80), ""}, 13
	}
	if legacy != "" {
		return Check{name, Warn, "只有已废弃的 Feature-Policy", "迁移到 Permissions-Policy"}, 7
	}
	return Check{name, Fail, "缺失", "加 Permissions-Policy 关掉不用的浏览器特性（camera=(), geolocation=() 等）"}, 0
}

func gradeCrossOrigin(name, v string, strict bool, score *int) Check {
	if v != "" {
		return Check{name, Pass, v, ""}
	}
	if strict {
		*score -= 5
		return Check{name, Fail, "缺失（--strict 已计入）", "加 " + name + " 以启用跨源隔离"}
	}
	return Check{name, Info, "缺失（跨源隔离，默认不计入评级）", ""}
}

func leakServer(v string, score *int) Check {
	const name = "Server"
	if v == "" {
		return Check{name, Pass, "未暴露", ""}
	}
	if versionRe.MatchString(v) {
		*score -= 5
		return Check{name, Warn, v + "（暴露了版本号，方便攻击者按版本找 CVE）", "隐藏版本号（nginx server_tokens off / Apache ServerTokens Prod）"}
	}
	return Check{name, Info, v + "（未带版本号）", ""}
}

func gradeLetter(score int) string {
	switch {
	case score >= 95:
		return "A+"
	case score >= 85:
		return "A"
	case score >= 70:
		return "B"
	case score >= 55:
		return "C"
	case score >= 40:
		return "D"
	default:
		return "F"
	}
}

// Rank 把字母等级映射成序（越大越好），供 --fail-under 阈值比较。
func Rank(grade string) int {
	switch strings.ToUpper(strings.TrimSpace(grade)) {
	case "A+":
		return 6
	case "A":
		return 5
	case "B":
		return 4
	case "C":
		return 3
	case "D":
		return 2
	case "F":
		return 1
	default:
		return 0
	}
}

func truncate(s string, n int) string {
	r := []rune(s)
	if len(r) <= n {
		return s
	}
	return string(r[:n]) + "…"
}

var statusMark = map[string]string{Pass: "✓", Warn: "⚠", Fail: "✗", Info: "·"}

// Render 渲染成人类可读文本。
func (r Report) Render() string {
	var b strings.Builder
	head := "安全响应头评级：" + r.Grade + fmt.Sprintf(" (%d/100)", r.Score)
	if r.URL != "" {
		head += "  " + r.URL
	}
	b.WriteString(head + "\n\n")

	// 先核心/信息泄露（非 Info），再把纯 Info（跨源隔离等）归到末尾提示。
	var scored, infos []Check
	for _, c := range r.Checks {
		if c.Status == Info {
			infos = append(infos, c)
		} else {
			scored = append(scored, c)
		}
	}
	for _, c := range scored {
		fmt.Fprintf(&b, "%s %-28s %s\n", statusMark[c.Status], c.Header, c.Detail)
		if c.Advice != "" {
			fmt.Fprintf(&b, "  %-28s 建议：%s\n", "", c.Advice)
		}
	}
	if len(infos) > 0 {
		b.WriteString("\n提示（不计入评级）：\n")
		for _, c := range infos {
			fmt.Fprintf(&b, "%s %-28s %s\n", statusMark[c.Status], c.Header, c.Detail)
		}
	}
	return b.String()
}

// FormatJSON 输出结构化结果（空 checks → []）。
func (r Report) FormatJSON() (string, error) {
	if r.Checks == nil {
		r.Checks = []Check{}
	}
	data, err := json.MarshalIndent(r, "", "  ")
	if err != nil {
		return "", err
	}
	return string(data), nil
}
