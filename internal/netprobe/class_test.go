package netprobe

import (
	"context"
	"errors"
	"io"
	"net"
	"net/http"
	"net/http/httptest"
	"net/url"
	"strings"
	"syscall"
	"testing"
	"time"
)

func TestClassifyTCPError_RealConnRefused(t *testing.T) {
	// 真实触发 connection refused：dial 没人监听的 localhost 端口
	_, err := net.DialTimeout("tcp", "127.0.0.1:1", 500*time.Millisecond)
	if err == nil {
		t.Skip("127.0.0.1:1 unexpectedly accepted connection")
	}
	if got := ClassifyTCPError(err); got != ClassConnRefused {
		t.Errorf("real connection refused → %s, want CONNECTION_REFUSED", got)
	}
}

func TestClassifyTCPError_TimeoutInterface(t *testing.T) {
	// 用真实 dial 触发 timeout —— dial 一个不可达的 RFC 5737 IP，超时短
	_, err := net.DialTimeout("tcp", "192.0.2.1:1", 100*time.Millisecond)
	if err == nil {
		t.Skip("192.0.2.1 unexpectedly reachable")
	}
	got := ClassifyTCPError(err)
	if got != ClassConnTimeout && got != ClassNoRoute && got != ClassNetUnreachable {
		// Different OS/network conditions can yield different errors;
		// timeout / no-route / unreachable are all acceptable here
		t.Errorf("unreachable IP → %s, want one of TIMEOUT/NO_ROUTE/UNREACHABLE", got)
	}
}

func TestClassifyTCPError_ECONNREFUSED_Errno(t *testing.T) {
	// 直接构造一个包 ECONNREFUSED 的 *net.OpError
	opErr := &net.OpError{
		Op:  "dial",
		Net: "tcp",
		Err: &net.AddrError{Err: "fallback"},
	}
	// 包装一层让 errors.Is 能找到 ECONNREFUSED
	wrapped := &wrappedErr{wrap: syscall.ECONNREFUSED, str: opErr.Error() + ": connection refused"}
	if got := ClassifyTCPError(wrapped); got != ClassConnRefused {
		t.Errorf("got %s, want CONNECTION_REFUSED", got)
	}
}

func TestClassifyTCPError_ENETUNREACH(t *testing.T) {
	wrapped := &wrappedErr{wrap: syscall.ENETUNREACH, str: "network is unreachable"}
	if got := ClassifyTCPError(wrapped); got != ClassNetUnreachable {
		t.Errorf("got %s, want NETWORK_UNREACHABLE", got)
	}
}

func TestClassifyTCPError_EHOSTUNREACH(t *testing.T) {
	wrapped := &wrappedErr{wrap: syscall.EHOSTUNREACH, str: "no route to host"}
	if got := ClassifyTCPError(wrapped); got != ClassNoRoute {
		t.Errorf("got %s, want NO_ROUTE_TO_HOST", got)
	}
}

func TestClassifyTCPError_StringFallback(t *testing.T) {
	// 一个 plain error 走字符串匹配路径
	err := errors.New("dial tcp 10.0.0.5:80: connect: connection refused")
	if got := ClassifyTCPError(err); got != ClassConnRefused {
		t.Errorf("string fallback got %s, want CONNECTION_REFUSED", got)
	}
}

func TestClassifyTCPError_Nil(t *testing.T) {
	if got := ClassifyTCPError(nil); got != ClassNone {
		t.Errorf("nil err → %s, want ClassNone", got)
	}
}

func TestClassifyTCPError_Unknown(t *testing.T) {
	err := errors.New("completely unrecognizable error")
	if got := ClassifyTCPError(err); got != ClassUnknown {
		t.Errorf("got %s, want UNKNOWN", got)
	}
}

func TestClassifyTCPHealthError_RemoteReset(t *testing.T) {
	wrapped := &wrappedErr{wrap: syscall.ECONNRESET, str: "read: connection reset by peer"}
	if got := ClassifyTCPHealthError(wrapped); got != ClassRemoteReset {
		t.Errorf("got %s, want REMOTE_RESET", got)
	}
}

