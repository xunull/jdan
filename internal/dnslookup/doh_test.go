package dnslookup

import (
	"context"
	"io"
	"net/http"
	"net/http/httptest"
	"reflect"
	"strings"
	"testing"
	"time"

	"github.com/miekg/dns"
)

// --- ResolveDoHTarget ---

func TestResolveDoHTarget_Alias(t *testing.T) {
	got, err := ResolveDoHTarget("google")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if got.URL != "https://dns.google/dns-query" {
		t.Errorf("URL = %q", got.URL)
	}
	if len(got.BootstrapIPs) != 2 || got.BootstrapIPs[0] != "8.8.8.8" {
		t.Errorf("BootstrapIPs = %v", got.BootstrapIPs)
	}
}

func TestResolveDoHTarget_AliasCaseInsensitive(t *testing.T) {
	for _, input := range []string{"Google", "GOOGLE", "  google  "} {
		got, err := ResolveDoHTarget(input)
		if err != nil {
			t.Errorf("%q: %v", input, err)
			continue
		}
		if got.URL != "https://dns.google/dns-query" {
			t.Errorf("%q → URL = %q", input, got.URL)
		}
	}
}

func TestResolveDoHTarget_AllProvidersResolve(t *testing.T) {
	for _, alias := range ProviderAliases() {
		got, err := ResolveDoHTarget(alias)
		if err != nil {
			t.Errorf("%q: %v", alias, err)
			continue
		}
		if !strings.HasPrefix(got.URL, "https://") {
			t.Errorf("%q: URL not https: %q", alias, got.URL)
		}
		if len(got.BootstrapIPs) < 1 {
			t.Errorf("%q: expected ≥1 bootstrap IP", alias)
		}
	}
}

func TestResolveDoHTarget_FullURL(t *testing.T) {
	got, err := ResolveDoHTarget("https://custom.example/dns-query")
	if err != nil {
		t.Fatal(err)
	}
	if got.URL != "https://custom.example/dns-query" {
		t.Errorf("URL = %q", got.URL)
	}
	if len(got.BootstrapIPs) != 0 {
		t.Errorf("expected no bootstrap, got %v", got.BootstrapIPs)
	}
}

func TestResolveDoHTarget_HostnameCompletes(t *testing.T) {
	got, err := ResolveDoHTarget("dns.example.com")
	if err != nil {
		t.Fatal(err)
	}
	if got.URL != "https://dns.example.com/dns-query" {
		t.Errorf("URL = %q", got.URL)
	}
	if len(got.BootstrapIPs) != 0 {
		t.Errorf("hostname should not bring bootstrap, got %v", got.BootstrapIPs)
	}
}

func TestResolveDoHTarget_Errors(t *testing.T) {
	cases := []string{
		"",
		"   ",
		"http://dns.google/dns-query", // scheme 必须 https
		"ftp://dns.example/dns-query",
		"https://",                  // 缺 host
		"dns.example.com/dns-query", // 带 path 但没 scheme → 提示用完整 URL
		"with space.example",        // 含空白
		"dns example com",
	}
	for _, in := range cases {
		t.Run(in, func(t *testing.T) {
			if _, err := ResolveDoHTarget(in); err == nil {
				t.Errorf("expected error for %q", in)
			}
		})
	}
}

func TestResolveDoHTarget_AliasReturnsCopyOfIPs(t *testing.T) {
	// 修改返回值不应影响内部表。
	a, _ := ResolveDoHTarget("google")
	a.BootstrapIPs[0] = "MUTATED"
	b, _ := ResolveDoHTarget("google")
	if b.BootstrapIPs[0] == "MUTATED" {
		t.Error("internal alias table was mutated")
	}
}

// --- IsDoHURL ---

