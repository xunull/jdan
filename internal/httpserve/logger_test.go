package httpserve

import (
	"bytes"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"strings"
	"sync"
	"testing"
	"time"
)

func TestStripPort(t *testing.T) {
	for _, tc := range []struct {
		in, want string
	}{
		{"192.168.1.5:12345", "192.168.1.5"},
		{"[::1]:8080", "::1"},
		{"no-port", "no-port"},
	} {
		if got := stripPort(tc.in); got != tc.want {
			t.Errorf("stripPort(%q) = %q, want %q", tc.in, got, tc.want)
		}
	}
}

func TestClientSet_CountsDistinctIPs(t *testing.T) {
	c := newClientSet()
	c.Add("192.168.1.5:1111")
	c.Add("192.168.1.5:2222") // 同 IP 不同端口
	c.Add("192.168.1.6:1111")
	if c.Len() != 2 {
		t.Errorf("expected 2 distinct clients, got %d", c.Len())
	}
}

func TestClientSet_Concurrent(t *testing.T) {
	c := newClientSet()
	var wg sync.WaitGroup
	for range 50 {
		wg.Add(1)
		go func() {
			defer wg.Done()
			c.Add("192.168.1.5:1234")
		}()
	}
	wg.Wait()
	if c.Len() != 1 {
		t.Errorf("expected 1 client under concurrent Add, got %d", c.Len())
	}
}

func TestFormatBytes(t *testing.T) {
	for _, tc := range []struct {
		in   int64
		want string
	}{
		{0, "-"},
		{500, "500B"},
		{2048, "2.0KB"},
		{1024 * 1024 * 3, "3.0MB"},
		{int64(1024 * 1024 * 1024 * 2), "2.0GB"},
	} {
		if got := formatBytes(tc.in); got != tc.want {
			t.Errorf("formatBytes(%d) = %q, want %q", tc.in, got, tc.want)
		}
	}
}

func TestFormatDuration(t *testing.T) {
	if got := formatDuration(500 * time.Microsecond); got != "500µs" {
		t.Errorf("got %s, want 500µs", got)
	}
	if got := formatDuration(2 * time.Millisecond); got != "2ms" {
		t.Errorf("got %s, want 2ms", got)
	}
}

func TestWithLogging_TextFormat(t *testing.T) {
	var buf bytes.Buffer
	stats := newStats()
	h := withLogging(&buf, LogText, stats,
		http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			w.WriteHeader(http.StatusOK)
			_, _ = w.Write([]byte("hello"))
		}))
	req := httptest.NewRequest("GET", "/path", nil)
	req.RemoteAddr = "192.168.1.5:1234"
	rec := httptest.NewRecorder()
	h.ServeHTTP(rec, req)

	out := buf.String()
	for _, want := range []string{"[GET]", "200", "/path", "192.168.1.5"} {
		if !strings.Contains(out, want) {
			t.Errorf("log missing %q: %s", want, out)
		}
	}
	if reqs, bytesN, clients := stats.Snapshot(); reqs != 1 || bytesN != 5 || clients != 1 {
		t.Errorf("stats: reqs=%d bytes=%d clients=%d", reqs, bytesN, clients)
	}
}

func TestWithLogging_JSONFormat(t *testing.T) {
	var buf bytes.Buffer
	h := withLogging(&buf, LogJSON, newStats(),
		http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			w.WriteHeader(http.StatusNotFound)
		}))
	req := httptest.NewRequest("GET", "/missing", nil)
	req.RemoteAddr = "10.0.0.1:5555"
	rec := httptest.NewRecorder()
	h.ServeHTTP(rec, req)

	var entry map[string]any
	if err := json.Unmarshal(buf.Bytes(), &entry); err != nil {
		t.Fatalf("invalid JSON: %v\n%s", err, buf.String())
	}
	if entry["status"].(float64) != 404 {
		t.Errorf("status: %v", entry["status"])
	}
	if entry["path"] != "/missing" {
		t.Errorf("path: %v", entry["path"])
	}
	if entry["remote"] != "10.0.0.1" {
		t.Errorf("remote: %v", entry["remote"])
	}
	if entry["method"] != "GET" {
		t.Errorf("method: %v", entry["method"])
	}
}

func TestWithLogging_LogOff(t *testing.T) {
	var buf bytes.Buffer
	stats := newStats()
	h := withLogging(&buf, LogOff, stats,
		http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			_, _ = w.Write([]byte("x"))
		}))
	h.ServeHTTP(httptest.NewRecorder(), httptest.NewRequest("GET", "/", nil))
	if buf.Len() != 0 {
		t.Errorf("LogOff should write nothing, got %q", buf.String())
	}
	// 但 stats 仍然累加
	if reqs, _, _ := stats.Snapshot(); reqs != 1 {
		t.Errorf("LogOff should still accumulate stats, got %d requests", reqs)
	}
}

func TestWithLogging_NilOut(t *testing.T) {
	stats := newStats()
	h := withLogging(nil, LogText, stats,
		http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			w.WriteHeader(http.StatusOK)
		}))
	// 应当不 panic
	h.ServeHTTP(httptest.NewRecorder(), httptest.NewRequest("GET", "/", nil))
	if reqs, _, _ := stats.Snapshot(); reqs != 1 {
		t.Errorf("nil out should still update stats")
	}
}

func TestCaptureWriter_DefaultStatus200(t *testing.T) {
	// 没显式调 WriteHeader 时，captureWriter 应当推断 200
	stats := newStats()
	var buf bytes.Buffer
	h := withLogging(&buf, LogText, stats,
		http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			_, _ = w.Write([]byte("implicit 200"))
		}))
	h.ServeHTTP(httptest.NewRecorder(), httptest.NewRequest("GET", "/", nil))
	if !strings.Contains(buf.String(), "200") {
		t.Errorf("implicit status should be 200: %s", buf.String())
	}
}
