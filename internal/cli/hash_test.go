package cli

import (
	"bytes"
	"encoding/json"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

// 经典 test vector：sha256 of "abc" + "abc\n" 文件
const (
	abcSHA256        = "ba7816bf8f01cfea414140de5dae2223b00361a396177a9cb410ff61f20015ad"
	abcNewlineSHA256 = "edeaaff3f1774ad2888673770c6d64097e391bc362d7d6fb34982ddf0efd18cb" // "abc\n"
)

func writeTempFile(t *testing.T, content string) string {
	t.Helper()
	dir := t.TempDir()
	path := filepath.Join(dir, "data.bin")
	if err := os.WriteFile(path, []byte(content), 0o644); err != nil {
		t.Fatal(err)
	}
	return path
}

func TestHash_DefaultSHA256_FileArg(t *testing.T) {
	path := writeTempFile(t, "abc")

	var buf bytes.Buffer
	cmd := newHashCommand(hashCmdDeps{out: &buf})
	cmd.SetArgs([]string{path})
	if err := cmd.Execute(); err != nil {
		t.Fatal(err)
	}
	out := buf.String()
	if !strings.Contains(out, abcSHA256) {
		t.Errorf("missing sha256 of 'abc' in output:\n%s", out)
	}
	// 默认输出格式：`hash  path`
	if !strings.Contains(out, "  "+path) {
		t.Errorf("missing path in output: %s", out)
	}
}

func TestHash_Stdin(t *testing.T) {
	var buf bytes.Buffer
	cmd := newHashCommand(hashCmdDeps{
		out: &buf,
		in:  strings.NewReader("abc"),
	})
	cmd.SetArgs([]string{"-"})
	if err := cmd.Execute(); err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(buf.String(), abcSHA256) {
		t.Errorf("stdin sha256 missing: %s", buf.String())
	}
}

func TestHash_MultiAlgo(t *testing.T) {
	path := writeTempFile(t, "abc")
	var buf bytes.Buffer
	cmd := newHashCommand(hashCmdDeps{out: &buf})
	cmd.SetArgs([]string{path, "--algo", "md5,sha256"})
	if err := cmd.Execute(); err != nil {
		t.Fatal(err)
	}
	out := buf.String()
	if !strings.Contains(out, abcSHA256) {
		t.Errorf("missing sha256:\n%s", out)
	}
	if !strings.Contains(out, "900150983cd24fb0d6963f7d28e17f72") {
		t.Errorf("missing md5:\n%s", out)
	}
}

func TestHash_All(t *testing.T) {
	path := writeTempFile(t, "abc")
	var buf bytes.Buffer
	cmd := newHashCommand(hashCmdDeps{out: &buf})
	cmd.SetArgs([]string{path, "--all"})
	if err := cmd.Execute(); err != nil {
		t.Fatal(err)
	}
	out := buf.String()
	for _, want := range []string{"MD5", "SHA1", "SHA256", "SHA512"} {
		if !strings.Contains(out, want) {
			t.Errorf("--all missing %s section: %s", want, out)
		}
	}
}

func TestHash_JSON(t *testing.T) {
	path := writeTempFile(t, "abc")
	var buf bytes.Buffer
	cmd := newHashCommand(hashCmdDeps{out: &buf})
	cmd.SetArgs([]string{path, "--json"})
	if err := cmd.Execute(); err != nil {
		t.Fatal(err)
	}
	var got map[string]interface{}
	if err := json.Unmarshal(buf.Bytes(), &got); err != nil {
		t.Fatalf("invalid JSON: %v\n%s", err, buf.String())
	}
	if got["path"] != path {
		t.Errorf("path: %v", got["path"])
	}
	sums := got["sums"].(map[string]interface{})
	if sums["sha256"] != abcSHA256 {
		t.Errorf("sha256: %v", sums["sha256"])
	}
}

func TestHash_InvalidAlgo_Errors(t *testing.T) {
	path := writeTempFile(t, "x")
	cmd := newHashCommand(hashCmdDeps{out: &bytes.Buffer{}})
	cmd.SetArgs([]string{path, "--algo", "sha3"})
	if err := cmd.Execute(); err == nil {
		t.Error("unknown algo should error")
	}
}

func TestHash_NoArgs_Errors(t *testing.T) {
	cmd := newHashCommand(hashCmdDeps{
		out: &bytes.Buffer{},
		in:  strings.NewReader("x"),
	})
	cmd.SetArgs([]string{})
	if err := cmd.Execute(); err == nil {
		t.Error("no file should error")
	}
}

func TestHash_MissingFile_Errors(t *testing.T) {
	cmd := newHashCommand(hashCmdDeps{out: &bytes.Buffer{}})
	cmd.SetArgs([]string{"/definitely/not/exist.bin"})
	if err := cmd.Execute(); err == nil {
		t.Error("missing file should error")
	}
}

// --check 模式测试

func writeChecksumFile(t *testing.T, dir string, entries [][]string) string {
	t.Helper()
	var lines []string
	for _, e := range entries {
		lines = append(lines, e[0]+"  "+e[1])
	}
	path := filepath.Join(dir, "checksums.txt")
	if err := os.WriteFile(path, []byte(strings.Join(lines, "\n")+"\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	return path
}

func TestHashCheck_AllOK(t *testing.T) {
	dir := t.TempDir()
	for _, name := range []string{"a.txt", "b.bin"} {
		if err := os.WriteFile(filepath.Join(dir, name), []byte("abc"), 0o644); err != nil {
			t.Fatal(err)
		}
	}
	checksums := writeChecksumFile(t, dir, [][]string{
		{abcSHA256, "a.txt"},
		{abcSHA256, "b.bin"},
	})

	var buf bytes.Buffer
	cmd := newHashCommand(hashCmdDeps{out: &buf})
	cmd.SetArgs([]string{"--check", checksums})
	if err := cmd.Execute(); err != nil {
		t.Errorf("all-OK check should not error: %v", err)
	}
	out := buf.String()
	for _, want := range []string{"a.txt: OK", "b.bin: OK", "2 total, 0 failed"} {
		if !strings.Contains(out, want) {
			t.Errorf("missing %q in output: %s", want, out)
		}
	}
}

func TestHashCheck_OneFails(t *testing.T) {
	dir := t.TempDir()
	_ = os.WriteFile(filepath.Join(dir, "good.txt"), []byte("abc"), 0o644)
	_ = os.WriteFile(filepath.Join(dir, "bad.txt"), []byte("xyz"), 0o644)
	checksums := writeChecksumFile(t, dir, [][]string{
		{abcSHA256, "good.txt"},
		{abcSHA256, "bad.txt"}, // hash 不匹配 'xyz'
	})

	var buf bytes.Buffer
	cmd := newHashCommand(hashCmdDeps{out: &buf})
	cmd.SetArgs([]string{"--check", checksums})
	err := cmd.Execute()
	if err == nil {
		t.Error("failed check should error (exit 1)")
	}
	if _, ok := err.(*hashCmdExitErr); !ok {
		t.Errorf("expected hashCmdExitErr, got %T", err)
	}
	out := buf.String()
	if !strings.Contains(out, "good.txt: OK") {
		t.Errorf("missing OK: %s", out)
	}
	if !strings.Contains(out, "bad.txt: FAILED") {
		t.Errorf("missing FAILED: %s", out)
	}
}

func TestHashCheck_MissingFile(t *testing.T) {
	dir := t.TempDir()
	checksums := writeChecksumFile(t, dir, [][]string{
		{abcSHA256, "nonexistent.txt"},
	})
	cmd := newHashCommand(hashCmdDeps{out: &bytes.Buffer{}})
	cmd.SetArgs([]string{"--check", checksums})
	if err := cmd.Execute(); err == nil {
		t.Error("missing file should fail")
	}
}

func TestHashCheck_AutoDetectAlgoByLength(t *testing.T) {
	// 用 md5 hash (32 chars) — 应当自动选 md5
	dir := t.TempDir()
	_ = os.WriteFile(filepath.Join(dir, "a.txt"), []byte("abc"), 0o644)
	checksums := writeChecksumFile(t, dir, [][]string{
		{"900150983cd24fb0d6963f7d28e17f72", "a.txt"}, // md5 of "abc"
	})
	var buf bytes.Buffer
	cmd := newHashCommand(hashCmdDeps{out: &buf})
	cmd.SetArgs([]string{"--check", checksums})
	if err := cmd.Execute(); err != nil {
		t.Errorf("md5 auto-detect should pass: %v\n%s", err, buf.String())
	}
}
