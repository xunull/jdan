package dnslookup

import (
	"os"
	"path/filepath"
	"testing"
)

func writeResolvConf(t *testing.T, content string) string {
	t.Helper()
	dir := t.TempDir()
	path := filepath.Join(dir, "resolv.conf")
	if err := os.WriteFile(path, []byte(content), 0o644); err != nil {
		t.Fatalf("WriteFile: %v", err)
	}
	return path
}

func TestDetectFromFile_TakesFirstNameserver(t *testing.T) {
	path := writeResolvConf(t, "nameserver 192.168.1.1\nnameserver 10.0.0.1\n")
	if got := detectFromFile(path); got != "192.168.1.1:53" {
		t.Errorf("got %q, want 192.168.1.1:53", got)
	}
}

func TestDetectFromFile_MissingFileFallsBack(t *testing.T) {
	if got := detectFromFile("/this/path/should/not/exist"); got != fallbackServer {
		t.Errorf("got %q, want %q", got, fallbackServer)
	}
}

func TestDetectFromFile_NoNameserverFallsBack(t *testing.T) {
	path := writeResolvConf(t, "# only a comment\nsearch example.com\n")
	if got := detectFromFile(path); got != fallbackServer {
		t.Errorf("got %q, want %q", got, fallbackServer)
	}
}

func TestDetectFromFile_BracketsIPv6Nameserver(t *testing.T) {
	path := writeResolvConf(t, "nameserver 2001:db8::1\n")
	if got := detectFromFile(path); got != "[2001:db8::1]:53" {
		t.Errorf("got %q, want [2001:db8::1]:53", got)
	}
}
