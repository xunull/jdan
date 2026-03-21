package filebak

import (
	"errors"
	"fmt"
	"path/filepath"
	"testing"
	"time"
)

func TestNormalizeDesc(t *testing.T) {
	tests := []struct {
		in     string
		want   string
		wantOK bool
	}{
		{"", "", true},
		{"  ", "", true},
		{"a b", "a_b", true},
		{"ab", "ab", true},
		{"中文", "中文", true},
		{"a,b", "", false},
		{"a\tb", "", false},
	}
	for i, tt := range tests {
		t.Run(fmt.Sprintf("%d", i), func(t *testing.T) {
			got, ok := NormalizeDesc(tt.in)
			if ok != tt.wantOK {
				t.Fatalf("ok = %v, want %v", ok, tt.wantOK)
			}
			if tt.wantOK && got != tt.want {
				t.Fatalf("got %q, want %q", got, tt.want)
			}
		})
	}
}

func TestBackupDestination(t *testing.T) {
	now := time.Date(2025, 3, 21, 15, 30, 45, 0, time.Local)
	src := filepath.Join("some", "dir", "foo.txt")

	got, err := BackupDestination(src, now, "")
	if err != nil {
		t.Fatal(err)
	}
	want := filepath.Join("some", "dir", "foo.txt.bak.20250321-153045")
	if got != want {
		t.Fatalf("got %q want %q", got, want)
	}

	got, err = BackupDestination(src, now, "a b")
	if err != nil {
		t.Fatal(err)
	}
	want = filepath.Join("some", "dir", "foo.txt.bak.20250321-153045-a_b")
	if got != want {
		t.Fatalf("got %q want %q", got, want)
	}

	_, err = BackupDestination(src, now, "a,b")
	if !errors.Is(err, ErrInvalidDesc) {
		t.Fatalf("want ErrInvalidDesc, got %v", err)
	}
}
