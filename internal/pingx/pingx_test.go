package pingx

import (
	"context"
	"strings"
	"testing"

	"github.com/miekg/dns"
)

// ---- BuildCommand ----

func TestBuildCommand_Basic(t *testing.T) {
	bin, args := BuildCommand("1.2.3.4", Options{}, "linux")
	if bin != "ping" {
		t.Errorf("bin = %q, want ping", bin)
	}
	if len(args) != 1 || args[0] != "1.2.3.4" {
		t.Errorf("args = %v, want [1.2.3.4]", args)
	}
}

func TestBuildCommand_Count(t *testing.T) {
	_, args := BuildCommand("host", Options{Count: 3}, "linux")
	want := []string{"-c", "3", "host"}
	if strings.Join(args, " ") != strings.Join(want, " ") {
		t.Errorf("args = %v, want %v", args, want)
	}
}

func TestBuildCommand_V6Linux(t *testing.T) {
	bin, args := BuildCommand("::1", Options{V6: true}, "linux")
	if bin != "ping" {
		t.Errorf("linux v6 bin = %q, want ping", bin)
	}
	if args[0] != "-6" || args[len(args)-1] != "::1" {
		t.Errorf("linux v6 args = %v, want -6 ... ::1", args)
	}
}

func TestBuildCommand_V6Darwin(t *testing.T) {
	bin, args := BuildCommand("::1", Options{V6: true}, "darwin")
	if bin != "ping6" {
		t.Errorf("darwin v6 bin = %q, want ping6", bin)
	}
	for _, a := range args {
		if a == "-6" {
			t.Error("darwin ping6 should not take -6")
		}
	}
	if args[len(args)-1] != "::1" {
		t.Errorf("target should be last: %v", args)
	}
}

func TestBuildCommand_ExtraPassthrough(t *testing.T) {
	_, args := BuildCommand("host", Options{Count: 2, Extra: []string{"-i", "0.2", "-s", "64"}}, "linux")
	want := "-c 2 -i 0.2 -s 64 host"
	if strings.Join(args, " ") != want {
		t.Errorf("args = %v, want %q", args, want)
	}
}

func TestBuildCommand_TargetAlwaysLast(t *testing.T) {
	_, args := BuildCommand("example.com", Options{Count: 5, V6: true, Extra: []string{"-x"}}, "linux")
	if args[len(args)-1] != "example.com" {
		t.Errorf("target must be last arg: %v", args)
	}
}

// ---- Resolve ----

type fakeResolver struct {
	msg       *dns.Msg
	err       error
	gotType   uint16
	gotServer string
	gotDomain string
}

func (f *fakeResolver) Query(_ context.Context, domain string, qtype uint16, server string) (*dns.Msg, error) {
	f.gotDomain = domain
	f.gotType = qtype
	f.gotServer = server
	return f.msg, f.err
}

func msgWith(t *testing.T, rrs ...string) *dns.Msg {
	t.Helper()
	m := new(dns.Msg)
	for _, s := range rrs {
		rr, err := dns.NewRR(s)
		if err != nil {
			t.Fatal(err)
		}
		m.Answer = append(m.Answer, rr)
	}
	return m
}

func TestResolve_A(t *testing.T) {
	fr := &fakeResolver{msg: msgWith(t, "example.com. 300 IN A 93.184.216.34")}
	ip, err := Resolve(context.Background(), fr, "example.com", "8.8.8.8", false)
	if err != nil {
		t.Fatal(err)
	}
	if ip != "93.184.216.34" {
		t.Errorf("ip = %q, want 93.184.216.34", ip)
	}
	if fr.gotType != dns.TypeA || fr.gotServer != "8.8.8.8" || fr.gotDomain != "example.com" {
		t.Errorf("query args wrong: type=%d server=%q domain=%q", fr.gotType, fr.gotServer, fr.gotDomain)
	}
}

func TestResolve_AAAA(t *testing.T) {
	fr := &fakeResolver{msg: msgWith(t, "example.com. 300 IN AAAA 2606:2800:220:1:248:1893:25c8:1946")}
	ip, err := Resolve(context.Background(), fr, "example.com", "8.8.8.8", true)
	if err != nil {
		t.Fatal(err)
	}
	if ip != "2606:2800:220:1:248:1893:25c8:1946" {
		t.Errorf("ip = %q", ip)
	}
	if fr.gotType != dns.TypeAAAA {
		t.Errorf("v6 should query AAAA, got %d", fr.gotType)
	}
}

func TestResolve_SkipsWrongType(t *testing.T) {
	// 要 A，但只有 CNAME + AAAA → 应当报「没有 A 记录」
	fr := &fakeResolver{msg: msgWith(t,
		"www.example.com. 300 IN CNAME example.com.",
		"example.com. 300 IN AAAA 2606:2800:220:1:248:1893:25c8:1946",
	)}
	if _, err := Resolve(context.Background(), fr, "www.example.com", "8.8.8.8", false); err == nil {
		t.Error("should error when no A record present")
	}
}

func TestResolve_NoRecord(t *testing.T) {
	fr := &fakeResolver{msg: new(dns.Msg)}
	if _, err := Resolve(context.Background(), fr, "nope.invalid", "8.8.8.8", false); err == nil {
		t.Error("empty answer should error")
	}
}

func TestResolve_QueryError(t *testing.T) {
	fr := &fakeResolver{err: context.DeadlineExceeded}
	if _, err := Resolve(context.Background(), fr, "x", "8.8.8.8", false); err == nil {
		t.Error("query error should propagate")
	}
}

// ---- ParseSummary ----

func TestParseSummary_Linux(t *testing.T) {
	out := `3 packets transmitted, 3 received, 0% packet loss, time 2003ms
rtt min/avg/max/mdev = 11.8/12.0/12.1/0.1 ms`
	s := ParseSummary(out)
	if !s.HasStats || s.Transmitted != 3 || s.Received != 3 || s.LossPct != 0 {
		t.Errorf("linux stats wrong: %+v", s)
	}
	if !s.HasRTT || s.RTTMin != 11.8 || s.RTTAvg != 12.0 || s.RTTMax != 12.1 {
		t.Errorf("linux rtt wrong: %+v", s)
	}
}

func TestParseSummary_MacOS(t *testing.T) {
	out := `3 packets transmitted, 3 packets received, 0.0% packet loss
round-trip min/avg/max/stddev = 11.8/12.0/12.1/0.1 ms`
	s := ParseSummary(out)
	if !s.HasStats || s.Transmitted != 3 || s.Received != 3 {
		t.Errorf("macos stats wrong: %+v", s)
	}
	if !s.HasRTT || s.RTTAvg != 12.0 {
		t.Errorf("macos rtt wrong: %+v", s)
	}
}

func TestParseSummary_Loss(t *testing.T) {
	out := `5 packets transmitted, 2 received, 60% packet loss, time 4005ms`
	s := ParseSummary(out)
	if s.Transmitted != 5 || s.Received != 2 || s.LossPct != 60 {
		t.Errorf("loss parse wrong: %+v", s)
	}
	if s.HasRTT {
		t.Error("no rtt line → HasRTT should be false")
	}
}

func TestParseSummary_Garbage(t *testing.T) {
	s := ParseSummary("this is not ping output at all")
	if s.HasStats || s.HasRTT {
		t.Errorf("garbage should parse nothing: %+v", s)
	}
}
