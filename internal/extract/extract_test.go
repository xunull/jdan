package extract

import (
	"archive/tar"
	"archive/zip"
	"bytes"
	"compress/gzip"
	"errors"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

// ─── archive 构造 helper ────────────────────────────────────────────────

// writeTestZip 在 dir 下创建一个 test.zip，包含给定 entries。
func writeTestZip(t *testing.T, dir string, entries map[string]string) string {
	t.Helper()
	path := filepath.Join(dir, "test.zip")
	buf := &bytes.Buffer{}
	w := zip.NewWriter(buf)
	for name, content := range entries {
		f, err := w.Create(name)
		if err != nil {
			t.Fatal(err)
		}
		if _, err := f.Write([]byte(content)); err != nil {
			t.Fatal(err)
		}
	}
	if err := w.Close(); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(path, buf.Bytes(), 0o644); err != nil {
		t.Fatal(err)
	}
	return path
}

// writeTestTarGz 创建一个 test.tar.gz。
func writeTestTarGz(t *testing.T, dir string, entries map[string]string) string {
	t.Helper()
	path := filepath.Join(dir, "test.tar.gz")
	var buf bytes.Buffer
	gz := gzip.NewWriter(&buf)
	tw := tar.NewWriter(gz)
	for name, content := range entries {
		hdr := &tar.Header{
			Name:     name,
			Mode:     0o644,
			Size:     int64(len(content)),
			Typeflag: tar.TypeReg,
		}
		if err := tw.WriteHeader(hdr); err != nil {
			t.Fatal(err)
		}
		if _, err := tw.Write([]byte(content)); err != nil {
			t.Fatal(err)
		}
	}
	tw.Close()
	gz.Close()
	if err := os.WriteFile(path, buf.Bytes(), 0o644); err != nil {
		t.Fatal(err)
	}
	return path
}

// writeMaliciousZip 创建一个含 `..` traversal 的 zip。
func writeMaliciousZip(t *testing.T, dir string) string {
	t.Helper()
	path := filepath.Join(dir, "evil.zip")
	buf := &bytes.Buffer{}
	w := zip.NewWriter(buf)
	f, _ := w.Create("../../etc/escaped.txt")
	_, _ = f.Write([]byte("pwn"))
	_ = w.Close()
	_ = os.WriteFile(path, buf.Bytes(), 0o644)
	return path
}

// ─── tests ────────────────────────────────────────────────────────────────

func TestDetectFormat_KnownSuffixes(t *testing.T) {
	for _, tc := range []struct {
		path string
		want Format
	}{
		{"file.zip", FormatZip},
		{"file.ZIP", FormatZip},
		{"file.tar", FormatTar},
		{"file.tar.gz", FormatTarGz},
		{"file.tgz", FormatTarGz},
		{"file.tar.bz2", FormatTarBz2},
		{"file.tbz2", FormatTarBz2},
		{"file.tbz", FormatTarBz2},
		{"file.gz", FormatGz},
		{"file.bz2", FormatBz2},
	} {
		got, err := DetectFormat(tc.path)
		if err != nil {
			t.Errorf("DetectFormat(%q) error: %v", tc.path, err)
			continue
		}
		if got != tc.want {
			t.Errorf("DetectFormat(%q) = %s, want %s", tc.path, got, tc.want)
		}
	}
}

func TestDetectFormat_Unknown(t *testing.T) {
	_, err := DetectFormat("file.rar")
	if err == nil {
		t.Error("unknown format should error")
	}
	if !errors.Is(err, ErrUnknownFormat) {
		t.Errorf("error should wrap ErrUnknownFormat, got %T", err)
	}
}

func TestExtractZip_SimpleHappyPath(t *testing.T) {
	dir := t.TempDir()
	zipPath := writeTestZip(t, dir, map[string]string{
		"hello.txt":    "hi there",
		"nested/x.bin": "binary content",
	})
	outDir := filepath.Join(dir, "out")
	_, err := Extract(zipPath, Options{OutDir: outDir})
	if err != nil {
		t.Fatal(err)
	}
	// 验证文件落地
	for _, name := range []string{"hello.txt", "nested/x.bin"} {
		fullPath := filepath.Join(outDir, name)
		if _, err := os.Stat(fullPath); err != nil {
			t.Errorf("missing extracted file %s: %v", name, err)
		}
	}
	// 验证内容
	body, _ := os.ReadFile(filepath.Join(outDir, "hello.txt"))
	if string(body) != "hi there" {
		t.Errorf("hello.txt content: %q", body)
	}
}

func TestExtractZip_ListMode(t *testing.T) {
	dir := t.TempDir()
	zipPath := writeTestZip(t, dir, map[string]string{
		"a.txt": "111",
		"b.txt": "22222",
	})
	entries, err := Extract(zipPath, Options{ListOnly: true})
	if err != nil {
		t.Fatal(err)
	}
	if len(entries) != 2 {
		t.Errorf("expected 2 entries, got %d", len(entries))
	}
	// 没有 OutDir 也不该写文件
}

func TestExtractTarGz(t *testing.T) {
	dir := t.TempDir()
	tarPath := writeTestTarGz(t, dir, map[string]string{
		"file1.txt":      "one",
		"sub/file2.txt":  "two",
	})
	outDir := filepath.Join(dir, "out")
	entries, err := Extract(tarPath, Options{OutDir: outDir})
	if err != nil {
		t.Fatal(err)
	}
	if len(entries) != 2 {
		t.Errorf("expected 2 entries, got %d", len(entries))
	}
	body, _ := os.ReadFile(filepath.Join(outDir, "sub/file2.txt"))
	if string(body) != "two" {
		t.Errorf("file2 content: %q", body)
	}
}

func TestExtract_DirectoryTraversalBlocked_Zip(t *testing.T) {
	dir := t.TempDir()
	zipPath := writeMaliciousZip(t, dir)
	outDir := filepath.Join(dir, "out")
	_, err := Extract(zipPath, Options{OutDir: outDir})
	if err == nil {
		t.Fatal("directory traversal should error")
	}
	if !strings.Contains(err.Error(), "..") {
		t.Errorf("error should mention '..', got: %v", err)
	}
	// 验证文件确实没逃出 outDir
	parentEscape := filepath.Join(filepath.Dir(dir), "..", "etc", "escaped.txt")
	if _, err := os.Stat(parentEscape); err == nil {
		t.Error("malicious file escaped to filesystem!")
	}
}

func TestExtract_UnknownFormat_Errors(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "file.rar")
	_ = os.WriteFile(path, []byte("x"), 0o644)
	_, err := Extract(path, Options{OutDir: dir})
	if err == nil {
		t.Error("unknown format should error")
	}
}

