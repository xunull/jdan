// Package commitlint 按 Conventional Commits 规范校验提交信息。
//
// 规范的 header：  <type>(<scope>)<!>: <subject>
// 之后是可选的空行 + body，再空行 + footer（如 BREAKING CHANGE: …）。
//
// 本包是纯函数、不碰 git：Parse 把一条信息拆成结构，Lint 逐规则查。
// 取 git 提交信息（range 模式）由 CLI 用 gitx.Runner 负责。0 新依赖（纯 stdlib）。
package commitlint

import (
	"fmt"
	"regexp"
	"slices"
	"strings"
	"unicode/utf8"
)

// DefaultMaxHeader 是 header 长度默认上限（按 rune 计），对齐 commitlint 默认值。
const DefaultMaxHeader = 100

// DefaultTypes 是 Conventional Commits / @commitlint/config-conventional 的默认 type 白名单。
func DefaultTypes() []string {
	return []string{"feat", "fix", "docs", "style", "refactor", "perf", "test", "build", "ci", "chore", "revert"}
}

// Options 控制校验行为。零值即采用默认（白名单 DefaultTypes、上限 DefaultMaxHeader）。
type Options struct {
	Types         []string // 允许的 type；空 = DefaultTypes
	MaxHeaderLen  int       // header 上限（rune）；<=0 = DefaultMaxHeader
	ScopeRequired bool      // 是否强制要有 scope
}

// Commit 是一条提交信息解析后的结构。
type Commit struct {
	Header   string `json:"header"`
	Type     string `json:"type,omitempty"`
	Scope    string `json:"scope,omitempty"`
	Subject  string `json:"subject,omitempty"`
	Breaking bool   `json:"breaking"`
	Parsed   bool   `json:"parsed"` // header 是否符合 type(scope): subject 结构

	secondLineBlank bool
	hasBody         bool
}

// Violation 是一条规则违规。
type Violation struct {
	Rule string `json:"rule"`
	Msg  string `json:"message"`
}

// headerRe 匹配 "type(scope)!: subject"。type 允许大写以便单独报 type-case；
// 冒号后强制一个空格（Conventional Commits 要求 ": "）；subject 允许为空以便报 subject-empty。
var headerRe = regexp.MustCompile(`^([A-Za-z]+)(?:\(([^)]*)\))?(!)?: (.*)$`)

// Parse 把一条原始提交信息拆成结构。会先剥掉 git 注释行（# 开头）与 verbose 模式的
// scissors（"# ---- >8 ----"）之后的 diff，再解析第一行 header。
func Parse(raw string) Commit {
	body := clean(raw)
	var c Commit
	if body == "" {
		return c
	}
	lines := strings.Split(body, "\n")
	c.Header = lines[0]
	if m := headerRe.FindStringSubmatch(c.Header); m != nil {
		c.Parsed = true
		c.Type = m[1]
		c.Scope = m[2]
		c.Breaking = m[3] == "!"
		c.Subject = m[4]
	}
	if len(lines) > 1 {
		c.secondLineBlank = strings.TrimSpace(lines[1]) == ""
		c.hasBody = strings.TrimSpace(strings.Join(lines[1:], "\n")) != ""
	}
	if strings.Contains(body, "BREAKING CHANGE:") || strings.Contains(body, "BREAKING-CHANGE:") {
		c.Breaking = true
	}
	return c
}

// Lint 对解析结果按规则查，返回违规列表（空 = 合规）。
func Lint(c Commit, opts Options) []Violation {
	types := opts.Types
	if len(types) == 0 {
		types = DefaultTypes()
	}
	maxLen := opts.MaxHeaderLen
	if maxLen <= 0 {
		maxLen = DefaultMaxHeader
	}

	var vs []Violation
	add := func(rule, msg string) { vs = append(vs, Violation{Rule: rule, Msg: msg}) }

	if c.Header == "" {
		add("header-empty", "提交信息为空")
		return vs
	}

	if !c.Parsed {
		add("header-structure", fmt.Sprintf("header 不符合 \"type(scope): subject\" 结构：%q", c.Header))
	} else {
		if c.Type == "" {
			add("type-empty", "缺少 type")
		} else {
			if c.Type != strings.ToLower(c.Type) {
				add("type-case", fmt.Sprintf("type %q 必须小写", c.Type))
			}
			if !slices.Contains(types, strings.ToLower(c.Type)) {
				add("type-enum", fmt.Sprintf("type %q 不在白名单 %s", c.Type, strings.Join(types, ",")))
			}
		}
		if c.Scope != "" && c.Scope != strings.ToLower(c.Scope) {
			add("scope-case", fmt.Sprintf("scope %q 必须小写", c.Scope))
		}
		if opts.ScopeRequired && c.Scope == "" {
			add("scope-empty", "缺少 scope（--scope-required）")
		}
		if strings.TrimSpace(c.Subject) == "" {
			add("subject-empty", "缺少 subject")
		} else if strings.HasSuffix(strings.TrimRight(c.Subject, " "), ".") {
			add("subject-full-stop", "subject 结尾不应有句号")
		}
	}

	if n := utf8.RuneCountInString(c.Header); n > maxLen {
		add("header-max-length", fmt.Sprintf("header %d 字符，超过上限 %d", n, maxLen))
	}
	if c.hasBody && !c.secondLineBlank {
		add("body-leading-blank", "header 与 body 之间需空一行")
	}
	return vs
}

// clean 去掉 git 注释行与 verbose scissors 之后的内容，并 trim 首尾空行。
func clean(msg string) string {
	msg = strings.ReplaceAll(msg, "\r\n", "\n")
	var out []string
	for _, l := range strings.Split(msg, "\n") {
		if strings.HasPrefix(l, "# ") && strings.Contains(l, ">8") {
			break // scissors：之后是 diff，整段丢弃
		}
		if strings.HasPrefix(l, "#") {
			continue // 注释行
		}
		out = append(out, l)
	}
	return strings.Trim(strings.Join(out, "\n"), "\n")
}
