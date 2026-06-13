package cli

import (
	"bytes"
	"encoding/json"
	"net"
	"net/http"
	"net/http/httptest"
	"net/url"
	"strconv"
	"strings"
	"testing"
)

func TestParsePortArg(t *testing.T) {
	for _, tc := range []struct {
		in     string
		want   int
		errExp bool
	}{
		{"8080", 8080, false},
		{":8080", 8080, false},
		{":443", 443, false},
		{"abc", 0, true},
		{"0", 0, true},
		{"65536", 0, true},
		{"-1", 0, true},
	} {
		got, err := parsePortArg(tc.in)
		if tc.errExp {
			if err == nil {
				t.Errorf("%q should error", tc.in)
			}
			continue
		}
		if err != nil {
			t.Errorf("%q: unexpected error %v", tc.in, err)
			continue
		}
		if got != tc.want {
			t.Errorf("%q: got %d, want %d", tc.in, got, tc.want)
		}
	}
}

func TestNetSelfcheck_NoArgs_GeneralReport(t *testing.T) {
	var buf bytes.Buffer
	cmd := newNetSelfcheckCommand(netSelfcheckCmdDeps{out: &buf})
	cmd.SetArgs([]string{})
	if err := cmd.Execute(); err != nil {
		t.Fatal(err)
	}
	out := buf.String()
	for _, want := range []string{"os & firewall", "network interfaces", "prediction"} {
		if !strings.Contains(out, want) {
			t.Errorf("output missing %q:\n%s", want, out)
		}
	}
}

func TestNetSelfcheck_WithPort_RealServer(t *testing.T) {
	ts := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(200)
	}))
	defer ts.Close()
	u, _ := url.Parse(ts.URL)
	_, portStr, _ := net.SplitHostPort(u.Host)
	port, _ := strconv.Atoi(portStr)

	var buf bytes.Buffer
	cmd := newNetSelfcheckCommand(netSelfcheckCmdDeps{out: &buf})
	cmd.SetArgs([]string{strconv.Itoa(port)})
	if err := cmd.Execute(); err != nil {
		t.Fatal(err)
	}
	out := buf.String()
	for _, want := range []string{"listening on", "self-loop test", "prediction"} {
		if !strings.Contains(out, want) {
			t.Errorf("output missing %q:\n%s", want, out)
		}
	}
}

func TestNetSelfcheck_JSONOutput(t *testing.T) {
	var buf bytes.Buffer
	cmd := newNetSelfcheckCommand(netSelfcheckCmdDeps{out: &buf})
	cmd.SetArgs([]string{"--json"})
	if err := cmd.Execute(); err != nil {
		t.Fatal(err)
	}
	var report map[string]interface{}
	if err := json.Unmarshal(buf.Bytes(), &report); err != nil {
		t.Fatalf("invalid JSON: %v\n%s", err, buf.String())
	}
	if report["os"] == nil {
		t.Error("JSON should contain os field")
	}
	if report["interfaces"] == nil {
		t.Error("JSON should contain interfaces field")
	}
	if report["prediction"] == nil {
		t.Error("JSON should contain prediction")
	}
}

func TestNetSelfcheck_InvalidPort_Errors(t *testing.T) {
	cmd := newNetSelfcheckCommand(netSelfcheckCmdDeps{out: &bytes.Buffer{}})
	cmd.SetArgs([]string{"abc"})
	if err := cmd.Execute(); err == nil {
		t.Error("invalid port should error")
	}
}

func TestNetSelfcheck_ColonPrefix_Works(t *testing.T) {
	// ":8080" 形式应当被接受
	var buf bytes.Buffer
	cmd := newNetSelfcheckCommand(netSelfcheckCmdDeps{out: &buf})
	cmd.SetArgs([]string{":1"})
	if err := cmd.Execute(); err != nil {
		t.Fatal(err)
	}
	// :1 上不会有 listener，但命令本身要成功
	if !strings.Contains(buf.String(), "listening on :1") {
		t.Errorf("expected listening section for :1, got: %s", buf.String())
	}
}
