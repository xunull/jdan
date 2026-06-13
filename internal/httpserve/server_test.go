package httpserve

import (
	"context"
	"encoding/base64"
	"fmt"
	"io"
	"net/http"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"
)

// startTestServer 起一个 server 在随机端口上，返回它的 base URL + cleanup。
func startTestServer(t *testing.T, opts Options) (string, func()) {
	t.Helper()
	s, err := New(opts)
	if err != nil {
		t.Fatalf("New: %v", err)
	}
	ctx, cancel := context.WithCancel(context.Background())
	done := make(chan error, 1)
	go func() {
		done <- s.Run(ctx)
	}()

	// 简单等 server 起来
	base := fmt.Sprintf("http://%s", s.Addr())
	// Listen 在 New 里已经完成，所以 Addr 已经 ready
	cleanup := func() {
		cancel()
		select {
		case err := <-done:
			if err != nil {
				t.Logf("server returned: %v", err)
			}
		case <-time.After(3 * time.Second):
			t.Error("server did not shut down")
		}
	}
	return base, cleanup
}

func TestServer_ServesFile(t *testing.T) {
	dir := t.TempDir()
	_ = os.WriteFile(filepath.Join(dir, "hello.txt"), []byte("hi there"), 0o644)

	base, stop := startTestServer(t, Options{
		Root:      dir,
		Bind:      "127.0.0.1",
		LogFormat: LogOff,
	})
	defer stop()

	resp, err := http.Get(base + "/hello.txt")
	if err != nil {
		t.Fatal(err)
	}
	defer resp.Body.Close()
	body, _ := io.ReadAll(resp.Body)
	if string(body) != "hi there" {
		t.Errorf("body %q, want 'hi there'", string(body))
	}
}

func TestServer_404OnMissing(t *testing.T) {
	dir := t.TempDir()
	base, stop := startTestServer(t, Options{
		Root: dir, Bind: "127.0.0.1", LogFormat: LogOff,
	})
	defer stop()

	resp, err := http.Get(base + "/does-not-exist")
	if err != nil {
		t.Fatal(err)
	}
	defer resp.Body.Close()
	if resp.StatusCode != http.StatusNotFound {
		t.Errorf("status %d, want 404", resp.StatusCode)
	}
}

func TestServer_DirectoryTraversalBlocked(t *testing.T) {
	dir := t.TempDir()
	_ = os.WriteFile(filepath.Join(dir, "ok.txt"), []byte("ok"), 0o644)
	// 在父目录创建一个 "secret" 文件
	parent := filepath.Dir(dir)
	secretPath := filepath.Join(parent, "secret-out.txt")
	_ = os.WriteFile(secretPath, []byte("secret"), 0o644)
	defer os.Remove(secretPath)

	base, stop := startTestServer(t, Options{
		Root: dir, Bind: "127.0.0.1", LogFormat: LogOff,
	})
	defer stop()

	// 试图通过 traversal 拿父目录的 secret
	resp, err := http.Get(base + "/../secret-out.txt")
	if err != nil {
		t.Fatal(err)
	}
	defer resp.Body.Close()
	body, _ := io.ReadAll(resp.Body)
	if strings.Contains(string(body), "secret") {
		t.Errorf("traversal succeeded, body: %q", string(body))
	}
}

func TestServer_SymlinkEscapeBlocked(t *testing.T) {
	dir := t.TempDir()
	parent := filepath.Dir(dir)
	secretPath := filepath.Join(parent, "outside.txt")
	_ = os.WriteFile(secretPath, []byte("outside content"), 0o644)
	defer os.Remove(secretPath)

	// 在 root 内创建一个指向外部的 symlink
	link := filepath.Join(dir, "linked.txt")
	if err := os.Symlink(secretPath, link); err != nil {
		t.Skipf("symlink not supported: %v", err)
	}

	base, stop := startTestServer(t, Options{
		Root: dir, Bind: "127.0.0.1", LogFormat: LogOff,
	})
	defer stop()

	resp, err := http.Get(base + "/linked.txt")
	if err != nil {
		t.Fatal(err)
	}
	defer resp.Body.Close()
	body, _ := io.ReadAll(resp.Body)
	if strings.Contains(string(body), "outside content") {
		t.Errorf("symlink escape allowed: %q", string(body))
	}
	if resp.StatusCode != http.StatusNotFound {
		t.Errorf("symlink escape should 404, got %d", resp.StatusCode)
	}
}

