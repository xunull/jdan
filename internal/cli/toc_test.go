package cli

import (
	"bytes"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func runTocCmd(t *testing.T, in []byte, args ...string) (string, error) {
	t.Helper()
	var buf bytes.Buffer
	deps := tocCmdDeps{out: &buf}
	if in != nil {
		deps.in = bytes.NewReader(in)
	}
	cmd := newTocCommand(deps)
	cmd.SetArgs(args)
	err := cmd.Execute()
	return buf.String(), err
}

func TestTocCmd_Stdout(t *testing.T) {
	dir := t.TempDir()
	p := filepath.Join(dir, "doc.md")
	os.WriteFile(p, []byte("# Title\n## Setup\n### Detail\n"), 0o644)
	out, err := runTocCmd(t, nil, p)
	if err != nil {
		t.Fatal(err)
	}
	// 默认 min=2，跳过 h1 标题
	if strings.Contains(out, "Title") {
		t.Errorf("default min=2 should skip h1:\n%s", out)
	}
	if !strings.Contains(out, "- [Setup](#setup)") || !strings.Contains(out, "  - [Detail](#detail)") {
		t.Errorf("output wrong:\n%s", out)
	}
}

func TestTocCmd_MinMax(t *testing.T) {
	dir := t.TempDir()
	p := filepath.Join(dir, "doc.md")
	os.WriteFile(p, []byte("# A\n## B\n### C\n#### D\n"), 0o644)
	out, err := runTocCmd(t, nil, p, "--min", "1", "--max", "2")
	if err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(out, "[A](#a)") || !strings.Contains(out, "[B](#b)") {
		t.Errorf("min=1 max=2 should include A,B:\n%s", out)
	}
	if strings.Contains(out, "[C]") || strings.Contains(out, "[D]") {
		t.Errorf("min=1 max=2 should exclude C,D:\n%s", out)
	}
}

func TestTocCmd_Stdin(t *testing.T) {
	out, err := runTocCmd(t, []byte("## Hello World\n"))
	if err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(out, "[Hello World](#hello-world)") {
		t.Errorf("stdin output wrong:\n%s", out)
	}
}

func TestTocCmd_Inplace(t *testing.T) {
	dir := t.TempDir()
	p := filepath.Join(dir, "doc.md")
	original := "# Doc\n\n<!-- toc -->\nOLD\n<!-- /toc -->\n\n## Setup\n## Usage\n"
	os.WriteFile(p, []byte(original), 0o644)

	if _, err := runTocCmd(t, nil, p, "--inplace"); err != nil {
		t.Fatal(err)
	}
	got, _ := os.ReadFile(p)
	s := string(got)
	if strings.Contains(s, "OLD") {
		t.Error("inplace should replace OLD")
	}
	if !strings.Contains(s, "- [Setup](#setup)") || !strings.Contains(s, "- [Usage](#usage)") {
		t.Errorf("inplace TOC wrong:\n%s", s)
	}
	// 标记仍在
	if !strings.Contains(s, "<!-- toc -->") || !strings.Contains(s, "<!-- /toc -->") {
		t.Error("markers should remain")
	}
}

func TestTocCmd_InplaceMissingMarkers(t *testing.T) {
	dir := t.TempDir()
	p := filepath.Join(dir, "doc.md")
	os.WriteFile(p, []byte("## Setup\n"), 0o644)
	if _, err := runTocCmd(t, nil, p, "--inplace"); err == nil {
		t.Error("missing markers should error")
	}
}

func TestTocCmd_InplaceNeedsFile(t *testing.T) {
	// stdin + --inplace 不允许
	if _, err := runTocCmd(t, []byte("## X\n"), "--inplace"); err == nil {
		t.Error("--inplace without a file should error")
	}
}

func TestTocCmd_FileNotFound(t *testing.T) {
	if _, err := runTocCmd(t, nil, "/no/such/file.md"); err == nil {
		t.Error("missing file should error")
	}
}

func TestTocCmd_BadLevels(t *testing.T) {
	dir := t.TempDir()
	p := filepath.Join(dir, "doc.md")
	os.WriteFile(p, []byte("## X\n"), 0o644)
	if _, err := runTocCmd(t, nil, p, "--min", "3", "--max", "2"); err == nil {
		t.Error("min>max should error")
	}
	if _, err := runTocCmd(t, nil, p, "--max", "9"); err == nil {
		t.Error("max>6 should error")
	}
}
