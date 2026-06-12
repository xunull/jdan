package cli

import (
	"bytes"
	"strings"
	"testing"
)

func TestVersion_DefaultFormat(t *testing.T) {
	var buf bytes.Buffer
	cmd := newVersionCommand(versionDeps{
		out:     &buf,
		version: "v0.1.0",
		commit:  "abc1234",
		date:    "2026-06-12T10:00:00Z",
		goos:    "darwin",
		goarch:  "arm64",
	})
	if err := cmd.RunE(cmd, nil); err != nil {
		t.Fatal(err)
	}
	got := buf.String()
	want := "jdan v0.1.0 (commit abc1234, built 2026-06-12T10:00:00Z, darwin/arm64)\n"
	if got != want {
		t.Errorf("got %q\nwant %q", got, want)
	}
}

func TestVersion_ShortFlag(t *testing.T) {
	var buf bytes.Buffer
	cmd := newVersionCommand(versionDeps{
		out:     &buf,
		version: "v1.2.3",
		commit:  "deadbeef",
		date:    "2026-06-12",
	})
	cmd.SetArgs([]string{"--short"})
	if err := cmd.Execute(); err != nil {
		t.Fatal(err)
	}
	got := strings.TrimSpace(buf.String())
	if got != "v1.2.3" {
		t.Errorf("--short got %q, want %q", got, "v1.2.3")
	}
}

func TestVersion_DevFallback(t *testing.T) {
	var buf bytes.Buffer
	cmd := newVersionCommand(versionDeps{
		out:     &buf,
		version: "dev",
		commit:  "none",
		date:    "unknown",
		goos:    "linux",
		goarch:  "amd64",
	})
	if err := cmd.RunE(cmd, nil); err != nil {
		t.Fatal(err)
	}
	got := buf.String()
	if !strings.Contains(got, "jdan dev") {
		t.Errorf("dev fallback missing 'jdan dev' in %q", got)
	}
	if !strings.Contains(got, "linux/amd64") {
		t.Errorf("platform missing in %q", got)
	}
}

func TestVersion_DefaultsToRuntimePlatform(t *testing.T) {
	// goos/goarch 留空时应当 fallback 到 runtime.GOOS/GOARCH，不应输出空白
	var buf bytes.Buffer
	cmd := newVersionCommand(versionDeps{
		out:     &buf,
		version: "v0.1.0",
		commit:  "x",
		date:    "y",
	})
	if err := cmd.RunE(cmd, nil); err != nil {
		t.Fatal(err)
	}
	got := buf.String()
	// 应当包含 / 分隔的 GOOS/GOARCH，不是空白
	if strings.Contains(got, "  ") || strings.Contains(got, "/)") {
		t.Errorf("empty platform leaked: %q", got)
	}
}