func TestServer_BasicAuth(t *testing.T) {
	dir := t.TempDir()
	_ = os.WriteFile(filepath.Join(dir, "a.txt"), []byte("data"), 0o644)

	base, stop := startTestServer(t, Options{
		Root: dir, Bind: "127.0.0.1", LogFormat: LogOff,
		BasicAuth: "alice:wonder",
	})
	defer stop()

	// 无 auth → 401
	resp, err := http.Get(base + "/a.txt")
	if err != nil {
		t.Fatal(err)
	}
	if resp.StatusCode != http.StatusUnauthorized {
		t.Errorf("no auth: status %d, want 401", resp.StatusCode)
	}
	_ = resp.Body.Close()
	if h := resp.Header.Get("WWW-Authenticate"); !strings.Contains(h, "Basic") {
		t.Errorf("missing Basic challenge header: %q", h)
	}

	// 错 auth → 401
	req, _ := http.NewRequest("GET", base+"/a.txt", nil)
	req.Header.Set("Authorization", "Basic "+base64Encode("alice:wrong"))
	resp2, _ := http.DefaultClient.Do(req)
	if resp2.StatusCode != http.StatusUnauthorized {
		t.Errorf("wrong auth: status %d, want 401", resp2.StatusCode)
	}
	_ = resp2.Body.Close()

	// 对 auth → 200
	req, _ = http.NewRequest("GET", base+"/a.txt", nil)
	req.Header.Set("Authorization", "Basic "+base64Encode("alice:wonder"))
	resp3, _ := http.DefaultClient.Do(req)
	if resp3.StatusCode != http.StatusOK {
		t.Errorf("correct auth: status %d, want 200", resp3.StatusCode)
	}
	_ = resp3.Body.Close()
}

func base64Encode(s string) string {
	return base64.StdEncoding.EncodeToString([]byte(s))
}

func TestServer_StatsAccumulate(t *testing.T) {
	dir := t.TempDir()
	_ = os.WriteFile(filepath.Join(dir, "x.txt"), []byte("12345"), 0o644)

	s, err := New(Options{Root: dir, Bind: "127.0.0.1", LogFormat: LogOff})
	if err != nil {
		t.Fatal(err)
	}
	ctx, cancel := context.WithCancel(context.Background())
	go s.Run(ctx)
	defer cancel()

	for range 3 {
		resp, _ := http.Get("http://" + s.Addr() + "/x.txt")
		_, _ = io.Copy(io.Discard, resp.Body)
		_ = resp.Body.Close()
	}

	reqs, bytesN, clients := s.Stats().Snapshot()
	if reqs != 3 {
		t.Errorf("requests %d, want 3", reqs)
	}
	if bytesN != 15 { // 3 * 5
		t.Errorf("bytes %d, want 15", bytesN)
	}
	if clients != 1 {
		t.Errorf("distinct clients %d, want 1", clients)
	}
}

func TestServer_RejectsNonDirRoot(t *testing.T) {
	f := filepath.Join(t.TempDir(), "x.txt")
	_ = os.WriteFile(f, []byte("x"), 0o644)
	if _, err := New(Options{Root: f, Bind: "127.0.0.1"}); err == nil {
		t.Error("file as root should error")
	}
}

func TestServer_RejectsMissingRoot(t *testing.T) {
	if _, err := New(Options{Root: "/definitely/not/exist", Bind: "127.0.0.1"}); err == nil {
		t.Error("missing root should error")
	}
}

func TestServer_UploadRoute(t *testing.T) {
	dir := t.TempDir()
	uploadDir := filepath.Join(dir, "in")

	base, stop := startTestServer(t, Options{
		Root:      dir,
		Bind:      "127.0.0.1",
		LogFormat: LogOff,
		Upload:    true,
		UploadDir: uploadDir,
	})
	defer stop()

	// GET /upload 应当返回 HTML 表单
	resp, err := http.Get(base + "/upload")
	if err != nil {
		t.Fatal(err)
	}
	body, _ := io.ReadAll(resp.Body)
	_ = resp.Body.Close()
	if !strings.Contains(string(body), "<form") {
		t.Error("GET /upload should return form HTML")
	}
}

func TestServer_OnStartedCallbackFires(t *testing.T) {
	dir := t.TempDir()
	called := make(chan string, 1)

	s, err := New(Options{
		Root: dir, Bind: "127.0.0.1", LogFormat: LogOff,
		OnStarted: func(addr string) { called <- addr },
	})
	if err != nil {
		t.Fatal(err)
	}
	ctx, cancel := context.WithCancel(context.Background())
	go s.Run(ctx)
	defer cancel()

	select {
	case addr := <-called:
		if !strings.HasPrefix(addr, "127.0.0.1:") {
			t.Errorf("addr in callback unexpected: %q", addr)
		}
	case <-time.After(2 * time.Second):
		t.Error("OnStarted not called within 2s")
	}
}

func TestServer_GracefulShutdown(t *testing.T) {
	dir := t.TempDir()
	s, err := New(Options{
		Root: dir, Bind: "127.0.0.1", LogFormat: LogOff,
		Shutdown: 1 * time.Second,
	})
	if err != nil {
		t.Fatal(err)
	}
	ctx, cancel := context.WithCancel(context.Background())
	done := make(chan error, 1)
	go func() { done <- s.Run(ctx) }()

	time.Sleep(50 * time.Millisecond) // 让 server 起来
	cancel()

	select {
	case err := <-done:
		if err != nil {
			t.Errorf("graceful shutdown returned error: %v", err)
		}
	case <-time.After(3 * time.Second):
		t.Fatal("shutdown timed out")
	}
}
