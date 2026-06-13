package cli

import (
	"bytes"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"net/url"
	"strings"
	"testing"
)

func TestNetProbe_TextOutput_Success(t *testing.T) {
	ts := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Server", "test")
		w.WriteHeader(200)
	}))
	defer ts.Close()
	u, _ := url.Parse(ts.URL)

	var buf bytes.Buffer
	cmd := newNetProbeCommand(netProbeCmdDeps{out: &buf})
	cmd.SetArgs([]string{"http://" + u.Host})
	if err := cmd.Execute(); err != nil {
		t.Fatal(err)
	}
	out := buf.String()
	for _, want := range []string{"resolve", "tcp", "http", "all green"} {
		if !strings.Contains(out, want) {
			t.Errorf("output missing %q:\n%s", want, out)
		}
	}
	// 不是 https，TLS 阶段不应出现
	if strings.Contains(out, "tls") {
		t.Error("http:// should not emit TLS stage")
	}
}

func TestNetProbe_ConnectionRefusedShowsHint(t *testing.T) {
	var buf bytes.Buffer
	cmd := newNetProbeCommand(netProbeCmdDeps{out: &buf})
	cmd.SetArgs([]string{"127.0.0.1:1", "--timeout", "500ms"})
	if err := cmd.Execute(); err != nil {
		t.Fatal(err)
	}
	out := buf.String()
	if !strings.Contains(out, "failed at tcp") {
		t.Errorf("should fail at tcp: %s", out)
	}
	if !strings.Contains(out, "connection refused") {
		t.Errorf("should show conn refused: %s", out)
	}
	// hint 必须 cross-ref selfcheck
	if !strings.Contains(out, "selfcheck") {
		t.Errorf("hint should cross-ref selfcheck: %s", out)
	}
}

func TestNetProbe_JSONOutput(t *testing.T) {
	ts := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(200)
	}))
	defer ts.Close()
	u, _ := url.Parse(ts.URL)

	var buf bytes.Buffer
	cmd := newNetProbeCommand(netProbeCmdDeps{out: &buf})
	cmd.SetArgs([]string{"http://" + u.Host, "--json"})
	if err := cmd.Execute(); err != nil {
		t.Fatal(err)
	}
	var result map[string]interface{}
	if err := json.Unmarshal(buf.Bytes(), &result); err != nil {
		t.Fatalf("invalid JSON: %v\n%s", err, buf.String())
	}
	if result["ok"] != true {
		t.Errorf("ok should be true, got %v", result["ok"])
	}
}

func TestNetProbe_InsecureFlag_AcceptsSelfSigned(t *testing.T) {
	ts := httptest.NewTLSServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(200)
	}))
	defer ts.Close()
	u, _ := url.Parse(ts.URL)

	// 不加 --insecure 应当 TLS 失败
	var buf1 bytes.Buffer
	cmd1 := newNetProbeCommand(netProbeCmdDeps{out: &buf1})
	cmd1.SetArgs([]string{"https://" + u.Host})
	cmd1.Execute()
	if !strings.Contains(buf1.String(), "failed at tls") {
		t.Errorf("self-signed without --insecure should fail at tls: %s", buf1.String())
	}

	// 加 --insecure 应通过
	var buf2 bytes.Buffer
	cmd2 := newNetProbeCommand(netProbeCmdDeps{out: &buf2})
	cmd2.SetArgs([]string{"https://" + u.Host, "--insecure"})
	cmd2.Execute()
	if !strings.Contains(buf2.String(), "all green") {
		t.Errorf("--insecure should pass self-signed cert: %s", buf2.String())
	}
}
