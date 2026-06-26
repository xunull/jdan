package httphdr

import (
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
)

// ---- EnsureScheme ----

func TestEnsureScheme(t *testing.T) {
	cases := []struct{ in, want string }{
		{"example.com", "https://example.com"},
		{"http://x.com", "http://x.com"},
		{"https://x.com", "https://x.com"},
		{"example.com:8080/path", "https://example.com:8080/path"},
	}
	for _, c := range cases {
		if got := EnsureScheme(c.in); got != c.want {
			t.Errorf("EnsureScheme(%q) = %q, want %q", c.in, got, c.want)
		}
	}
}

// ---- Fetch ----

func TestFetch_Single(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("X-Test", "hi")
		w.WriteHeader(200)
	}))
	defer srv.Close()

	hops, err := Fetch(srv.Client(), srv.URL, "GET", nil, 10)
	if err != nil {
		t.Fatal(err)
	}
	if len(hops) != 1 {
		t.Fatalf("want 1 hop, got %d", len(hops))
	}
	if hops[0].StatusCode != 200 || hops[0].Header.Get("X-Test") != "hi" {
		t.Errorf("bad hop: %+v", hops[0])
	}
}

func TestFetch_RedirectChain(t *testing.T) {
	mux := http.NewServeMux()
	mux.HandleFunc("/", func(w http.ResponseWriter, r *http.Request) {
		http.Redirect(w, r, "/final", http.StatusMovedPermanently) // 301 → 相对 /final
	})
	mux.HandleFunc("/final", func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(200)
	})
	srv := httptest.NewServer(mux)
	defer srv.Close()

	hops, err := Fetch(srv.Client(), srv.URL+"/", "GET", nil, 10)
	if err != nil {
		t.Fatal(err)
	}
	if len(hops) != 2 {
		t.Fatalf("want 2 hops, got %d: %+v", len(hops), hops)
	}
	if hops[0].StatusCode != 301 {
		t.Errorf("hop0 = %d, want 301", hops[0].StatusCode)
	}
	if !strings.HasSuffix(hops[1].URL, "/final") {
		t.Errorf("relative Location not resolved: %q", hops[1].URL)
	}
	if hops[1].StatusCode != 200 {
		t.Errorf("hop1 = %d, want 200", hops[1].StatusCode)
	}
}

func TestFetch_NoFollow(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		http.Redirect(w, r, "/x", http.StatusFound)
	}))
	defer srv.Close()

	hops, _ := Fetch(srv.Client(), srv.URL, "GET", nil, 0) // max=0 → 不跟
	if len(hops) != 1 {
		t.Fatalf("max-redirects 0 should give 1 hop, got %d", len(hops))
	}
	if hops[0].StatusCode != 302 {
		t.Errorf("should stop at the redirect, got %d", hops[0].StatusCode)
	}
}

func TestFetch_LoopCappedByMax(t *testing.T) {
	// 永远 302 到自己 → 必须被 max-redirects 截断，不无限
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		http.Redirect(w, r, "/loop", http.StatusFound)
	}))
	defer srv.Close()

	hops, _ := Fetch(srv.Client(), srv.URL+"/loop", "GET", nil, 3)
	if len(hops) != 4 { // 3 跳 + 第 4 跳到上限停
		t.Fatalf("loop should be capped at max+1=4 hops, got %d", len(hops))
	}
}

func TestFetch_Method(t *testing.T) {
	var gotMethod string
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		gotMethod = r.Method
		w.WriteHeader(200)
	}))
	defer srv.Close()

	if _, err := Fetch(srv.Client(), srv.URL, "HEAD", nil, 10); err != nil {
		t.Fatal(err)
	}
	if gotMethod != "HEAD" {
		t.Errorf("method = %q, want HEAD", gotMethod)
	}
}

func TestFetch_RequestHeader(t *testing.T) {
	var gotAuth string
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		gotAuth = r.Header.Get("Authorization")
		w.WriteHeader(200)
	}))
	defer srv.Close()

	h := http.Header{}
	h.Set("Authorization", "Bearer xyz")
	if _, err := Fetch(srv.Client(), srv.URL, "GET", h, 10); err != nil {
		t.Fatal(err)
	}
	if gotAuth != "Bearer xyz" {
		t.Errorf("request header not sent: %q", gotAuth)
	}
}

