package cspx

import (
	"strings"
	"testing"
)

func TestParse(t *testing.T) {
	p := Parse("default-src 'self'; script-src 'self' 'unsafe-inline'")
	if len(p.Directives) != 2 {
		t.Fatalf("应解析出 2 条指令，got %d", len(p.Directives))
	}
	d, ok := p.Get("script-src")
	if !ok || len(d.Sources) != 2 || d.Sources[1] != "'unsafe-inline'" {
		t.Errorf("script-src 解析错: %+v", d)
	}
}

func auditMsgs(p Policy) string {
	var b strings.Builder
	for _, is := range Audit(p) {
		b.WriteString(is.Directive + ":" + is.Msg + "\n")
	}
	return b.String()
}

func TestAudit_Weak(t *testing.T) {
	msgs := auditMsgs(Parse("script-src 'self' 'unsafe-inline' *; style-src 'unsafe-eval'"))
	for _, want := range []string{"unsafe-inline", "通配", "default-src", "unsafe-eval", "frame-ancestors"} {
		if !strings.Contains(msgs, want) {
			t.Errorf("体检应提到 %q，实际:\n%s", want, msgs)
		}
	}
}

func TestAudit_DataInScript(t *testing.T) {
	msgs := auditMsgs(Parse("default-src 'self'; script-src data:; object-src 'none'; frame-ancestors 'none'"))
	if !strings.Contains(msgs, "data:") {
		t.Errorf("script-src 含 data: 应告警:\n%s", msgs)
	}
}

func TestAudit_Strong(t *testing.T) {
	// 一条没有常见弱点的 CSP
	p := Parse("default-src 'none'; script-src 'self'; object-src 'none'; frame-ancestors 'none'")
	if got := Audit(p); len(got) != 0 {
		t.Errorf("强 CSP 不应有体检项，got %v", got)
	}
}
