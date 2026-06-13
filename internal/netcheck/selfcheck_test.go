package netcheck

import (
	"context"
	"net"
	"net/http"
	"net/http/httptest"
	"net/url"
	"runtime"
	"strconv"
	"strings"
	"testing"
	"time"
)

func TestSelfCheck_NoPortGivesGeneralReport(t *testing.T) {
	r, err := SelfCheck(context.Background(), Options{})
	if err != nil {
		t.Fatal(err)
	}
	if r.OS != runtime.GOOS {
		t.Errorf("OS = %q, want %q", r.OS, runtime.GOOS)
	}
	if r.Listening != nil {
		t.Error("no port specified, Listening should be nil")
	}
	if r.SelfLoop != nil {
		t.Error("no port specified, SelfLoop should be nil")
	}
	if r.Prediction == "" {
		t.Error("prediction should always be set")
	}
	if len(r.Interfaces) == 0 {
		t.Error("expected at least one interface (loopback)")
	}
}

func TestSelfCheck_RealServerListening(t *testing.T) {
	ts := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusOK)
	}))
	defer ts.Close()

	u, _ := url.Parse(ts.URL)
	_, portStr, _ := net.SplitHostPort(u.Host)
	port, _ := strconv.Atoi(portStr)

	r, err := SelfCheck(context.Background(), Options{Port: port})
	if err != nil {
		t.Fatal(err)
	}

	if r.Listening == nil {
		t.Fatal("Listening should be populated for explicit port")
	}
	if r.Listening.LsofMissing {
		t.Skip("lsof not installed")
	}
	if len(r.Listening.Listeners) == 0 {
		t.Errorf("expected to find listener on port %d", port)
	}

	if r.SelfLoop == nil {
		t.Fatal("SelfLoop should be populated for explicit port")
	}
	if !r.SelfLoop.Localhost.Success {
		t.Errorf("localhost self-loop should succeed: %s", r.SelfLoop.Localhost.Err)
	}
}

func TestSelfCheck_NothingListeningGivesHint(t *testing.T) {
	// 用一个 ephemeral port，但不绑定（让 lsof 找不到）
	l, _ := net.Listen("tcp", "127.0.0.1:0")
	port := l.Addr().(*net.TCPAddr).Port
	l.Close() // 立刻关，留下一个空 port

	// 等一下让 OS 释放
	time.Sleep(50 * time.Millisecond)

	r, err := SelfCheck(context.Background(), Options{Port: port, Timeout: 300 * time.Millisecond})
	if err != nil {
		t.Fatal(err)
	}
	if r.Listening == nil {
		t.Fatal("expected listening section")
	}
	if r.Listening.LsofMissing {
		t.Skip("lsof not installed")
	}
	if len(r.Listening.Listeners) != 0 {
		t.Skipf("port %d was reused by another process, can't test 'nothing listening' branch", port)
	}
	if !strings.Contains(r.Prediction, "nothing is listening") {
		t.Errorf("prediction should mention 'nothing is listening', got: %s", r.Prediction)
	}
}

func TestBuildPrediction_NoListener(t *testing.T) {
	r := &Report{
		Firewall:  FirewallSection{State: "disabled"},
		Listening: &ListeningSection{Port: 8080},
	}
	p := buildPrediction(r)
	if !strings.Contains(p, "nothing is listening") {
		t.Errorf("got: %s", p)
	}
}

func TestBuildPrediction_LsofMissing(t *testing.T) {
	r := &Report{
		Listening: &ListeningSection{Port: 8080, LsofMissing: true},
	}
	p := buildPrediction(r)
	if !strings.Contains(p, "install lsof") {
		t.Errorf("got: %s", p)
	}
}

func TestBuildPrediction_LoopbackOnly(t *testing.T) {
	r := &Report{
		Firewall: FirewallSection{State: "disabled"},
		Listening: &ListeningSection{
			Port: 8080,
			Listeners: []Listener{
				{Bind: "127.0.0.1:8080"},
			},
		},
	}
	p := buildPrediction(r)
	if !strings.Contains(p, "loopback only") {
		t.Errorf("got: %s", p)
	}
	if !strings.Contains(p, "CANNOT reach") {
		t.Errorf("should warn external CANNOT reach: %s", p)
	}
}

func TestBuildPrediction_FirewallBlocking(t *testing.T) {
	r := &Report{
		Firewall: FirewallSection{State: "enabled"},
		Listening: &ListeningSection{
			Port: 8080,
			Listeners: []Listener{
				{Bind: "*:8080"},
			},
		},
	}
	p := buildPrediction(r)
	if !strings.Contains(p, "firewall is on") {
		t.Errorf("should mention firewall: %s", p)
	}
}

func TestBuildPrediction_AllGoodLANReachable(t *testing.T) {
	r := &Report{
		Firewall: FirewallSection{State: "disabled"},
		Listening: &ListeningSection{
			Port: 8080,
			Listeners: []Listener{
				{Bind: "*:8080"},
			},
		},
		SelfLoop: &SelfLoopSection{
			PrimaryLAN: ConnectResult{Addr: "http://192.168.1.42:8080", Success: true},
		},
	}
	p := buildPrediction(r)
	if !strings.Contains(p, "LAN-reachable from self") {
		t.Errorf("should confirm LAN-reachable: %s", p)
	}
}

func TestIsPrivateV4(t *testing.T) {
	for _, tc := range []struct {
		ip   string
		want bool
	}{
		{"10.0.0.1", true},
		{"172.16.0.1", true},
		{"172.31.0.1", true},
		{"172.32.0.1", false},
		{"192.168.1.1", true},
		{"8.8.8.8", false},
		{"127.0.0.1", false},
	} {
		ip := net.ParseIP(tc.ip).To4()
		if got := isPrivateV4(ip); got != tc.want {
			t.Errorf("isPrivateV4(%s) = %v, want %v", tc.ip, got, tc.want)
		}
	}
}
