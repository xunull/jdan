package cli

import (
	"bytes"
	"encoding/json"
	"io"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
)

func runCSP(t *testing.T, in io.Reader, args ...string) (string, error) {
	t.Helper()
	var out bytes.Buffer
	cmd := newCSPCommand(cspCmdDeps{out: &out, in: in})
	cmd.SetArgs(args)
	cmd.SetOut(io.Discard)
	cmd.SetErr(io.Discard)
	err := cmd.Execute()
	return out.String(), err
}

func runCookie(t *testing.T, in io.Reader, args ...string) (string, error) {
	t.Helper()
	var out bytes.Buffer
	cmd := newCookieCommand(cookieCmdDeps{out: &out, in: in})
	cmd.SetArgs(args)
	cmd.SetOut(io.Discard)
	cmd.SetErr(io.Discard)
	err := cmd.Execute()
	return out.String(), err
}

func TestCSP_Literal(t *testing.T) {
	out, err := runCSP(t, nil, "script-src 'self' 'unsafe-inline'")
	if err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(out, "unsafe-inline") || !strings.Contains(out, "default-src") {
		t.Errorf("应解析并体检:\n%s", out)
	}
}

func TestCSP_Stdin(t *testing.T) {
	out, err := runCSP(t, strings.NewReader("default-src 'self'"))
	if err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(out, "default-src") {
		t.Errorf("stdin 未解析:\n%s", out)
	}
}

func TestCSP_URL(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Security-Policy", "default-src 'self'; script-src 'self' 'unsafe-eval'")
		w.WriteHeader(200)
	}))
	defer srv.Close()
	out, err := runCSP(t, nil, srv.URL)
	if err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(out, "unsafe-eval") {
		t.Errorf("URL 抓取+体检失败:\n%s", out)
	}
}

func TestCSP_NoHeader(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(200)
	}))
	defer srv.Close()
	_, err := runCSP(t, nil, srv.URL)
	if err == nil || !strings.Contains(err.Error(), "没有 Content-Security-Policy") {
		t.Errorf("无 CSP 头应报错，got %v", err)
	}
}

func TestCSP_JSON(t *testing.T) {
	out, err := runCSP(t, nil, "script-src 'self'", "--json")
	if err != nil {
		t.Fatal(err)
	}
	var got map[string]any
	if err := json.Unmarshal([]byte(out), &got); err != nil {
		t.Fatalf("非法 JSON: %v\n%s", err, out)
	}
	if got["directives"] == nil || got["issues"] == nil {
		t.Errorf("json 缺字段: %v", got)
	}
}

func TestCookie_Literal(t *testing.T) {
	out, err := runCookie(t, nil, "sid=abc; Path=/")
	if err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(out, "sid = abc") || !strings.Contains(out, "Secure") {
		t.Errorf("应解析并体检:\n%s", out)
	}
}

func TestCookie_URL_Multiple(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Add("Set-Cookie", "a=1; Secure; HttpOnly; SameSite=Lax")
		w.Header().Add("Set-Cookie", "b=2")
		w.WriteHeader(200)
	}))
	defer srv.Close()
	out, err := runCookie(t, nil, srv.URL)
	if err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(out, "a = 1") || !strings.Contains(out, "b = 2") {
		t.Errorf("应解析多条 Set-Cookie:\n%s", out)
	}
}

func TestCookie_Request(t *testing.T) {
	out, err := runCookie(t, nil, "--request", "a=1; b=2; sid=abc")
	if err != nil {
		t.Fatal(err)
	}
	for _, want := range []string{"a", "b", "sid", "abc"} {
		if !strings.Contains(out, want) {
			t.Errorf("--request 应列出 %q:\n%s", want, out)
		}
	}
}

func TestCookie_JSON(t *testing.T) {
	out, err := runCookie(t, nil, "sid=x; Secure", "--json")
	if err != nil {
		t.Fatal(err)
	}
	var got map[string]any
	if err := json.Unmarshal([]byte(out), &got); err != nil {
		t.Fatalf("非法 JSON: %v\n%s", err, out)
	}
	if got["cookies"] == nil {
		t.Errorf("json 缺 cookies: %v", got)
	}
}
