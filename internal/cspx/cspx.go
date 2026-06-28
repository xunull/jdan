// Package cspx 解析 Content-Security-Policy 头并做安全体检（纯函数，0 依赖）。
//
// CSP 是分号分隔的指令列表，每条 "指令名 来源1 来源2…"。解析的价值不在拆开，在揪弱点：
// unsafe-inline / unsafe-eval / 通配 * / 缺 default-src / 缺 object-src 'none' 等。
package cspx

import "strings"

// Directive 是一条 CSP 指令。
type Directive struct {
	Name    string   `json:"name"`
	Sources []string `json:"sources"`
}

// Policy 是整条 CSP。
type Policy struct {
	Directives []Directive `json:"directives"`
}

// Get 按名取指令（名字大小写不敏感，已在 Parse 时转小写）。
func (p Policy) Get(name string) (Directive, bool) {
	for _, d := range p.Directives {
		if d.Name == name {
			return d, true
		}
	}
	return Directive{}, false
}

// Parse 把 CSP 头值拆成指令列表。
func Parse(value string) Policy {
	var p Policy
	for seg := range strings.SplitSeq(value, ";") {
		fields := strings.Fields(seg)
		if len(fields) == 0 {
			continue
		}
		p.Directives = append(p.Directives, Directive{
			Name:    strings.ToLower(fields[0]),
			Sources: fields[1:],
		})
	}
	return p
}

// Issue 是一条体检结果。
type Issue struct {
	Level     string `json:"level"` // warn | info
	Directive string `json:"directive"`
	Msg       string `json:"msg"`
}

// Audit 体检一条 CSP，返回弱点列表（高价值子集，非全套 CSP-Evaluator）。
func Audit(p Policy) []Issue {
	var out []Issue
	_, hasDefault := p.Get("default-src")

	if !hasDefault {
		out = append(out, Issue{"warn", "default-src", "缺 default-src 兜底 → 未声明的资源类型不受限"})
	}

	// 脚本/默认/样式源里的危险来源
	for _, name := range []string{"default-src", "script-src", "style-src"} {
		d, ok := p.Get(name)
		if !ok {
			continue
		}
		for _, s := range d.Sources {
			switch strings.ToLower(s) {
			case "'unsafe-inline'":
				out = append(out, Issue{"warn", name, "含 'unsafe-inline' → 内联脚本/样式可执行，CSP 几乎失效"})
			case "'unsafe-eval'":
				out = append(out, Issue{"warn", name, "含 'unsafe-eval' → eval() 可用，XSS 防护削弱"})
			case "*":
				out = append(out, Issue{"warn", name, "含 * 通配 → 允许任意来源，等于没限制"})
			case "data:":
				if name != "style-src" {
					out = append(out, Issue{"warn", name, "脚本源含 data: → 可注入 data:URI 脚本"})
				}
			}
		}
	}

	// object-src 'none'（禁插件注入）
	if d, ok := p.Get("object-src"); ok {
		if !(len(d.Sources) == 1 && strings.EqualFold(d.Sources[0], "'none'")) {
			out = append(out, Issue{"info", "object-src", "object-src 建议设为 'none'（禁用 Flash/插件注入）"})
		}
	} else if !hasDefault {
		out = append(out, Issue{"info", "object-src", "缺 object-src 且无 default-src 兜底 → 插件来源不受限"})
	}

	// frame-ancestors（防点击劫持）
	if _, ok := p.Get("frame-ancestors"); !ok {
		out = append(out, Issue{"info", "frame-ancestors", "缺 frame-ancestors → 建议加（防点击劫持，比 X-Frame-Options 强）"})
	}

	return out
}
