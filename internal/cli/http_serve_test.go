package cli

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func TestResolveServePath_Directory(t *testing.T) {
	dir := t.TempDir()
	root, redirect, err := resolveServePath(dir)
	if err != nil {
		t.Fatal(err)
	}
	if !filepath.IsAbs(root) {
		t.Errorf("root should be absolute, got %q", root)
	}
	if redirect != "" {
		t.Errorf("directory should not redirect, got %q", redirect)
	}
}

func TestResolveServePath_SingleFile(t *testing.T) {
	dir := t.TempDir()
	file := filepath.Join(dir, "report.pdf")
	_ = os.WriteFile(file, []byte("x"), 0o644)
	root, redirect, err := resolveServePath(file)
	if err != nil {
		t.Fatal(err)
	}
	// 期望 root = dir, redirect = "/report.pdf"
	if abs, _ := filepath.Abs(dir); root != abs {
		t.Errorf("root: got %q, want %q", root, abs)
	}
	if redirect != "/report.pdf" {
		t.Errorf("redirect: got %q, want /report.pdf", redirect)
	}
}

func TestResolveServePath_Missing(t *testing.T) {
	if _, _, err := resolveServePath("/definitely/not/exist/xyz"); err == nil {
		t.Error("missing path should error")
	}
}

func TestResolveServePath_Dot(t *testing.T) {
	// "." 解析成 cwd
	root, redirect, err := resolveServePath(".")
	if err != nil {
		t.Fatal(err)
	}
	if !filepath.IsAbs(root) {
		t.Errorf("'.' should resolve to absolute root, got %q", root)
	}
	if redirect != "" {
		t.Errorf("'.' is directory, no redirect, got %q", redirect)
	}
}

func TestHumanBytes(t *testing.T) {
	for _, tc := range []struct {
		in   uint64
		want string
	}{
		{0, "0B"},
		{500, "500B"},
		{2048, "2.0KB"},
		{1024 * 1024 * 3, "3.0MB"},
		{uint64(1024) * 1024 * 1024 * 2, "2.0GB"},
	} {
		got := humanBytes(tc.in)
		if got != tc.want {
			t.Errorf("humanBytes(%d) = %q, want %q", tc.in, got, tc.want)
		}
	}
}

func TestPrintStartBanner_IncludesWarning(t *testing.T) {
	var buf strings.Builder
	printStartBanner(&buf, "/tmp", "0.0.0.0", 8080, "", false, true)
	out := buf.String()
	if !strings.Contains(out, "⚠") {
		t.Error("bind 0.0.0.0 should print warning")
	}
	if !strings.Contains(out, "anyone on your LAN") {
		t.Error("warning should mention LAN visibility")
	}
}

func TestPrintStartBanner_NoBindWarning_OnLocalhost(t *testing.T) {
	var buf strings.Builder
	printStartBanner(&buf, "/tmp", "127.0.0.1", 8080, "", false, true)
	out := buf.String()
	if strings.Contains(out, "⚠") {
		t.Error("127.0.0.1 should NOT print LAN warning")
	}
}

func TestPrintStartBanner_NoQR(t *testing.T) {
	// noQR=true 应当不打印 ▀ ▄ block chars
	var buf strings.Builder
	printStartBanner(&buf, "/tmp", "127.0.0.1", 8080, "", false, true)
	if strings.ContainsAny(buf.String(), "▀▄█") {
		t.Error("--no-qr should suppress block chars")
	}
}

func TestPrintStartBanner_UploadHint(t *testing.T) {
	var buf strings.Builder
	printStartBanner(&buf, "/tmp", "127.0.0.1", 8080, "", true, true)
	if !strings.Contains(buf.String(), "/upload") {
		t.Error("upload mode should mention /upload in banner")
	}
}

func TestPrintStartBanner_RedirectInURL(t *testing.T) {
	var buf strings.Builder
	printStartBanner(&buf, "/tmp", "127.0.0.1", 8080, "/file.pdf", false, true)
	if !strings.Contains(buf.String(), "/file.pdf") {
		t.Errorf("single-file redirect should appear in URL, got: %s", buf.String())
	}
}
