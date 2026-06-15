package cli

import (
	"bytes"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func writeEnv(t *testing.T, name, content string) string {
	t.Helper()
	p := filepath.Join(t.TempDir(), name)
	if err := os.WriteFile(p, []byte(content), 0o644); err != nil {
		t.Fatal(err)
	}
	return p
}

func TestEnvLint_FindsIssues(t *testing.T) {
	p := writeEnv(t, ".env", "KEY=a\nKEY=b\n2FOO=bad\n")
	var buf bytes.Buffer
	cmd := newEnvCommand(envCmdDeps{out: &buf})
	cmd.SetArgs([]string{"lint", p})
	err := cmd.Execute()
	// 有 error（invalid key）→ 非 nil
	if err == nil {
		t.Error("file with error-level issue should return error")
	}
	out := buf.String()
	if !strings.Contains(out, "duplicate key KEY") {
		t.Errorf("missing duplicate warning:\n%s", out)
	}
	if !strings.Contains(out, "invalid key name") {
		t.Errorf("missing invalid key error:\n%s", out)
	}
}

func TestEnvLint_CleanFile(t *testing.T) {
	p := writeEnv(t, ".env", "KEY=value\nFOO=bar\n")
	var buf bytes.Buffer
	cmd := newEnvCommand(envCmdDeps{out: &buf})
	cmd.SetArgs([]string{"lint", p})
	if err := cmd.Execute(); err != nil {
		t.Fatalf("clean file should not error: %v", err)
	}
	if !strings.Contains(buf.String(), "no issues") {
		t.Errorf("got:\n%s", buf.String())
	}
}

func TestEnvLint_StrictFailsOnWarning(t *testing.T) {
	p := writeEnv(t, ".env", "KEY=a\nKEY=b\n") // only warning (duplicate)
	// 非 strict：退出码 0
	cmd1 := newEnvCommand(envCmdDeps{out: &bytes.Buffer{}})
	cmd1.SetArgs([]string{"lint", p})
	if err := cmd1.Execute(); err != nil {
		t.Errorf("non-strict warning should not error: %v", err)
	}
	// strict：退出码 1
	cmd2 := newEnvCommand(envCmdDeps{out: &bytes.Buffer{}})
	cmd2.SetArgs([]string{"lint", p, "--strict"})
	if err := cmd2.Execute(); err == nil {
		t.Error("strict should fail on warning")
	}
}

func TestEnvLint_JSON(t *testing.T) {
	p := writeEnv(t, ".env", "2FOO=bad\n")
	var buf bytes.Buffer
	cmd := newEnvCommand(envCmdDeps{out: &buf})
	cmd.SetArgs([]string{"lint", p, "--json"})
	_ = cmd.Execute()
	out := buf.String()
	for _, want := range []string{`"errors": 1`, `"severity": "error"`, `"issues"`} {
		if !strings.Contains(out, want) {
			t.Errorf("missing %q in:\n%s", want, out)
		}
	}
}

func TestEnvDiff_KeyDifferences(t *testing.T) {
	a := writeEnv(t, "a.env", "SHARED=1\nONLY_A=2\n")
	b := writeEnv(t, "b.env", "SHARED=9\nONLY_B=3\n")
	var buf bytes.Buffer
	cmd := newEnvCommand(envCmdDeps{out: &buf})
	cmd.SetArgs([]string{"diff", a, b})
	if err := cmd.Execute(); err != nil {
		t.Fatal(err)
	}
	out := buf.String()
	if !strings.Contains(out, "+ ONLY_A") {
		t.Errorf("missing only-in-a:\n%s", out)
	}
	if !strings.Contains(out, "- ONLY_B") {
		t.Errorf("missing only-in-b:\n%s", out)
	}
	if !strings.Contains(out, "Common keys: 1") {
		t.Errorf("missing common count:\n%s", out)
	}
	// 默认不比 value，SHARED 不同也不显示
	if strings.Contains(out, "Value differs") {
		t.Errorf("should not show value diff without --values:\n%s", out)
	}
}

func TestEnvDiff_Values(t *testing.T) {
	a := writeEnv(t, "a.env", "KEY=a\n")
	b := writeEnv(t, "b.env", "KEY=b\n")
	var buf bytes.Buffer
	cmd := newEnvCommand(envCmdDeps{out: &buf})
	cmd.SetArgs([]string{"diff", a, b, "--values"})
	if err := cmd.Execute(); err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(buf.String(), "Value differs") {
		t.Errorf("--values should show value diff:\n%s", buf.String())
	}
}

func TestEnvDiff_ExitCode(t *testing.T) {
	a := writeEnv(t, "a.env", "KEY=1\nONLY_A=2\n")
	b := writeEnv(t, "b.env", "KEY=1\n")
	cmd := newEnvCommand(envCmdDeps{out: &bytes.Buffer{}})
	cmd.SetArgs([]string{"diff", a, b, "--exit-code"})
	if err := cmd.Execute(); err == nil {
		t.Error("--exit-code with differences should error")
	}
}

func TestEnvDiff_JSON(t *testing.T) {
	a := writeEnv(t, "a.env", "ONLY_A=2\nSHARED=1\n")
	b := writeEnv(t, "b.env", "SHARED=1\n")
	var buf bytes.Buffer
	cmd := newEnvCommand(envCmdDeps{out: &buf})
	cmd.SetArgs([]string{"diff", a, b, "--json"})
	if err := cmd.Execute(); err != nil {
		t.Fatal(err)
	}
	out := buf.String()
	if !strings.Contains(out, `"only_in_a"`) || !strings.Contains(out, "ONLY_A") {
		t.Errorf("JSON missing only_in_a:\n%s", out)
	}
}

func TestEnvRedact_MasksValues(t *testing.T) {
	p := writeEnv(t, ".env", "export SECRET=verylongsecretvalue\nPORT=8080\n")
	var buf bytes.Buffer
	cmd := newEnvCommand(envCmdDeps{out: &buf})
	cmd.SetArgs([]string{"redact", p})
	if err := cmd.Execute(); err != nil {
		t.Fatal(err)
	}
	out := buf.String()
	if strings.Contains(out, "verylongsecretvalue") {
		t.Errorf("secret not redacted:\n%s", out)
	}
	if !strings.Contains(out, "export SECRET=") {
		t.Errorf("export prefix / key lost:\n%s", out)
	}
}

func TestEnvRedact_Full(t *testing.T) {
	p := writeEnv(t, ".env", "KEY=somevalue\n")
	var buf bytes.Buffer
	cmd := newEnvCommand(envCmdDeps{out: &buf})
	cmd.SetArgs([]string{"redact", p, "--full"})
	if err := cmd.Execute(); err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(buf.String(), "KEY=****") {
		t.Errorf("--full should fully mask:\n%s", buf.String())
	}
}

func TestEnvRedact_KeepShort(t *testing.T) {
	p := writeEnv(t, ".env", "DEBUG=true\nPORT=8080\n")
	var buf bytes.Buffer
	cmd := newEnvCommand(envCmdDeps{out: &buf})
	cmd.SetArgs([]string{"redact", p, "--keep-short"})
	if err := cmd.Execute(); err != nil {
		t.Fatal(err)
	}
	out := buf.String()
	if !strings.Contains(out, "DEBUG=true") {
		t.Errorf("--keep-short should keep boolish:\n%s", out)
	}
	if !strings.Contains(out, "PORT=8080") {
		t.Errorf("--keep-short should keep short:\n%s", out)
	}
}

func TestEnvGet_Found(t *testing.T) {
	p := writeEnv(t, ".env", `DATABASE_URL="postgres://localhost/db"`+"\n")
	var buf bytes.Buffer
	cmd := newEnvCommand(envCmdDeps{out: &buf})
	cmd.SetArgs([]string{"get", p, "DATABASE_URL"})
	if err := cmd.Execute(); err != nil {
		t.Fatal(err)
	}
	if strings.TrimSpace(buf.String()) != "postgres://localhost/db" {
		t.Errorf("got %q", buf.String())
	}
}

func TestEnvGet_NotFound(t *testing.T) {
	p := writeEnv(t, ".env", "KEY=value\n")
	cmd := newEnvCommand(envCmdDeps{out: &bytes.Buffer{}})
	cmd.SetArgs([]string{"get", p, "NOPE"})
	if err := cmd.Execute(); err == nil {
		t.Error("missing key should error")
	}
}

func TestEnvLint_FileNotFound(t *testing.T) {
	cmd := newEnvCommand(envCmdDeps{out: &bytes.Buffer{}})
	cmd.SetArgs([]string{"lint", "/nonexistent/.env"})
	if err := cmd.Execute(); err == nil {
		t.Error("missing file should error")
	}
}