func TestSafeJoin_AllowsNormalPaths(t *testing.T) {
	root := t.TempDir()
	for _, name := range []string{"a.txt", "sub/b.txt", "deep/nested/c.bin"} {
		got, err := safeJoin(root, name)
		if err != nil {
			t.Errorf("safeJoin(%q) error: %v", name, err)
			continue
		}
		if !strings.HasPrefix(got, root) {
			t.Errorf("safeJoin(%q) escaped: %s", name, got)
		}
	}
}

func TestSafeJoin_RejectsTraversal(t *testing.T) {
	root := t.TempDir()
	for _, bad := range []string{
		"../escape.txt",
		"../../etc/passwd",
		"sub/../../escape.txt",
	} {
		_, err := safeJoin(root, bad)
		if err == nil {
			t.Errorf("safeJoin(%q) should error", bad)
		}
	}
}

func TestExtract_SingleGz(t *testing.T) {
	dir := t.TempDir()
	gzPath := filepath.Join(dir, "data.gz")
	// 写一个 gz 文件
	f, _ := os.Create(gzPath)
	gw := gzip.NewWriter(f)
	gw.Write([]byte("hello gz"))
	gw.Close()
	f.Close()

	outDir := filepath.Join(dir, "out")
	entries, err := Extract(gzPath, Options{OutDir: outDir})
	if err != nil {
		t.Fatal(err)
	}
	if len(entries) != 1 {
		t.Errorf("expected 1 entry, got %d", len(entries))
	}
	if entries[0].Name != "data" {
		t.Errorf("output name should be 'data' (sans .gz), got %q", entries[0].Name)
	}
	body, _ := os.ReadFile(filepath.Join(outDir, "data"))
	if string(body) != "hello gz" {
		t.Errorf("content: %q", body)
	}
}

func TestDefaultOutDir(t *testing.T) {
	for _, tc := range []struct {
		path string
		want string
	}{
		{"foo.zip", "foo"},
		{"bar.tar.gz", "bar"},
		{"x.tgz", "x"},
		{"a.tar.bz2", "a"},
		{"plain.txt", "plain.txt-extracted"},
		{"/path/to/file.zip", "file"},
	} {
		if got := DefaultOutDir(tc.path); got != tc.want {
			t.Errorf("DefaultOutDir(%q) = %q, want %q", tc.path, got, tc.want)
		}
	}
}

func TestExtractZip_PreservesNestedDirs(t *testing.T) {
	dir := t.TempDir()
	zipPath := writeTestZip(t, dir, map[string]string{
		"a/b/c/deep.txt": "deep content",
	})
	outDir := filepath.Join(dir, "out")
	_, err := Extract(zipPath, Options{OutDir: outDir})
	if err != nil {
		t.Fatal(err)
	}
	body, err := os.ReadFile(filepath.Join(outDir, "a/b/c/deep.txt"))
	if err != nil {
		t.Fatal(err)
	}
	if string(body) != "deep content" {
		t.Errorf("nested content lost: %q", body)
	}
}
