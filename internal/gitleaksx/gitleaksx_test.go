package gitleaksx

import (
	"encoding/json"
	"strings"
	"testing"
)

// 一段真实形状的 gitleaks 报告（--redact=100 后 Secret/Match 为 REDACTED）。
const sampleReport = `[
 {
  "RuleID": "private-key",
  "Description": "Identified a Private Key",
  "StartLine": 1,
  "EndLine": 9,
  "Match": "REDACTED",
  "Secret": "REDACTED",
  "File": "internal/sshkey/testdata/ecdsa256",
  "Commit": "24a99a02a82034ba782088545592d84c35ce0c95",
  "Author": "xunull",
  "Email": "xunull@163.com",
  "Date": "2026-06-15T05:33:55Z",
  "Fingerprint": "24a99a02...:private-key:1"
 }
]`

func TestParseReport_Basic(t *testing.T) {
	fs, err := ParseReport([]byte(sampleReport))
	if err != nil {
		t.Fatal(err)
	}
	if len(fs) != 1 {
		t.Fatalf("want 1 finding, got %d", len(fs))
	}
	f := fs[0]
	if f.Rule != "private-key" {
		t.Errorf("rule = %q", f.Rule)
	}
	if f.File != "internal/sshkey/testdata/ecdsa256" {
		t.Errorf("file = %q", f.File)
	}
	if f.Line != 1 {
		t.Errorf("line = %d", f.Line)
	}
	if f.Commit != "24a99a02" { // 短 SHA（前 8）
		t.Errorf("commit = %q, want 24a99a02", f.Commit)
	}
	if f.Date != "2026-06-15" { // 去掉 T 之后的时间
		t.Errorf("date = %q, want 2026-06-15", f.Date)
	}
	if f.Secret != "REDACTED" {
		t.Errorf("secret = %q, want REDACTED", f.Secret)
	}
}

func TestParseReport_EmptyInputs(t *testing.T) {
	for _, in := range []string{"", "   ", "\n", "[]"} {
		fs, err := ParseReport([]byte(in))
		if err != nil {
			t.Errorf("in=%q should not error: %v", in, err)
		}
		if len(fs) != 0 {
			t.Errorf("in=%q should yield no findings, got %d", in, len(fs))
		}
	}
}

func TestParseReport_Invalid(t *testing.T) {
	if _, err := ParseReport([]byte("not json")); err == nil {
		t.Error("invalid JSON should error")
	}
}

func TestParseReport_ShortCommitAndDatePassthrough(t *testing.T) {
	// commit 短于 8、date 无 T：原样返回，不 panic。
	rep := `[{"RuleID":"x","File":"f","StartLine":2,"Commit":"abc","Author":"a","Date":"2026-01-02","Secret":"REDACTED"}]`
	fs, err := ParseReport([]byte(rep))
	if err != nil {
		t.Fatal(err)
	}
	if fs[0].Commit != "abc" || fs[0].Date != "2026-01-02" {
		t.Errorf("%+v", fs[0])
	}
}

func TestAuditFilenames_Kinds(t *testing.T) {
	cases := map[string]string{
		"config/.env":              "环境变量文件",
		"config/.env.production":   "环境变量文件",
		"deploy/id_rsa":            "SSH 私钥",
		"certs/server.pem":         "PEM 文件（私钥/证书）",
		"secret.key":               "密钥文件",
		"keys/app.p12":             "PKCS#12 证书库",
		"store.jks":                "Java 密钥库",
		".npmrc":                   "npm 凭据",
		"home/.netrc":              "netrc 凭据",
		"auth/.htpasswd":           "htpasswd 口令库",
		"gcp/service-account.json": "服务账号密钥",
		".kube/config":             "kubeconfig",
	}
	for path, wantKind := range cases {
		got := AuditFilenames([]string{path})
		if len(got) != 1 {
			t.Errorf("%q: want 1 finding, got %d", path, len(got))
			continue
		}
		if got[0].Kind != wantKind {
			t.Errorf("%q: kind = %q, want %q", path, got[0].Kind, wantKind)
		}
	}
}

func TestAuditFilenames_NonMatches(t *testing.T) {
	// 普通源码/文档不应命中（避免噪音）。
	safe := []string{
		"internal/cli/htpasswd.go", // 不是 htpasswd 结尾
		"README.md",
		"main.go",
		"internal/keyx/keyx.go", // 不是 .key 结尾
		"env.go",
	}
	if got := AuditFilenames(safe); len(got) != 0 {
		t.Errorf("safe paths should not match, got %+v", got)
	}
}

func TestAuditFilenames_DedupeAndSort(t *testing.T) {
	in := []string{"z/.env", "a/id_rsa", "z/.env", "", "  "}
	got := AuditFilenames(in)
	if len(got) != 2 {
		t.Fatalf("want 2 after dedupe/blank-drop, got %d: %+v", len(got), got)
	}
	if got[0].Path != "a/id_rsa" || got[1].Path != "z/.env" {
		t.Errorf("should be sorted by path: %+v", got)
	}
}

func TestResult_Detected(t *testing.T) {
	if (Result{}).Detected() {
		t.Error("empty result should not be detected")
	}
	if !(Result{Content: []ContentFinding{{Rule: "x"}}}).Detected() {
		t.Error("content finding should count")
	}
	if !(Result{Files: []FileFinding{{Path: "x"}}}).Detected() {
		t.Error("file finding should count")
	}
}

func TestRender(t *testing.T) {
	r := Result{
		Mode:    "history",
		Content: []ContentFinding{{Rule: "aws-access-key", File: "a.go", Line: 5, Commit: "deadbeef", Author: "Bob", Date: "2026-01-01", Secret: "REDACTED"}},
		Files:   []FileFinding{{Path: "config/.env", Kind: "环境变量文件"}},
	}
	s := r.Render()
	if !strings.Contains(s, "[history] a.go:5") {
		t.Errorf("content line wrong:\n%s", s)
	}
	if !strings.Contains(s, "aws-access-key") || !strings.Contains(s, "deadbeef") {
		t.Errorf("missing rule/commit:\n%s", s)
	}
	if !strings.Contains(s, "疑似敏感文件") || !strings.Contains(s, "config/.env") {
		t.Errorf("missing filename section:\n%s", s)
	}
}

func TestFormatJSON(t *testing.T) {
	r := Result{Mode: "history", Content: []ContentFinding{{Rule: "x", File: "f", Secret: "REDACTED"}}}
	s, err := r.FormatJSON()
	if err != nil {
		t.Fatal(err)
	}
	var v map[string]any
	if err := json.Unmarshal([]byte(s), &v); err != nil {
		t.Fatalf("invalid json:\n%s", s)
	}
	if v["detected"] != true {
		t.Error("detected should be true")
	}
	if v["mode"] != "history" {
		t.Errorf("mode = %v", v["mode"])
	}
}

func TestFormatJSON_EmptyArraysNotNull(t *testing.T) {
	s, err := (Result{Mode: "history"}).FormatJSON()
	if err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(s, `"content": []`) || !strings.Contains(s, `"files": []`) {
		t.Errorf("empty slices should render as [], not null:\n%s", s)
	}
	if !strings.Contains(s, `"detected": false`) {
		t.Errorf("detected should be false:\n%s", s)
	}
}
