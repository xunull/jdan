package netprobe

import (
	"context"
	"crypto/tls"
	"net/http"
	"net/http/httptest"
	"net/url"
	"strings"
	"testing"
	"time"
)

func TestParseTarget_FullURL(t *testing.T) {
	tg, err := ParseTarget("https://github.com/foo")
	if err != nil {
		t.Fatal(err)
	}
	if tg.Scheme != "https" || tg.Host != "github.com" || tg.Port != 443 || tg.Path != "/foo" {
		t.Errorf("got %+v", tg)
	}
}

func TestParseTarget_HostOnly(t *testing.T) {
	tg, _ := ParseTarget("example.com")
	if tg.Scheme != "https" || tg.Port != 443 || tg.Path != "/" {
		t.Errorf("got %+v", tg)
	}
}

func TestParseTarget_HostPort_8080(t *testing.T) {
	tg, _ := ParseTarget("example.com:8080")
	if tg.Scheme != "http" || tg.Port != 8080 {
		t.Errorf("port 8080 should infer http, got %+v", tg)
	}
}

func TestParseTarget_HostPort_443(t *testing.T) {
	tg, _ := ParseTarget("example.com:443")
	if tg.Scheme != "https" {
		t.Errorf("port 443 should infer https, got %+v", tg)
	}
}

func TestParseTarget_IPv4Port(t *testing.T) {
	tg, _ := ParseTarget("192.168.1.1:8080")
	if tg.Host != "192.168.1.1" || tg.Port != 8080 {
		t.Errorf("got %+v", tg)
	}
}

func TestParseTarget_IPv6Literal(t *testing.T) {
	tg, err := ParseTarget("[::1]:8080")
	if err != nil {
		t.Fatal(err)
	}
	if tg.Host != "::1" || tg.Port != 8080 {
		t.Errorf("IPv6 literal got %+v", tg)
	}
}

func TestParseTarget_RejectsEmpty(t *testing.T) {
	if _, err := ParseTarget(""); err == nil {
		t.Error("empty should error")
	}
}

func TestParseTarget_RejectsInvalidPort(t *testing.T) {
	if _, err := ParseTarget("example.com:abc"); err == nil {
		t.Error("non-numeric port should error")
	}
}

func TestProbe_RealLocalHTTPServer(t *testing.T) {
	// 起一个 httptest server，probe 它，应该 4 阶段（DNS 跳过—literal） 通过
	ts := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Server", "test-server")
		w.WriteHeader(http.StatusOK)
		_, _ = w.Write([]byte("hello"))
	}))
	defer ts.Close()

	u, _ := url.Parse(ts.URL)
	target := "http://" + u.Host

	res, err := Probe(context.Background(), target, Options{
		Timeout: 2 * time.Second,
	}, nil)
	if err != nil {
		t.Fatal(err)
	}
	if !res.OK {
		t.Errorf("probe should succeed, got %+v", res)
	}

	stages := stageMap(res)
	if stages[StageResolve] == nil || !stages[StageResolve].Success {
		t.Error("resolve missing or failed")
	}
	if stages[StageTCP] == nil || !stages[StageTCP].Success {
		t.Error("tcp missing or failed")
	}
	if stages[StageHTTP] == nil || !stages[StageHTTP].Success {
		t.Error("http missing or failed")
	}
	// 不是 https，所以 TLS 阶段不应存在
	if stages[StageTLS] != nil {
		t.Error("TLS stage should not run for http://")
	}

	h := stages[StageHTTP].HTTP
	if h.Status != 200 {
		t.Errorf("status %d", h.Status)
	}
	if h.Server != "test-server" {
		t.Errorf("server header lost: %q", h.Server)
	}
}

func TestProbe_TLSWithHTTPS(t *testing.T) {
	ts := httptest.NewTLSServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusOK)
	}))
	defer ts.Close()

	u, _ := url.Parse(ts.URL)
	target := "https://" + u.Host

	res, err := Probe(context.Background(), target, Options{
		Timeout:  2 * time.Second,
		Insecure: true, // httptest 自签
	}, nil)
	if err != nil {
		t.Fatal(err)
	}
	if !res.OK {
		t.Errorf("probe should succeed: stopped at %s", res.Stopped)
		for _, s := range res.Stages {
			t.Logf("  stage %s: success=%v err=%q", s.Stage, s.Success, s.Err)
		}
	}

	stages := stageMap(res)
	if stages[StageTLS] == nil {
		t.Fatal("TLS stage should run for https://")
	}
	if !stages[StageTLS].Success {
		t.Errorf("TLS failed: %s", stages[StageTLS].Err)
	}
	tlsD := stages[StageTLS].TLS
	if tlsD.Version == "" || !strings.HasPrefix(tlsD.Version, "TLS") {
		t.Errorf("TLS version unexpected: %q", tlsD.Version)
	}
}

