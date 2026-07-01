// Package gitleaksx 解析 gitleaks 的 JSON 报告，并补一层「敏感文件名」审计，
// 供 jdan git secrets 使用。
//
// 检测本身交给 gitleaks —— 选它就是不重造规则引擎。本包只做三件事：
//  1. 解析 gitleaks --report-format json 的报告为结构化命中；
//  2. 对「历史里曾新增过的路径」做文件名匹配，补 gitleaks 的盲区（内容无特征
//     的凭据文件，如加密 keystore、全低熵的 .env）；
//  3. 统一渲染（文本 / JSON）。
//
// 安全：脱敏在跑 gitleaks 时用 --redact=100 强制（Secret/Match 已是 REDACTED）；
// 本包只忠实呈现拿到的字段，默认不含明文。0 新依赖。
package gitleaksx

import (
	"encoding/json"
	"fmt"
	"regexp"
	"sort"
	"strings"
)

// ContentFinding 是 gitleaks 报出的一处内容命中。Secret 仅在显式 --show-secrets
// 时才含明文，默认是 gitleaks 脱敏后的占位（REDACTED）。
type ContentFinding struct {
	Rule   string `json:"rule"`
	File   string `json:"file"`
	Line   int    `json:"line"`
	Commit string `json:"commit"`
	Author string `json:"author"`
	Date   string `json:"date"`
	Secret string `json:"secret"`
}

// FileFinding 是文件名审计的一处命中（只看文件名，内容未验证）。
type FileFinding struct {
	Path string `json:"path"`
	Kind string `json:"kind"`
}

// Result 汇总内容层（gitleaks）与文件名层（jdan 自查）。
type Result struct {
	Mode    string           `json:"mode"` // "history" / "staged"
	Content []ContentFinding `json:"content"`
	Files   []FileFinding    `json:"files"`
}

// Detected 报告是否有任意一层命中。
func (r Result) Detected() bool { return len(r.Content) > 0 || len(r.Files) > 0 }

// rawFinding 对齐 gitleaks JSON 报告的字段（PascalCase，encoding/json 大小写不敏感
// 匹配，只取用得上的几项）。
type rawFinding struct {
	RuleID    string
	File      string
	StartLine int
	Commit    string
	Author    string
	Date      string
	Secret    string
}

// ParseReport 解析 gitleaks 的 JSON 报告。空/空白输入 → 无命中（gitleaks 无泄露时
// 报告为空数组或空串）。
func ParseReport(data []byte) ([]ContentFinding, error) {
	if strings.TrimSpace(string(data)) == "" {
		return nil, nil
	}
	var raw []rawFinding
	if err := json.Unmarshal(data, &raw); err != nil {
		return nil, fmt.Errorf("解析 gitleaks 报告失败：%w", err)
	}
	out := make([]ContentFinding, 0, len(raw))
	for _, r := range raw {
		out = append(out, ContentFinding{
			Rule:   r.RuleID,
			File:   r.File,
			Line:   r.StartLine,
			Commit: shortSHA(r.Commit),
			Author: r.Author,
			Date:   shortDate(r.Date),
			Secret: r.Secret,
		})
	}
	return out, nil
}

func shortSHA(s string) string {
	if len(s) > 8 {
		return s[:8]
	}
	return s
}

func shortDate(s string) string {
	if i := strings.IndexByte(s, 'T'); i > 0 {
		return s[:i]
	}
	return s
}

// ---- 文件名审计 ----

type fileRule struct {
	re   *regexp.Regexp
	kind string
}

var fileRules = buildFileRules()

func buildFileRules() []fileRule {
	type spec struct{ pat, kind string }
	specs := []spec{
		{`(?i)(^|/)\.env(\.[^/]+)?$`, "环境变量文件"},
		{`(?i)\.pem$`, "PEM 文件（私钥/证书）"},
		{`(?i)\.key$`, "密钥文件"},
		{`(?i)\.(p12|pfx)$`, "PKCS#12 证书库"},
		{`(?i)(^|/)id_(rsa|dsa|ecdsa|ed25519)$`, "SSH 私钥"},
		{`(?i)\.(keystore|jks)$`, "Java 密钥库"},
		{`(?i)\.ppk$`, "PuTTY 私钥"},
		{`(?i)(^|/)\.npmrc$`, "npm 凭据"},
		{`(?i)(^|/)\.pypirc$`, "PyPI 凭据"},
		{`(?i)(^|/)\.netrc$`, "netrc 凭据"},
		{`(?i)(^|/)\.?htpasswd$`, "htpasswd 口令库"},
		{`(?i)service[-_]?account.*\.json$`, "服务账号密钥"},
		{`(?i)(^|/)credentials$`, "凭据文件"},
		{`(?i)aws.*credentials`, "AWS 凭据"},
		{`(?i)(^|/)\.kube/config$`, "kubeconfig"},
	}
	out := make([]fileRule, 0, len(specs))
	for _, s := range specs {
		out = append(out, fileRule{re: regexp.MustCompile(s.pat), kind: s.kind})
	}
	return out
}

// AuditFilenames 对一批曾出现过的路径做文件名匹配（去重、排序、纯函数）。
// 每条路径命中第一条规则即计入；只看文件名，不读内容。
func AuditFilenames(paths []string) []FileFinding {
	seen := map[string]bool{}
	var out []FileFinding
	for _, p := range paths {
		p = strings.TrimSpace(p)
		if p == "" || seen[p] {
			continue
		}
		seen[p] = true
		for _, r := range fileRules {
			if r.re.MatchString(p) {
				out = append(out, FileFinding{Path: p, Kind: r.kind})
				break
			}
		}
	}
	sort.Slice(out, func(i, j int) bool { return out[i].Path < out[j].Path })
	return out
}

// ---- 渲染 ----

// Render 渲染成人类可读文本（不含汇总行，汇总由调用方写到 stderr）。
func (r Result) Render() string {
	var b strings.Builder
	for _, f := range r.Content {
		who := strings.TrimSpace(f.Author + " " + f.Date)
		fmt.Fprintf(&b, "[%s] %s:%d  %s  %s  (%s)  %s\n",
			r.Mode, f.File, f.Line, f.Rule, f.Commit, who, f.Secret)
	}
	if len(r.Files) > 0 {
		b.WriteString("\n疑似敏感文件（仅文件名，内容未验证）：\n")
		for _, f := range r.Files {
			fmt.Fprintf(&b, "  · %s  [%s]\n", f.Path, f.Kind)
		}
	}
	return b.String()
}

// FormatJSON 输出结构化结果（空数组而非 null，附 detected 便于机读判断）。
func (r Result) FormatJSON() (string, error) {
	if r.Content == nil {
		r.Content = []ContentFinding{}
	}
	if r.Files == nil {
		r.Files = []FileFinding{}
	}
	out := struct {
		Result
		Detected bool `json:"detected"`
	}{Result: r, Detected: r.Detected()}
	data, err := json.MarshalIndent(out, "", "  ")
	if err != nil {
		return "", err
	}
	return string(data), nil
}
