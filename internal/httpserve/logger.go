package httpserve

import (
	"encoding/json"
	"fmt"
	"io"
	"net"
	"net/http"
	"sync"
	"sync/atomic"
	"time"
)

// LogFormat 控制访问日志的输出格式。
type LogFormat int

const (
	LogText LogFormat = iota // [GET] 200 /path  192.168.1.5  12ms  (3.2KB)
	LogJSON                  // ndjson 一行一个 event
	LogOff                   // 不打日志（--quiet）
)

// Stats 是 server 在跑过程中累积的简单统计，server.Run 退出时打印 summary。
type Stats struct {
	requests atomic.Uint64
	bytes    atomic.Uint64
	clients  *clientSet
}

func newStats() *Stats {
	return &Stats{clients: newClientSet()}
}

// Snapshot 返回当前统计的瞬时拷贝。
func (s *Stats) Snapshot() (requests, bytes uint64, distinctClients int) {
	return s.requests.Load(), s.bytes.Load(), s.clients.Len()
}

// clientSet 跟踪 distinct remote address，用 mutex 保证并发安全。
// 注意只存 IP（去端口），同一手机多次请求只算一个 client。
type clientSet struct {
	m  map[string]struct{}
	mu sync.Mutex
}

func newClientSet() *clientSet {
	return &clientSet{m: make(map[string]struct{})}
}

func (c *clientSet) Add(remoteAddr string) {
	ip := stripPort(remoteAddr)
	c.mu.Lock()
	c.m[ip] = struct{}{}
	c.mu.Unlock()
}

func (c *clientSet) Len() int {
	c.mu.Lock()
	defer c.mu.Unlock()
	return len(c.m)
}

func stripPort(addr string) string {
	host, _, err := net.SplitHostPort(addr)
	if err != nil {
		return addr
	}
	return host
}

// captureWriter 包裹 http.ResponseWriter 抓 status code 和写入字节数，
// 给访问日志算"served N bytes"。
type captureWriter struct {
	http.ResponseWriter
	status int
	bytes  int64
}

func (w *captureWriter) WriteHeader(code int) {
	w.status = code
	w.ResponseWriter.WriteHeader(code)
}

func (w *captureWriter) Write(b []byte) (int, error) {
	if w.status == 0 {
		w.status = http.StatusOK
	}
	n, err := w.ResponseWriter.Write(b)
	w.bytes += int64(n)
	return n, err
}

// withLogging 把 next handler 包一层，按 format 写访问日志到 out + 累加 Stats。
// out == nil 时不写日志（但 Stats 还是更新，方便 summary）。
func withLogging(out io.Writer, format LogFormat, stats *Stats, next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		start := time.Now()
		cw := &captureWriter{ResponseWriter: w}
		next.ServeHTTP(cw, r)
		dur := time.Since(start)
		status := cw.status
		if status == 0 {
			status = http.StatusOK
		}

		if stats != nil {
			stats.requests.Add(1)
			if cw.bytes > 0 {
				stats.bytes.Add(uint64(cw.bytes))
			}
			stats.clients.Add(r.RemoteAddr)
		}
		if out == nil || format == LogOff {
			return
		}
		writeLog(out, format, r, status, cw.bytes, dur)
	})
}

func writeLog(out io.Writer, format LogFormat, r *http.Request, status int, bytes int64, dur time.Duration) {
	if format == LogJSON {
		entry := map[string]any{
			"ts":          time.Now().UTC().Format(time.RFC3339),
			"method":      r.Method,
			"path":        r.URL.Path,
			"status":      status,
			"remote":      stripPort(r.RemoteAddr),
			"duration_ms": dur.Milliseconds(),
			"bytes":       bytes,
		}
		_ = json.NewEncoder(out).Encode(entry)
		return
	}
	// text 格式
	fmt.Fprintf(out, "[%s] %d %s\t%s\t%s\t(%s)\n",
		r.Method, status, r.URL.Path,
		stripPort(r.RemoteAddr),
		formatDuration(dur),
		formatBytes(bytes),
	)
}

func formatDuration(d time.Duration) string {
	if d < time.Millisecond {
		return fmt.Sprintf("%dµs", d.Microseconds())
	}
	return fmt.Sprintf("%dms", d.Milliseconds())
}

func formatBytes(b int64) string {
	if b == 0 {
		return "-"
	}
	if b < 1024 {
		return fmt.Sprintf("%dB", b)
	}
	if b < 1024*1024 {
		return fmt.Sprintf("%.1fKB", float64(b)/1024)
	}
	if b < 1024*1024*1024 {
		return fmt.Sprintf("%.1fMB", float64(b)/(1024*1024))
	}
	return fmt.Sprintf("%.1fGB", float64(b)/(1024*1024*1024))
}