func TestProbe_ConnRefusedHasHint(t *testing.T) {
	// 探查一个没人 listen 的 port
	res, err := Probe(context.Background(), "127.0.0.1:1", Options{
		Timeout: 500 * time.Millisecond,
	}, nil)
	if err != nil {
		t.Fatal(err)
	}
	if res.OK {
		t.Error("probe should fail on closed port")
	}
	if res.Stopped != StageTCP {
		t.Errorf("expected to stop at TCP, got %s", res.Stopped)
	}
	stages := stageMap(res)
	if stages[StageTCP] == nil || stages[StageTCP].Hint == "" {
		t.Error("expected TCP failure to carry hint")
	}
	if !strings.Contains(stages[StageTCP].Hint, "selfcheck") {
		t.Errorf("hint should mention selfcheck cross-ref, got: %s", stages[StageTCP].Hint)
	}
}

func TestProbe_UnreachableResolverHasHint(t *testing.T) {
	// 用一个不可达的 resolver 强制 DNS 阶段失败。
	// 不用 "fake hostname" 是因为很多家用 Wi-Fi 有 DNS hijacking，
	// 会把不存在的域名劫持到 portal 页面，让 resolve 错误地"成功"。
	res, err := Probe(context.Background(), "example.com", Options{
		Timeout:  500 * time.Millisecond,
		Resolver: "127.0.0.1:1", // 这个端口没人监听
	}, nil)
	if err != nil {
		t.Fatal(err)
	}
	if res.OK {
		t.Error("probe should fail when resolver unreachable")
	}
	if res.Stopped != StageResolve {
		t.Errorf("expected to stop at resolve, got %s", res.Stopped)
	}
	stages := stageMap(res)
	if stages[StageResolve].Hint == "" {
		t.Error("resolve failure should carry hint")
	}
}

func TestProbe_EmitCallback(t *testing.T) {
	ts := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(200)
	}))
	defer ts.Close()
	u, _ := url.Parse(ts.URL)

	var emitted []Stage
	_, err := Probe(context.Background(), "http://"+u.Host, Options{
		Timeout:        2 * time.Second,
		HealthDuration: 100 * time.Millisecond, // 测试时不等 1s
	}, func(s *StageResult) {
		emitted = append(emitted, s.Stage)
	})
	if err != nil {
		t.Fatal(err)
	}
	// 应当看到 resolve → tcp → tcp_health → http（http scheme 没有 tls）
	want := []Stage{StageResolve, StageTCP, StageTCPHealth, StageHTTP}
	if len(emitted) != len(want) {
		t.Errorf("expected %v emit, got %v", want, emitted)
	}
	for i, s := range emitted {
		if i >= len(want) {
			break
		}
		if s != want[i] {
			t.Errorf("emit[%d] = %s, want %s", i, s, want[i])
		}
	}
}

func TestProbe_SkipHealthOmitsTcpHealthStage(t *testing.T) {
	ts := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(200)
	}))
	defer ts.Close()
	u, _ := url.Parse(ts.URL)

	var emitted []Stage
	_, err := Probe(context.Background(), "http://"+u.Host, Options{
		Timeout:    2 * time.Second,
		SkipHealth: true,
	}, func(s *StageResult) {
		emitted = append(emitted, s.Stage)
	})
	if err != nil {
		t.Fatal(err)
	}
	for _, s := range emitted {
		if s == StageTCPHealth {
			t.Error("SkipHealth=true should not emit tcp_health stage")
		}
	}
}

func TestClassifyHTTPStatus(t *testing.T) {
	for _, tc := range []struct {
		code int
		want ErrorClass
	}{
		{200, ClassNone},
		{301, ClassNone},
		{401, ClassHTTPClientError},
		{404, ClassHTTPClientError},
		{418, ClassHTTPClientError},
		{500, ClassHTTPServerError},
		{503, ClassHTTPServerError},
		{599, ClassHTTPServerError},
	} {
		got := ClassifyHTTPStatus(tc.code)
		if got != tc.want {
			t.Errorf("ClassifyHTTPStatus(%d) = %s, want %s", tc.code, got, tc.want)
		}
	}
}

func TestTLSVersionString(t *testing.T) {
	if got := tlsVersionString(tls.VersionTLS13); got != "TLS 1.3" {
		t.Errorf("got %q", got)
	}
	if got := tlsVersionString(tls.VersionTLS12); got != "TLS 1.2" {
		t.Errorf("got %q", got)
	}
}

func stageMap(r *Result) map[Stage]*StageResult {
	m := make(map[Stage]*StageResult)
	for _, s := range r.Stages {
		m[s.Stage] = s
	}
	return m
}
