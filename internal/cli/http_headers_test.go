package cli

import (
	"bytes"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
)

func runHTTPHeaders(t *testing.T, client *http.Client, args ...string) (string, error) {
	t.Helper()
	var buf bytes.Buffer
	cmd := newHTTPHeadersCommand(httpHeadersDeps{out: &buf, client: client})
	cmd.SetArgs(args)
	err := cmd.Execute()
	return buf.String(), err
}

func TestHTTPHeadersCmd_Basic(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("X-Demo", "yes")
		w.WriteHeader(200)
	}))
	defer srv.Close()

	out, err := runHTTPHeaders(t, srv.Client(), srv.URL)
	if err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(out, "200 OK") || !strings.Contains(out, "X-Demo: yes") {
		t.Errorf("output wrong:\n%s", out)
	}
}

func TestHTTPHeadersCmd_RedirectChain(t *testing.T) {
	mux := http.NewServeMux()
	mux.HandleFunc("/", func(w http.ResponseWriter, r *http.Request) {
		http.Redirect(w, r, "/done", http.StatusMovedPermanently)
	})
	mux.HandleFunc("/done", func(w http.ResponseWriter, r *http.Request) { w.WriteHeader(200) })
	srv := httptest.NewServer(mux)
	defer srv.Close()

	out, err := runHTTPHeaders(t, srv.Client(), srv.URL+"/")
	if err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(out, "301 Moved Permanently") || !strings.Contains(out, "→ 200 OK") {
		t.Errorf("redirect chain not shown:\n%s", out)
	}
	if !strings.Contains(out, "Location:") {
		t.Errorf("redirect should show Location:\n%s", out)
	}
}

func TestHTTPHeadersCmd_NoFollow(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		http.Redirect(w, r, "/x", http.StatusFound)
	}))
	defer srv.Close()

	out, err := runHTTPHeaders(t, srv.Client(), srv.URL, "--max-redirects", "0")
	if err != nil {
		t.Fatal(err)
	}
	if strings.Contains(out, "→") {
		t.Errorf("--max-redirects 0 should not follow:\n%s", out)
	}
}

func TestHTTPHeadersCmd_JSON(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(200)
	}))
	defer srv.Close()

	out, err := runHTTPHeaders(t, srv.Client(), srv.URL, "--json")
	if err != nil {
		t.Fatal(err)
	}
	var got []map[string]any
	if err := json.Unmarshal([]byte(out), &got); err != nil {
		t.Fatalf("invalid JSON: %v\n%s", err, out)
	}
	if len(got) != 1 || got[0]["status_code"].(float64) != 200 {
		t.Errorf("bad JSON: %v", got)
	}
}

func TestHTTPHeadersCmd_RequestHeader(t *testing.T) {
	var gotUA string
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		gotUA = r.Header.Get("User-Agent")
		w.WriteHeader(200)
	}))
	defer srv.Close()

	if _, err := runHTTPHeaders(t, srv.Client(), srv.URL, "-H", "User-Agent: jdan-test"); err != nil {
		t.Fatal(err)
	}
	if gotUA != "jdan-test" {
		t.Errorf("custom request header not sent: %q", gotUA)
	}
}

func TestHTTPHeadersCmd_BadHeader(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {}))
	defer srv.Close()
	if _, err := runHTTPHeaders(t, srv.Client(), srv.URL, "-H", "no-colon-here"); err == nil {
		t.Error("malformed -H should error")
	}
}

func TestHTTPHeadersCmd_ConnError(t *testing.T) {
	out, err := runHTTPHeaders(t, http.DefaultClient, "http://127.0.0.1:1")
	if err == nil {
		t.Error("connection error should surface")
	}
	if strings.Contains(out, "200") {
		t.Errorf("no output expected on conn error:\n%s", out)
	}
}
