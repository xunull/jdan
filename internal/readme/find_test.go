package readme

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func TestFindReadme_ExactMatch(t *testing.T) {
	dir := t.TempDir()
	want := filepath.Join(dir, "README.md")
	if err := os.WriteFile(want, []byte("hi"), 0o644); err != nil {
		t.Fatal(err)
	}
	got, err := FindReadme(dir)
	if err != nil {
		t.Fatalf("FindReadme error: %v", err)
	}
	if got != want {
		t.Errorf("want %s, got %s", want, got)
	}
}

func TestFindReadme_LowercaseVariant(t *testing.T) {
	dir := t.TempDir()
	want := filepath.Join(dir, "readme.md")
	if err := os.WriteFile(want, []byte("hi"), 0o644); err != nil {
		t.Fatal(err)
	}
	got, err := FindReadme(dir)
	if err != nil {
		t.Fatalf("FindReadme error: %v", err)
	}
	if got != want {
		t.Errorf("want %s, got %s", want, got)
	}
}

func TestFindReadme_MixedCaseVariant(t *testing.T) {
	dir := t.TempDir()
	want := filepath.Join(dir, "ReadMe.md")
	if err := os.WriteFile(want, []byte("hi"), 0o644); err != nil {
		t.Fatal(err)
	}
	got, err := FindReadme(dir)
	if err != nil {
		t.Fatalf("FindReadme error: %v", err)
	}
	if got != want {
		t.Errorf("want %s, got %s", want, got)
	}
}

func TestFindReadme_NotFound(t *testing.T) {
	dir := t.TempDir()
	if err := os.WriteFile(filepath.Join(dir, "other.txt"), []byte("hi"), 0o644); err != nil {
		t.Fatal(err)
	}
	_, err := FindReadme(dir)
	if err == nil {
		t.Fatal("expected error, got nil")
	}
	if !strings.Contains(err.Error(), "未找到") {
		t.Errorf("unexpected error: %v", err)
	}
}

func TestFindReadme_NonexistentDir(t *testing.T) {
	_, err := FindReadme(filepath.Join(os.TempDir(), "jdan-readme-does-not-exist-xyz-123"))
	if err == nil {
		t.Fatal("expected error, got nil")
	}
}

func TestFindReadme_NotADirectory(t *testing.T) {
	dir := t.TempDir()
	f := filepath.Join(dir, "some.txt")
	if err := os.WriteFile(f, []byte("hi"), 0o644); err != nil {
		t.Fatal(err)
	}
	_, err := FindReadme(f)
	if err == nil {
		t.Fatal("expected error, got nil")
	}
	if !strings.Contains(err.Error(), "不是一个目录") {
		t.Errorf("unexpected error: %v", err)
	}
}

func TestFindReadme_IgnoresDirectoryEntry(t *testing.T) {
	dir := t.TempDir()
	// 在大小写敏感文件系统上 README.md 目录可能与 readme.md 文件共存；
	// 在 macOS APFS 默认（大小写不敏感）环境上这两者会冲突。
	// 这里只验证“目录名匹配时被跳过、找不到合法 README.md 时报错”。
	if err := os.Mkdir(filepath.Join(dir, "README.md"), 0o755); err != nil {
		t.Fatal(err)
	}
	_, err := FindReadme(dir)
	if err == nil {
		t.Fatal("expected error because README.md is a directory")
	}
	if !strings.Contains(err.Error(), "未找到") {
		t.Errorf("unexpected error: %v", err)
	}
}
