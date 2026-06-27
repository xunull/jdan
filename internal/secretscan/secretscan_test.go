package secretscan

import (
	"fmt"
	"strings"
	"testing"
)

// synth 拼出「形状像真密钥」的测试夹具。源码里前缀与主体是分开的字面量，运行时才
// 拼成完整串——这样文件里不存在连续的「真密钥形状」，GitHub push protection / 各家
// secret scanner（以及本工具自己）都不会把测试夹具误当真密钥拦截。
func synth(parts ...string) string { return strings.Join(parts, "") }

func TestScanBytes_PatternRules(t *testing.T) {
	cases := []struct {
		line string
		rule string
	}{
		{"AWS_KEY=" + synth("AKIA", "1234567890ABCDEF"), "aws-access-key"},
		{"tok=" + synth("ghp_", "abcdefghijklmnopqrstuvwxyz0123456789"), "github-pat"},
		{"gl=" + synth("glpat-", "aBcD1234eFgH5678iJkL"), "gitlab-pat"},
		{"stripe=" + synth("sk_", "live_abcdefghijklmnopqrstuvwx"), "stripe-secret"},
		{"g=" + synth("AIza", "SyA1234567890abcdefghijklmnopqrstuv"), "google-api-key"},
		{"-----BEGIN RSA PRIVATE KEY-----", "private-key"},
		{"url=https://user:" + "supersecret" + "@example.com", "url-basic-auth"},
		{`api_key = "` + synth("Ab1Cd2Ef3Gh4", "Ij5Kl6") + `"`, "generic-assign"},
	}
	for _, c := range cases {
		got := ScanBytes([]byte(c.line), Options{NoEntropy: true})
		found := false
		for _, f := range got {
			if f.Rule == c.rule {
				found = true
			}
		}
		if !found {
			t.Errorf("规则 %s 未命中 %q，得到 %+v", c.rule, c.line, got)
		}
	}
}

// 安全命门：脱敏后绝不含完整 secret。
func TestScanBytes_NeverLeaksFullSecret(t *testing.T) {
	secrets := []string{
		synth("AKIA", "1234567890ABCDEF"),
		synth("ghp_", "abcdefghijklmnopqrstuvwxyz0123456789"),
		synth("sk_", "live_abcdefghijklmnopqrstuvwx"),
		synth("Xy9Kf3pQ2mNvB8w", "LrT4uZ1aShjEoP6dC"),
	}
	var sb strings.Builder
	for i, s := range secrets {
		fmt.Fprintf(&sb, "k%d = \"%s\"\n", i, s)
		fmt.Fprintf(&sb, "r%d=%s\n", i, s)
	}
	findings := ScanBytes([]byte(sb.String()), Options{})
	if len(findings) == 0 {
		t.Fatal("应有命中")
	}
	for _, f := range findings {
		for _, s := range secrets {
			if strings.Contains(f.Redacted, s) {
				t.Errorf("脱敏后仍含完整 secret %q（Redacted=%q）", s, f.Redacted)
			}
		}
	}
}

func TestRedact(t *testing.T) {
	if r := Redact(synth("AKIA", "1234567890ABCDEF")); r != "AKIA…CDEF" {
		t.Errorf("Redact = %q, want AKIA…CDEF", r)
	}
	// 太短 → 整体打码，不露原文
	short := Redact("abc123")
	if strings.Contains(short, "abc") {
		t.Errorf("短串应整体打码，得到 %q", short)
	}
}

func TestAllowlist(t *testing.T) {
	// AWS 文档示例不报
	if got := ScanBytes([]byte("k="+synth("AKIAIOSFODNN7", "EXAMPLE")), Options{NoEntropy: true}); len(got) != 0 {
		t.Errorf("AWS 示例占位应被 allowlist，得到 %+v", got)
	}
	// UUID 不被高熵误报
	if got := ScanBytes([]byte(`id=550e8400-e29b-41d4-a716-446655440000`), Options{}); len(got) != 0 {
		t.Errorf("UUID 不应命中，得到 %+v", got)
	}
	// changeme 占位不报
	if got := ScanBytes([]byte(`password = "changeme"`), Options{NoEntropy: true}); len(got) != 0 {
		t.Errorf("changeme 占位不应命中，得到 %+v", got)
	}
}

func TestInlinePragma(t *testing.T) {
	line := "tok=" + synth("ghp_", "abcdefghijklmnopqrstuvwxyz0123456789") + " # pragma: allowlist secret"
	if got := ScanBytes([]byte(line), Options{}); len(got) != 0 {
		t.Errorf("pragma 豁免行不应命中，得到 %+v", got)
	}
}

func TestGenericAssign_SkipsWeak(t *testing.T) {
	// 全小写无数字 → 多半占位，不报
	if got := ScanBytes([]byte(`password = "iloveyou"`), Options{NoEntropy: true}); len(got) != 0 {
		t.Errorf("弱口令占位不应命中，得到 %+v", got)
	}
}

func TestEntropyEngine(t *testing.T) {
	// 高熵随机 base64 → 命中，低置信
	blob := "blob=" + synth("dGhpc2lzYVZlcnlS", "YW5kb20xMjM0NTY3ODkwQWJD")
	hi := ScanBytes([]byte(blob), Options{})
	found := false
	for _, f := range hi {
		if f.Rule == "high-entropy" && f.Confidence == Low {
			found = true
		}
	}
	if !found {
		t.Errorf("高熵串应命中（low）：%+v", hi)
	}
	// 纯单词（无数字）不命中高熵
	if got := ScanBytes([]byte(`note=thisisjustalonglowercasesentencewithnodigits`), Options{}); len(got) != 0 {
		t.Errorf("纯单词不应高熵命中，得到 %+v", got)
	}
	// --no-entropy 关闭后高熵串不报
	if got := ScanBytes([]byte(blob), Options{NoEntropy: true}); len(got) != 0 {
		t.Errorf("--no-entropy 应关闭高熵引擎，得到 %+v", got)
	}
}

func TestEntropyDedup(t *testing.T) {
	// 已被正则命中的 token 不应再被高熵重复报
	got := ScanBytes([]byte("GITHUB_TOKEN="+synth("ghp_", "abcdefghijklmnopqrstuvwxyz0123456789")), Options{})
	n := 0
	for _, f := range got {
		if f.Line == 1 {
			n++
		}
	}
	if n != 1 {
		t.Errorf("同一 secret 应只报一次，得到 %d：%+v", n, got)
	}
}
