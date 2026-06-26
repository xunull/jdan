package gitx

import (
	"encoding/json"
	"fmt"
	"regexp"
	"strings"
)

// Commit 是一条解析过的提交。Type 为空表示 subject 不符合 Conventional Commits。
type Commit struct {
	Hash     string
	Type     string // feat / fix / ...（小写）；非规范时为空
	Scope    string
	Subject  string // 描述（规范时去掉 type: 前缀，否则为完整 subject）
	Breaking bool
}

// Changelog 是某个范围内的提交集合。
type Changelog struct {
	From    string // 解析后的起点（tag / commit；空表示从头）
	To      string // 终点（默认 HEAD）
	Commits []Commit
}

// ccRe 解析 Conventional Commit：type(scope)!: desc
var ccRe = regexp.MustCompile(`^(\w+)(?:\(([^)]*)\))?(!)?:\s*(.+)$`)

func parseCommit(hash, subject, body string) Commit {
	subject = strings.TrimSpace(subject)
	c := Commit{Hash: hash, Subject: subject}
	if m := ccRe.FindStringSubmatch(subject); m != nil {
		c.Type = strings.ToLower(m[1])
		c.Scope = m[2]
		c.Subject = m[4]
		if m[3] == "!" {
			c.Breaking = true
		}
	}
	up := strings.ToUpper(body)
	if strings.Contains(up, "BREAKING CHANGE") || strings.Contains(up, "BREAKING-CHANGE") {
		c.Breaking = true
	}
	return c
}

// latestTag 返回 to 可达的最近 tag；无 tag 返回空串。
func latestTag(run Runner, dir, to string) string {
	out, err := run(dir, "describe", "--tags", "--abbrev=0", to)
	if err != nil {
		return ""
	}
	return strings.TrimSpace(out)
}

// BuildChangelog 收集 from..to 范围内的提交并解析。
// from 空 → 取最近 tag（再无 tag 则全部历史）；to 空 → HEAD。
func BuildChangelog(run Runner, dir, from, to string) (Changelog, error) {
	if !IsRepo(run, dir) {
		return Changelog{}, fmt.Errorf("不是 git 仓库（试试 git init）")
	}
	if to == "" {
		to = "HEAD"
	}
	if from == "" {
		from = latestTag(run, dir, to)
	}
	rangeArg := to
	if from != "" {
		rangeArg = from + ".." + to
	}
	out, err := run(dir, "log", rangeArg, "--no-merges", "--format=%H%x1f%s%x1f%b%x1e")
	if err != nil {
		return Changelog{}, err
	}

	cl := Changelog{From: from, To: to}
	for rec := range strings.SplitSeq(out, "\x1e") {
		rec = strings.Trim(rec, "\n")
		if strings.TrimSpace(rec) == "" {
			continue
		}
		parts := strings.SplitN(rec, "\x1f", 3)
		if len(parts) < 2 {
			continue
		}
		body := ""
		if len(parts) > 2 {
			body = parts[2]
		}
		cl.Commits = append(cl.Commits, parseCommit(parts[0], parts[1], body))
	}
	return cl, nil
}

type clSection struct{ key, heading string }

// 固定的「已知类型 → 段落」映射与顺序。其余类型 + 非规范提交归到 Other。
var clSections = []clSection{
	{"feat", "Features"},
	{"fix", "Bug Fixes"},
	{"perf", "Performance"},
	{"refactor", "Refactoring"},
	{"docs", "Documentation"},
}

func isKnownType(t string) bool {
	for _, s := range clSections {
		if s.key == t {
			return true
		}
	}
	return false
}

// Markdown 渲染成 changelog（markdown）。breaking 单独拎出、不重复进类型段。
func (cl Changelog) Markdown() string {
	var b strings.Builder

	heading := "未发布"
	if cl.To != "HEAD" {
		heading = cl.To
	}
	rangeDesc := "全部历史"
	if cl.From != "" {
		rangeDesc = "自 " + cl.From
	}
	fmt.Fprintf(&b, "## %s (%s)\n", heading, rangeDesc)

	if len(cl.Commits) == 0 {
		b.WriteString("\n_(无提交)_\n")
		return b.String()
	}

	var breaking, other []Commit
	byType := map[string][]Commit{}
	for _, c := range cl.Commits {
		switch {
		case c.Breaking:
			breaking = append(breaking, c)
		case isKnownType(c.Type):
			byType[c.Type] = append(byType[c.Type], c)
		default:
			other = append(other, c)
		}
	}

	writeSection := func(title string, cs []Commit) {
		if len(cs) == 0 {
			return
		}
		fmt.Fprintf(&b, "\n### %s\n", title)
		for _, c := range cs {
			b.WriteString("- " + commitLine(c) + "\n")
		}
	}
	writeSection("⚠ Breaking Changes", breaking)
	for _, s := range clSections {
		writeSection(s.heading, byType[s.key])
	}
	writeSection("Other", other)
	return b.String()
}

func commitLine(c Commit) string {
	if c.Scope != "" {
		return "(" + c.Scope + ") " + c.Subject
	}
	return c.Subject
}

type commitJSON struct {
	Hash     string `json:"hash"`
	Type     string `json:"type,omitempty"`
	Scope    string `json:"scope,omitempty"`
	Subject  string `json:"subject"`
	Breaking bool   `json:"breaking"`
}

// JSON 渲染成结构化输出。
func (cl Changelog) JSON() (string, error) {
	out := struct {
		From    string       `json:"from"`
		To      string       `json:"to"`
		Commits []commitJSON `json:"commits"`
	}{From: cl.From, To: cl.To, Commits: []commitJSON{}}
	for _, c := range cl.Commits {
		out.Commits = append(out.Commits, commitJSON{c.Hash, c.Type, c.Scope, c.Subject, c.Breaking})
	}
	data, err := json.MarshalIndent(out, "", "  ")
	if err != nil {
		return "", err
	}
	return string(data), nil
}
