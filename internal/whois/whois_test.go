package whois

import (
	"bufio"
	"context"
	"fmt"
	"net"
	"strings"
	"sync"
	"testing"
	"time"
)

// mockServer 是 in-process WHOIS server，按 (target → response) 字典回复。
// 用于端到端测试 Query / Lookup 而不依赖外网。
type mockServer struct {
	listener net.Listener
	addr     string
	mu       sync.Mutex
	queries  []string // 收到的所有 query（顺序）
	closed   chan struct{}
}

// newMockServer 起一个 mockServer，handler 接收 query 字符串返回 response。
func newMockServer(t *testing.T, handler func(query string) string) *mockServer {
	t.Helper()
	l, err := net.Listen("tcp", "127.0.0.1:0")
	if err != nil {
		t.Fatal(err)
	}
	m := &mockServer{
		listener: l,
		addr:     l.Addr().String(),
		closed:   make(chan struct{}),
	}
	go func() {
		defer close(m.closed)
		for {
			conn, err := l.Accept()
			if err != nil {
				return
			}
			go func(c net.Conn) {
				defer c.Close()
				_ = c.SetReadDeadline(time.Now().Add(2 * time.Second))
				r := bufio.NewReader(c)
				line, err := r.ReadString('\n')
				if err != nil && line == "" {
					return
				}
				query := strings.TrimSpace(line)
				m.mu.Lock()
				m.queries = append(m.queries, query)
				m.mu.Unlock()
				resp := handler(query)
				_, _ = c.Write([]byte(resp))
			}(conn)
		}
	}()
	t.Cleanup(func() {
		_ = l.Close()
		<-m.closed
	})
	return m
}

func (m *mockServer) lastQueries() []string {
	m.mu.Lock()
	defer m.mu.Unlock()
	out := make([]string, len(m.queries))
	copy(out, m.queries)
	return out
}

func TestQuery_RoundTrip(t *testing.T) {
	m := newMockServer(t, func(query string) string {
		return fmt.Sprintf("Got: %s\nStatus: OK\n", query)
	})
	ctx := context.Background()
	raw, err := Query(ctx, m.addr, "example.com", time.Second)
	if err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(raw, "Got: example.com") {
		t.Errorf("response missing query echo: %q", raw)
	}
	if !strings.Contains(raw, "Status: OK") {
		t.Errorf("response missing status: %q", raw)
	}
	if q := m.lastQueries(); len(q) != 1 || q[0] != "example.com" {
		t.Errorf("server received %v", q)
	}
}

func TestQuery_AddsDefaultPort(t *testing.T) {
	// 含 ':' 的 server 不补端口（用 mock listener 上的真实 host:port 验证）
	m := newMockServer(t, func(string) string { return "" })
	ctx := context.Background()
	_, err := Query(ctx, m.addr, "example.com", 500*time.Millisecond)
	if err != nil {
		t.Errorf("with-port server should not error: %v", err)
	}
	// 不带端口 → 自动补 :43；本地 127.0.0.1:43 应当 connection refused（除非 root 起了 server）
	_, err = Query(ctx, "127.0.0.1", "example.com", 200*time.Millisecond)
	if err == nil {
		t.Error("expected error connecting to 127.0.0.1:43 (no server running)")
	}
}

func TestQuery_TimeoutHonored(t *testing.T) {
	// Mock server 故意不回包，触发 read timeout
	l, _ := net.Listen("tcp", "127.0.0.1:0")
	defer l.Close()
	go func() {
		conn, err := l.Accept()
		if err != nil {
			return
		}
		// 收下连接，永不回复
		_ = conn
		time.Sleep(5 * time.Second)
	}()
	ctx := context.Background()
	start := time.Now()
	_, err := Query(ctx, l.Addr().String(), "example.com", 200*time.Millisecond)
	elapsed := time.Since(start)
	if err == nil {
		t.Error("expected timeout error")
	}
	if elapsed > time.Second {
		t.Errorf("timeout not enforced (took %v)", elapsed)
	}
}