func TestClassifyTCPHealthError_EOF(t *testing.T) {
	if got := ClassifyTCPHealthError(io.EOF); got != ClassRemoteEOF {
		t.Errorf("got %s, want REMOTE_CLOSED", got)
	}
}

func TestClassifyDNSError_NoSuchHost(t *testing.T) {
	// 用真实 lookup 触发 no such host
	_, err := net.DefaultResolver.LookupIP(context.Background(), "ip",
		"definitely-not-a-real-host-xyz-abc.invalid.")
	if err == nil {
		t.Skip("DNS hijacking detected; cannot test NoSuchHost path")
	}
	got := ClassifyDNSError(err)
	if got != ClassDNSNoSuchHost && got != ClassDNSTimeout {
		// 某些网络 DNS hijacking 可能给 timeout，两者都是合理结果
		t.Errorf("got %s, want DNS_NO_SUCH_HOST or DNS_TIMEOUT", got)
	}
}

func TestClassifyDNSError_ResolverDown(t *testing.T) {
	// 用 unreachable resolver
	r, _ := buildResolver("127.0.0.1:1")
	ctx, cancel := context.WithTimeout(context.Background(), 200*time.Millisecond)
	defer cancel()
	_, err := r.LookupIP(ctx, "ip", "example.com")
	if err == nil {
		t.Skip("port 1 unexpectedly responding")
	}
	got := ClassifyDNSError(err)
	if got != ClassDNSResolverDown && got != ClassDNSTimeout {
		t.Errorf("got %s, want DNS_RESOLVER_UNREACHABLE or DNS_TIMEOUT", got)
	}
}

func TestClassifyTLSError_SelfSigned(t *testing.T) {
	err := errors.New("x509: certificate signed by unknown authority")
	if got := ClassifyTLSError(err); got != ClassTLSCertInvalid {
		t.Errorf("got %s, want TLS_CERT_INVALID", got)
	}
}

func TestClassifyTLSError_NotHTTPS(t *testing.T) {
	err := errors.New("tls: first record does not look like a TLS handshake")
	if got := ClassifyTLSError(err); got != ClassTLSNotHTTPS {
		t.Errorf("got %s, want TLS_PLAIN_HTTP_ON_TLS_PORT", got)
	}
}

func TestClassifyTLSError_Handshake(t *testing.T) {
	err := errors.New("tls: handshake failure")
	if got := ClassifyTLSError(err); got != ClassTLSHandshakeFail {
		t.Errorf("got %s, want TLS_HANDSHAKE_FAIL", got)
	}
}

func TestClassifyHTTPError_Reset(t *testing.T) {
	err := errors.New("read: connection reset by peer")
	if got := ClassifyHTTPError(err); got != ClassRemoteReset {
		t.Errorf("got %s, want REMOTE_RESET", got)
	}
}

func TestClassifyHTTPError_EOF(t *testing.T) {
	if got := ClassifyHTTPError(io.EOF); got != ClassRemoteEOF {
		t.Errorf("got %s, want REMOTE_CLOSED", got)
	}
}

func TestClassCatalog_AllClassesHaveInfo(t *testing.T) {
	// 编译期保证：每个 ErrorClass 都有 whatItMeans + hint 文案
	for _, c := range []ErrorClass{
		ClassDNSNoSuchHost, ClassDNSResolverDown, ClassDNSTimeout,
		ClassConnRefused, ClassConnTimeout, ClassNoRoute, ClassNetUnreachable,
		ClassRemoteReset, ClassRemoteEOF,
		ClassTLSCertInvalid, ClassTLSHandshakeFail, ClassTLSNotHTTPS,
		ClassHTTPProtocolErr, ClassHTTPClientError, ClassHTTPServerError,
		ClassUnknown,
	} {
		if WhatItMeans(c) == "" {
			t.Errorf("class %s missing WhatItMeans", c)
		}
		if HintForClass(c) == "" {
			t.Errorf("class %s missing Hint", c)
		}
	}
}