func TestIsDoHURL(t *testing.T) {
	cases := []struct {
		in   string
		want bool
	}{
		{"https://dns.google/dns-query", true},
		{"HTTPS://dns.google/dns-query", true},
		{"http://example.com", false},
		{"8.8.8.8:53", false},
		{"dns.google", false},
		{"", false},
	}
	for _, c := range cases {
		if got := IsDoHURL(c.in); got != c.want {
			t.Errorf("IsDoHURL(%q) = %v, want %v", c.in, got, c.want)
		}
	}
}

// --- resolveDialAddr ---

func TestResolveDialAddr_NoBootstrapPassesThrough(t *testing.T) {
	got := resolveDialAddr(DoHTarget{URL: "https://x/y"}, "x:443", "x")
	if !reflect.DeepEqual(got, []string{"x:443"}) {
		t.Errorf("got %v", got)
	}
}

func TestResolveDialAddr_HostMismatchPassesThrough(t *testing.T) {
	target := DoHTarget{URL: "https://dns.google/q", BootstrapIPs: []string{"8.8.8.8"}}
	// addr host 不是 dns.google（如 SNI 不同）→ 不替换
	got := resolveDialAddr(target, "other.host:443", "dns.google")
	if !reflect.DeepEqual(got, []string{"other.host:443"}) {
		t.Errorf("got %v", got)
	}
}

func TestResolveDialAddr_BootstrapHit(t *testing.T) {
	target := DoHTarget{
		URL:          "https://dns.google/q",
		BootstrapIPs: []string{"8.8.8.8", "8.8.4.4"},
	}
	got := resolveDialAddr(target, "dns.google:443", "dns.google")
	want := []string{"8.8.8.8:443", "8.8.4.4:443"}
	if !reflect.DeepEqual(got, want) {
		t.Errorf("got %v, want %v", got, want)
	}
}

func TestResolveDialAddr_MalformedAddrPassesThrough(t *testing.T) {
	target := DoHTarget{BootstrapIPs: []string{"8.8.8.8"}}
	got := resolveDialAddr(target, "bogus-no-port", "dns.google")
	if !reflect.DeepEqual(got, []string{"bogus-no-port"}) {
		t.Errorf("got %v", got)
	}
}

// --- dohResolver.Query (httptest) ---

// dohServerStub spins up an httptest TLS server that handles a single canned response.
type dohServerStub struct {
	t             *testing.T
	response      *dns.Msg
	statusCode    int           // 0 = 200
	rawResponse   []byte        // 非 nil 时无视 response，直接写
	requestDelay  time.Duration // 模拟慢响应
	gotMethod     string
	gotContentTyp string
	gotQuestion   dns.Question
}

func newDoHServer(t *testing.T, stub *dohServerStub) *httptest.Server {
	stub.t = t
	srv := httptest.NewTLSServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		stub.gotMethod = r.Method
		stub.gotContentTyp = r.Header.Get("Content-Type")
		if r.Method != http.MethodPost {
			http.Error(w, "method", http.StatusMethodNotAllowed)
			return
		}
		body, _ := io.ReadAll(r.Body)
		var req dns.Msg
		if err := req.Unpack(body); err == nil && len(req.Question) > 0 {
			stub.gotQuestion = req.Question[0]
		}
		if stub.requestDelay > 0 {
			select {
			case <-time.After(stub.requestDelay):
			case <-r.Context().Done():
				return
			}
		}
		w.Header().Set("Content-Type", "application/dns-message")
		if stub.statusCode != 0 {
			w.WriteHeader(stub.statusCode)
		}
		if stub.rawResponse != nil {
			_, _ = w.Write(stub.rawResponse)
			return
		}
		if stub.response != nil {
			b, _ := stub.response.Pack()
			_, _ = w.Write(b)
		}
	}))
	return srv
}