func TestLookupWithServer_BypassesRouting(t *testing.T) {
	m := newMockServer(t, func(query string) string {
		return "% custom server\nstatus: ok\n"
	})
	ctx := context.Background()
	res, err := LookupWithServer(ctx, "example.com", m.addr, time.Second)
	if err != nil {
		t.Fatal(err)
	}
	if res.Server != m.addr {
		t.Errorf("server = %q, want %q", res.Server, m.addr)
	}
	if !strings.Contains(res.RawText, "custom server") {
		t.Errorf("raw = %q", res.RawText)
	}
}

func TestLookup_IANAFallback_FollowsReferral(t *testing.T) {
	// 真实 tld server（mock #2）
	tldServer := newMockServer(t, func(query string) string {
		return fmt.Sprintf("Domain Name: %s\nRegistrar: Mock Registrar\n", query)
	})
	// IANA root mock：返回 referral 指向 tld server
	ianaServer := newMockServer(t, func(query string) string {
		return fmt.Sprintf("%% IANA root\nwhois: %s\nstatus: ACTIVE\n", tldServer.addr)
	})

	// 触发 IANA 跟随逻辑的关键是 cur == IANARoot。monkey-patch package var。
	origIANA := IANARoot
	IANARoot = ianaServer.addr
	defer func() { IANARoot = origIANA }()

	ctx := context.Background()
	res, err := lookupChain(ctx, "test.unknown-tld-9999", KindDomain, ianaServer.addr, time.Second)
	if err != nil {
		t.Fatal(err)
	}
	if res.Server != tldServer.addr {
		t.Errorf("final server = %q, want %q (tld)", res.Server, tldServer.addr)
	}
	if len(res.Hops) != 1 || res.Hops[0].Server != ianaServer.addr {
		t.Errorf("hops = %+v, want 1 hop through IANA", res.Hops)
	}
	if !strings.Contains(res.RawText, "Mock Registrar") {
		t.Errorf("final raw = %q", res.RawText)
	}
}

func TestLookup_IPReferralChain(t *testing.T) {
	// 终点 server（RIPE）
	ripeServer := newMockServer(t, func(query string) string {
		return fmt.Sprintf("inetnum: %s\norigin: RIPE\n", query)
	})
	// ARIN mock：返回 ReferralServer 指向 RIPE
	arinServer := newMockServer(t, func(query string) string {
		return fmt.Sprintf("NetRange: 1.2.3.0/24\nReferralServer: whois://%s\n", ripeServer.addr)
	})

	ctx := context.Background()
	res, err := lookupChain(ctx, "1.2.3.4", KindIPv4, arinServer.addr, time.Second)
	if err != nil {
		t.Fatal(err)
	}
	if res.Server != ripeServer.addr {
		t.Errorf("final server = %q, want %q", res.Server, ripeServer.addr)
	}
	if len(res.Hops) != 1 || res.Hops[0].Server != arinServer.addr {
		t.Errorf("hops = %+v", res.Hops)
	}
	if !strings.Contains(res.RawText, "origin: RIPE") {
		t.Errorf("raw = %q", res.RawText)
	}
}

func TestLookup_NoReferral_StopsAtFirstServer(t *testing.T) {
	tldServer := newMockServer(t, func(query string) string {
		return fmt.Sprintf("Domain: %s\nRegistrar: X\n", query)
	})
	ctx := context.Background()
	res, err := lookupChain(ctx, "example.com", KindDomain, tldServer.addr, time.Second)
	if err != nil {
		t.Fatal(err)
	}
	if res.Server != tldServer.addr {
		t.Errorf("server = %q", res.Server)
	}
	if len(res.Hops) != 0 {
		t.Errorf("expected 0 hops, got %+v", res.Hops)
	}
}
