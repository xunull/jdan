package cli

import (
	"archive/tar"
	"archive/zip"
	"bytes"
	"compress/gzip"
	"encoding/json"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func makeTestZip(t *testing.T, dir string, entries map[string]string) string {
	t.Helper()
	path := filepath.Join(dir, "test.zip")
	buf := &bytes.Buffer{}
	w := zip.NewWriter(buf)
	for name, content := range entries {
		f, _ := w.Create(name)
		_, _ = f.Write([]byte(content))
	}
	_ = w.Close()
	_ = os.WriteFile(path, buf.Bytes(), 0o644)
	return path
}

func makeTestTarGz(t *testing.T, dir string, entries map[string]string) string {
	t.Helper()
	path := filepath.Join(dir, "test.tar.gz")
	var buf bytes.Buffer
	gz := gzip.NewWriter(&buf)
	tw := tar.NewWriter(gz)
	for name, content := range entries {
		_ = tw.WriteHeader(&tar.Header{
			Name: name, Mode: 0o644, Size: int64(len(content)), Typeflag: tar.TypeReg,
		})
		_, _ = tw.Write([]byte(content))
	}
	tw.Close()
	gz.Close()
	_ = os.WriteFile(path, buf.Bytes(), 0o644)
	return path
}

func TestExtract_DefaultOutDirIsSubdir(t *testing.T) {
	dir := t.TempDir()
	zipPath := makeTestZip(t, dir, map[string]string{
		"a.txt": "hello",
	})

	cwd, _ := os.Getwd()
	defer os.Chdir(cwd)
	_ = os.Chdir(dir)

	var buf bytes.Buffer
	cmd := newExtractCommand(extractCmdDeps{out: &buf})
	cmd.SetArgs([]string{zipPath})
	if err := cmd.Execute(); err != nil {
		t.Fatal(err)
	}
	// 默认解压到 test/ 子目录（archive name 是 test.zip，去后缀 test）
	want := filepath.Join(dir, "test", "a.txt")
	if _, err := os.Stat(want); err != nil {
		t.Errorf("expected file at %s: %v", want, err)
	}
}

func TestExtract_OutputFlag(t *testing.T) {
	dir := t.TempDir()
	zipPath := makeTestZip(t, dir, map[string]string{
		"hi.txt": "yo",
	})
	outDir := filepath.Join(dir, "custom-out")

	var buf bytes.Buffer
	cmd := newExtractCommand(extractCmdDeps{out: &buf})
	cmd.SetArgs([]string{zipPath, "-o", outDir})
	if err := cmd.Execute(); err != nil {
		t.Fatal(err)
	}
	if _, err := os.Stat(filepath.Join(outDir, "hi.txt")); err != nil {
		t.Errorf("--output not respected: %v", err)
	}
}

func TestExtract_HereFlag(t *testing.T) {
	dir := t.TempDir()
	zipPath := makeTestZip(t, dir, map[string]string{
		"file.txt": "here mode",
	})
	cwd, _ := os.Getwd()
	defer os.Chdir(cwd)
	_ = os.Chdir(dir)

	var buf bytes.Buffer
	cmd := newExtractCommand(extractCmdDeps{out: &buf})
	cmd.SetArgs([]string{zipPath, "--here"})
	if err := cmd.Execute(); err != nil {
		t.Fatal(err)
	}
	// --here 解压到当前目录（不创建子目录）
	if _, err := os.Stat(filepath.Join(dir, "file.txt")); err != nil {
		t.Errorf("--here should put file in cwd: %v", err)
	}
}

func TestExtract_ListMode(t *testing.T) {
	dir := t.TempDir()
	zipPath := makeTestZip(t, dir, map[string]string{
		"a.txt": "111",
		"b.bin": "22",
	})

	var buf bytes.Buffer
	cmd := newExtractCommand(extractCmdDeps{out: &buf})
	cmd.SetArgs([]string{zipPath, "--list"})
	if err := cmd.Execute(); err != nil {
		t.Fatal(err)
	}
	out := buf.String()
	for _, want := range []string{"a.txt", "b.bin", "archive:"} {
		if !strings.Contains(out, want) {
			t.Errorf("--list output missing %q:\n%s", want, out)
		}
	}
	// 不该真的写文件
	if _, err := os.Stat(filepath.Join(dir, "test", "a.txt")); err == nil {
		t.Error("--list should not actually extract files")
	}
}

func TestExtract_ListJSON(t *testing.T) {
	dir := t.TempDir()
	zipPath := makeTestZip(t, dir, map[string]string{
		"x.txt": "data",
	})
	var buf bytes.Buffer
	cmd := newExtractCommand(extractCmdDeps{out: &buf})
	cmd.SetArgs([]string{zipPath, "--list", "--json"})
	if err := cmd.Execute(); err != nil {
		t.Fatal(err)
	}
	var got map[string]interface{}
	if err := json.Unmarshal(buf.Bytes(), &got); err != nil {
		t.Fatalf("invalid JSON: %v\n%s", err, buf.String())
	}
	if got["archive"] != zipPath {
		t.Errorf("archive: %v", got["archive"])
	}
}

func TestExtract_TarGz(t *testing.T) {
	dir := t.TempDir()
	tgzPath := makeTestTarGz(t, dir, map[string]string{
		"sub/file.txt": "tar.gz content",
	})
	outDir := filepath.Join(dir, "out")
	cmd := newExtractCommand(extractCmdDeps{out: &bytes.Buffer{}})
	cmd.SetArgs([]string{tgzPath, "-o", outDir})
	if err := cmd.Execute(); err != nil {
		t.Fatal(err)
	}
	body, _ := os.ReadFile(filepath.Join(outDir, "sub/file.txt"))
	if string(body) != "tar.gz content" {
		t.Errorf("content lost: %q", body)
	}
}

func TestExtract_MissingArchive_Errors(t *testing.T) {
	cmd := newExtractCommand(extractCmdDeps{out: &bytes.Buffer{}})
	cmd.SetArgs([]string{"/definitely/not/exist.zip"})
	if err := cmd.Execute(); err == nil {
		t.Error("missing archive should error")
	}
}

func TestExtract_UnknownFormat_Errors(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "file.unknown")
	_ = os.WriteFile(path, []byte("x"), 0o644)
	cmd := newExtractCommand(extractCmdDeps{out: &bytes.Buffer{}})
	cmd.SetArgs([]string{path})
	if err := cmd.Execute(); err == nil {
		t.Error("unknown format should error")
	}
}

func TestHumanSize(t *testing.T) {
	for _, tc := range []struct {
		in   int64
		want string
	}{
		{0, "-"},
		{500, "500B"},
		{2048, "2.0KB"},
		{1024 * 1024 * 5, "5.0MB"},
	} {
		if got := humanSize(tc.in); got != tc.want {
			t.Errorf("humanSize(%d) = %q, want %q", tc.in, got, tc.want)
		}
	}
}