func TestDoHResolver_HappyPath(t *testing.T) {
	resp := mkMsg(dns.RcodeSuccess, mkA("example.com", 60, "1.2.3.4"))
	stub := &dohServerStub{response: resp}
	srv := newDoHServer(t, stub)
	defer srv.Close()

	r := newDoHResolverWithClient(DoHTarget{URL: srv.URL + "/dns-query"}, srv.Client())
	got, err := r.Query(context.Background(), "example.com", dns.TypeA, "")
	if err != nil {
		t.Fatalf("Query: %v", err)
	}
	if len(got.Answer) != 1 {
		t.Fatalf("expected 1 answer, got %d", len(got.Answer))
	}
	if stub.gotMethod != "POST" {
		t.Errorf("method = %q, want POST", stub.gotMethod)
	}
	if stub.gotContentTyp != "application/dns-message" {
		t.Errorf("Content-Type = %q", stub.gotContentTyp)
	}
	if stub.gotQuestion.Name != "example.com." {
		t.Errorf("question = %q", stub.gotQuestion.Name)
	}
}

func TestDoHResolver_HTTPError(t *testing.T) {
	stub := &dohServerStub{statusCode: http.StatusInternalServerError}
	srv := newDoHServer(t, stub)
	defer srv.Close()

	r := newDoHResolverWithClient(DoHTarget{URL: srv.URL + "/dns-query"}, srv.Client())
	_, err := r.Query(context.Background(), "example.com", dns.TypeA, "")
	if err == nil || err.Error() != "HTTP_500" {
		t.Errorf("expected HTTP_500, got %v", err)
	}
}

func TestDoHResolver_BadResponseBody(t *testing.T) {
	stub := &dohServerStub{rawResponse: []byte("not a dns message")}
	srv := newDoHServer(t, stub)
	defer srv.Close()

	r := newDoHResolverWithClient(DoHTarget{URL: srv.URL + "/dns-query"}, srv.Client())
	_, err := r.Query(context.Background(), "example.com", dns.TypeA, "")
	if err == nil || !strings.Contains(err.Error(), "UNPACK_ERROR") {
		t.Errorf("expected UNPACK_ERROR, got %v", err)
	}
}

func TestDoHResolver_ContextTimeout(t *testing.T) {
	stub := &dohServerStub{
		response:     mkMsg(dns.RcodeSuccess),
		requestDelay: 500 * time.Millisecond,
	}
	srv := newDoHServer(t, stub)
	defer srv.Close()

	// 给 client 配一个很短的 timeout
	client := srv.Client()
	client.Timeout = 50 * time.Millisecond
	r := newDoHResolverWithClient(DoHTarget{URL: srv.URL + "/dns-query"}, client)

	_, err := r.Query(context.Background(), "example.com", dns.TypeA, "")
	if err == nil || err.Error() != "TIMEOUT" {
		t.Errorf("expected TIMEOUT, got %v", err)
	}
}

// --- ProviderAliases ---

func TestProviderAliases_SortedAndComplete(t *testing.T) {
	got := ProviderAliases()
	expected := []string{"360", "ali", "cloudflare", "google", "opendns", "quad9"}
	if !reflect.DeepEqual(got, expected) {
		t.Errorf("got %v, want %v", got, expected)
	}
}

// --- friendlyDoHErr ---

func TestFriendlyDoHErr(t *testing.T) {
	cases := []struct {
		in   string
		want string
	}{
		{"tls: handshake failure", "TLS_ERROR"},
		{"x509: certificate signed by unknown authority", "TLS_ERROR"},
		{"Get https://dns.google: net/http: TLS handshake timeout", "TIMEOUT"},
		{"context deadline exceeded", "TIMEOUT"},
		{"context canceled", "TIMEOUT"},
		{"Client.Timeout exceeded while awaiting headers", "TIMEOUT"},
		{"dial tcp 1.2.3.4:443: connect: connection refused", "CONNECTION_REFUSED"},
		{"lookup x: no such host", "NO_SUCH_HOST"},
	}
	for _, c := range cases {
		t.Run(c.in, func(t *testing.T) {
			got := friendlyDoHErr(fakeErr(c.in)).Error()
			if got != c.want {
				t.Errorf("got %q, want %q", got, c.want)
			}
		})
	}
}

type fakeErr string

func (e fakeErr) Error() string { return string(e) }