func TestFetch_MultiValueHeader(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Add("Set-Cookie", "a=1")
		w.Header().Add("Set-Cookie", "b=2")
		w.WriteHeader(200)
	}))
	defer srv.Close()

	hops, _ := Fetch(srv.Client(), srv.URL, "GET", nil, 10)
	if got := hops[0].Header["Set-Cookie"]; len(got) != 2 {
		t.Errorf("want 2 Set-Cookie values, got %v", got)
	}
}

func TestFetch_ConnError(t *testing.T) {
	// 指向一个不监听的端口 → 返回 error，hops 为空
	hops, err := Fetch(http.DefaultClient, "http://127.0.0.1:1", "GET", nil, 10)
	if err == nil {
		t.Error("connection error should surface")
	}
	if len(hops) != 0 {
		t.Errorf("no successful hops expected, got %d", len(hops))
	}
}

func TestFetch_DoesNotMutateClient(t *testing.T) {
	// Fetch 用浅拷贝设 CheckRedirect，不该改到调用方 client
	c := &http.Client{}
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(200)
	}))
	defer srv.Close()
	_, _ = Fetch(c, srv.URL, "GET", nil, 10)
	if c.CheckRedirect != nil {
		t.Error("Fetch must not set CheckRedirect on the caller's client")
	}
}

// ---- FormatText ----

func hop(code int, status string, hdr map[string]string) Hop {
	h := http.Header{}
	for k, v := range hdr {
		h.Set(k, v)
	}
	return Hop{Status: status, StatusCode: code, Header: h}
}

func TestFormatText_RedirectThenFinal(t *testing.T) {
	hops := []Hop{
		hop(301, "301 Moved Permanently", map[string]string{"Location": "https://x/", "Server": "nginx"}),
		hop(200, "200 OK", map[string]string{"Content-Type": "text/html"}),
	}
	out := FormatText(hops, false)
	// 重定向跳只显 Location（不显 Server）
	if !strings.Contains(out, "301 Moved Permanently") || !strings.Contains(out, "  Location: https://x/") {
		t.Errorf("redirect hop format wrong:\n%s", out)
	}
	if strings.Contains(out, "Server: nginx") {
		t.Errorf("redirect hop should not show all headers without -a:\n%s", out)
	}
	// 箭头 + 最终跳显全部头
	if !strings.Contains(out, "→ 200 OK") || !strings.Contains(out, "Content-Type: text/html") {
		t.Errorf("final hop format wrong:\n%s", out)
	}
}

func TestFormatText_ShowAll(t *testing.T) {
	hops := []Hop{
		hop(301, "301 Moved Permanently", map[string]string{"Location": "https://x/", "Server": "nginx"}),
		hop(200, "200 OK", map[string]string{"Content-Type": "text/html"}),
	}
	out := FormatText(hops, true)
	if !strings.Contains(out, "Server: nginx") {
		t.Errorf("-a should show all headers on redirect hop too:\n%s", out)
	}
}

func TestFormatText_SortedHeaders(t *testing.T) {
	hops := []Hop{hop(200, "200 OK", map[string]string{"Zebra": "z", "Alpha": "a"})}
	out := FormatText(hops, false)
	if strings.Index(out, "Alpha") > strings.Index(out, "Zebra") {
		t.Errorf("headers should be sorted:\n%s", out)
	}
}

// ---- FormatJSON ----

func TestFormatJSON(t *testing.T) {
	hops := []Hop{
		hop(301, "301", map[string]string{"Location": "https://x/"}),
		hop(200, "200 OK", map[string]string{"Content-Type": "text/html"}),
	}
	s, err := FormatJSON(hops)
	if err != nil {
		t.Fatal(err)
	}
	var got []map[string]any
	if err := json.Unmarshal([]byte(s), &got); err != nil {
		t.Fatalf("invalid JSON: %v\n%s", err, s)
	}
	if len(got) != 2 {
		t.Fatalf("want 2 elements, got %d", len(got))
	}
	if got[0]["location"] != "https://x/" {
		t.Errorf("location field wrong: %v", got[0]["location"])
	}
	if got[1]["status_code"].(float64) != 200 {
		t.Errorf("status_code wrong: %v", got[1]["status_code"])
	}
}

func TestFormatJSON_Empty(t *testing.T) {
	s, err := FormatJSON(nil)
	if err != nil {
		t.Fatal(err)
	}
	if strings.TrimSpace(s) != "[]" {
		t.Errorf("empty should be [], got %q", s)
	}
}
