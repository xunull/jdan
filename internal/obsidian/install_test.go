package obsidian

import (
	"fmt"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"sync/atomic"
	"testing"
)

func newTestInstaller(srv *httptest.Server) *Installer {
	return &Installer{
		Client:  srv.Client(),
		BaseURL: srv.URL,
	}
}

func happySrv() *httptest.Server {
	return httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		fmt.Fprintf(w, "mock content for %s", r.URL.Path)
	}))
}

func TestInstall_HappyPath(t *testing.T) {
	srv := happySrv()
	defer srv.Close()

	vaultDir := t.TempDir()
	if err := newTestInstaller(srv).Install(vaultDir, false); err != nil {
		t.Fatalf("Install() unexpected error: %v", err)
	}

	pluginDir := filepath.Join(vaultDir, ".obsidian", "plugins", "claudian")
	for _, f := range claudianFiles {
		if _, err := os.Stat(filepath.Join(pluginDir, f)); err != nil {
			t.Errorf("expected file %s to exist: %v", f, err)
		}
	}
}

func TestInstall_VaultNotFound(t *testing.T) {
	srv := happySrv()
	defer srv.Close()

	err := newTestInstaller(srv).Install("/nonexistent/vault/path", false)
	if err == nil {
		t.Fatal("expected error for nonexistent vault path")
	}
}

func TestInstall_VaultIsFile(t *testing.T) {
	srv := happySrv()
	defer srv.Close()

	f, err := os.CreateTemp("", "jdan-test-*")
	if err != nil {
		t.Fatal(err)
	}
	f.Close()
	defer os.Remove(f.Name())

	if err := newTestInstaller(srv).Install(f.Name(), false); err == nil {
		t.Fatal("expected error when vault path is a file, not a directory")
	}
}

func TestInstall_AlreadyInstalled_NoForce(t *testing.T) {
	srv := happySrv()
	defer srv.Close()
	ins := newTestInstaller(srv)

	vaultDir := t.TempDir()
	if err := ins.Install(vaultDir, false); err != nil {
		t.Fatalf("first Install() error: %v", err)
	}

	err := ins.Install(vaultDir, false)
	if err == nil {
		t.Fatal("expected error when re-installing without --force")
	}
}

func TestInstall_AlreadyInstalled_WithForce(t *testing.T) {
	srv := happySrv()
	defer srv.Close()
	ins := newTestInstaller(srv)

	vaultDir := t.TempDir()
	if err := ins.Install(vaultDir, false); err != nil {
		t.Fatalf("first Install() error: %v", err)
	}

	if err := ins.Install(vaultDir, true); err != nil {
		t.Fatalf("Install() with --force error: %v", err)
	}
}

func TestInstall_HTTP404_CleansUp(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusNotFound)
	}))
	defer srv.Close()

	vaultDir := t.TempDir()
	err := newTestInstaller(srv).Install(vaultDir, false)
	if err == nil {
		t.Fatal("expected error for HTTP 404")
	}

	pluginDir := filepath.Join(vaultDir, ".obsidian", "plugins", "claudian")
	if _, statErr := os.Stat(pluginDir); !os.IsNotExist(statErr) {
		t.Error("expected plugin dir to be removed after failed install")
	}
}

func TestInstall_PartialFailure_CleansUp(t *testing.T) {
	var calls atomic.Int32
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if calls.Add(1) >= 2 {
			w.WriteHeader(http.StatusInternalServerError)
			return
		}
		fmt.Fprint(w, "ok")
	}))
	defer srv.Close()

	vaultDir := t.TempDir()
	err := newTestInstaller(srv).Install(vaultDir, false)
	if err == nil {
		t.Fatal("expected error when second file download fails")
	}

	pluginDir := filepath.Join(vaultDir, ".obsidian", "plugins", "claudian")
	if _, statErr := os.Stat(pluginDir); !os.IsNotExist(statErr) {
		t.Error("expected plugin dir to be removed after partial failure")
	}
}