func TestTCPHealth_HealthyHTTPServer(t *testing.T) {
	ts := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(200)
	}))
	defer ts.Close()
	u, _ := url.Parse(ts.URL)
	host, portStr := splitHostPort(u.Host)
	tgt := &Target{Host: host, Port: parsePort(portStr)}

	res := runTCPHealth(context.Background(), tgt, net.ParseIP("127.0.0.1"), Options{
		Timeout:        1 * time.Second,
		HealthDuration: 200 * time.Millisecond,
	})
	if !res.Success {
		t.Errorf("HTTP server should pass tcp_health, got class=%s err=%s",
			res.Class, res.Err)
	}
	if res.TCPHealth == nil {
		t.Fatal("TCPHealth detail missing")
	}
	if res.TCPHealth.GotBanner {
		t.Error("HTTP server should not push banner")
	}
}

func TestTCPHealth_RemoteResetSimulated(t *testing.T) {
	// 起一个 TCP listener：accept 后立刻 Close (RST/FIN scenario)
	l, err := net.Listen("tcp", "127.0.0.1:0")
	if err != nil {
		t.Fatal(err)
	}
	go func() {
		for {
			conn, err := l.Accept()
			if err != nil {
				return
			}
			_ = conn.Close()
		}
	}()
	defer l.Close()

	port := l.Addr().(*net.TCPAddr).Port
	tgt := &Target{Host: "127.0.0.1", Port: port}

	res := runTCPHealth(context.Background(), tgt, net.ParseIP("127.0.0.1"), Options{
		Timeout:        500 * time.Millisecond,
		HealthDuration: 200 * time.Millisecond,
	})
	if res.Success {
		t.Errorf("immediately-closed server should fail tcp_health")
	}
	// EOF（FIN）和 RST 都是合法结果，取决于 OS / kernel timing
	if res.Class != ClassRemoteEOF && res.Class != ClassRemoteReset {
		t.Errorf("got class=%s, want REMOTE_CLOSED or REMOTE_RESET", res.Class)
	}
}

func TestTCPHealth_ServerBanner(t *testing.T) {
	// 起一个 listener：accept 后立刻发 banner
	l, err := net.Listen("tcp", "127.0.0.1:0")
	if err != nil {
		t.Fatal(err)
	}
	go func() {
		for {
			conn, err := l.Accept()
			if err != nil {
				return
			}
			_, _ = conn.Write([]byte("SSH-2.0-OpenSSH_8.0\r\n"))
			// 不关，等 client 关
		}
	}()
	defer l.Close()

	port := l.Addr().(*net.TCPAddr).Port
	tgt := &Target{Host: "127.0.0.1", Port: port}

	res := runTCPHealth(context.Background(), tgt, net.ParseIP("127.0.0.1"), Options{
		Timeout:        500 * time.Millisecond,
		HealthDuration: 200 * time.Millisecond,
	})
	if !res.Success {
		t.Errorf("banner case should succeed: class=%s err=%s", res.Class, res.Err)
	}
	if res.TCPHealth == nil || !res.TCPHealth.GotBanner {
		t.Errorf("should detect banner")
	}
	if !strings.Contains(res.TCPHealth.BannerPreview, "SSH-2.0") {
		t.Errorf("banner preview missing SSH header: %s", res.TCPHealth.BannerPreview)
	}
}

// wrappedErr 是一个测试 helper：让 errors.Is(x, wrap) 返回 true 同时
// Error() 给出指定字符串。
type wrappedErr struct {
	wrap error
	str  string
}

func (e *wrappedErr) Error() string { return e.str }
func (e *wrappedErr) Unwrap() error { return e.wrap }

func parsePort(s string) int {
	var p int
	for _, c := range s {
		p = p*10 + int(c-'0')
	}
	return p
}
