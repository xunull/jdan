package filebak

import (
	"os"
	"path/filepath"
	"testing"
	"time"
)

func TestBackupFile_successAndConflict(t *testing.T) {
	dir := t.TempDir()
	src := filepath.Join(dir, "a.txt")
	if err := os.WriteFile(src, []byte("hello"), 0o644); err != nil {
		t.Fatal(err)
	}
	now := time.Date(2025, 3, 21, 15, 30, 45, 0, time.Local)

	if err := BackupFile(src, "", now); err != nil {
		t.Fatal(err)
	}
	dst := filepath.Join(dir, "a.txt.bak.20250321-153045")
	got, err := os.ReadFile(dst)
	if err != nil {
		t.Fatal(err)
	}
	if string(got) != "hello" {
		t.Fatalf("content = %q", got)
	}

	if err := BackupFile(src, "", now); err == nil {
		t.Fatal("expected conflict error")
	}
}

func TestBackupFile_notRegular(t *testing.T) {
	dir := t.TempDir()
	if err := os.Mkdir(filepath.Join(dir, "d"), 0o755); err != nil {
		t.Fatal(err)
	}
	now := time.Date(2025, 3, 21, 15, 30, 45, 0, time.Local)
	err := BackupFile(filepath.Join(dir, "d"), "", now)
	if err == nil {
		t.Fatal("expected error for directory")
	}
}
