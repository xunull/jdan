package cli

import (
	"bytes"
	"context"
	"encoding/json"
	"io"
	"strings"
	"testing"

	"github.com/miekg/dns"
)

// fakePingRunner 记录收到的 argv，并把预设 stdout 写出去。
type fakePingRunner struct {
	bin    string
	args   []string
	stdout string
	code   int
	err    error
	calls  int
}

func (f *fakePingRunner) run(_ context.Context, bin string, args []string, stdout, stderr io.Writer) (int, error) {
	f.calls++
	f.bin = bin
	f.args = args
	if f.stdout != "" {
		io.WriteString(stdout, f.stdout)
	}
	return f.code, f.err
}

type fakePingResolver struct {
	ip string
}

func (f *fakePingResolver) Query(_ context.Context, domain string, qtype uint16, server string) (*dns.Msg, error) {
	m := new(dns.Msg)
	rrtype := "A"
	if qtype == dns.TypeAAAA {
		rrtype = "AAAA"
	}
	rr, err := dns.NewRR(domain + ". 300 IN " + rrtype + " " + f.ip)
	if err != nil {
		return nil, err
	}
	m.Answer = append(m.Answer, rr)
	return m, nil
}

func runPingTest(t *testing.T, runner *fakePingRunner, resolver *fakePingResolver, goos string, args ...string) (string, error) {
	t.Helper()
	var out bytes.Buffer
	deps := pingCmdDeps{out: &out, errOut: &out, runner: runner.run, goos: goos}
	if resolver != nil {
		deps.resolver = resolver
	}
	cmd := newPingCommand(deps)
	cmd.SetArgs(args)
	err := cmd.Execute()
	return out.String(), err
}

func TestPingCmd_DefaultNoDNS(t *testing.T) {
	fr := &fakePingRunner{stdout: "PING ...\n"}
	out, err := runPingTest(t, fr, nil, "linux", "example.com")
	if err != nil {
		t.Fatal(err)
	}
	// 默认：直接把 host 交给 ping，不解析、无解析头
	if fr.bin != "ping" || fr.args[len(fr.args)-1] != "example.com" {
		t.Errorf("argv wrong: %s %v", fr.bin, fr.args)
	}
	if strings.Contains(out, "→") {
		t.Errorf("no --dns → no resolution header:\n%s", out)
	}
}

func TestPingCmd_WithDNSResolves(t *testing.T) {
	fr := &fakePingRunner{stdout: "PING ...\n"}
	res := &fakePingResolver{ip: "93.184.216.34"}
	out, err := runPingTest(t, fr, res, "linux", "--dns", "8.8.8.8", "example.com")
	if err != nil {
		t.Fatal(err)
	}
	// ping 的是解析出的 IP，不是域名
	if fr.args[len(fr.args)-1] != "93.184.216.34" {
		t.Errorf("should ping resolved IP, got argv %v", fr.args)
	}
	if !strings.Contains(out, "example.com → 93.184.216.34 (via 8.8.8.8)") {
		t.Errorf("resolution header missing:\n%s", out)
	}
}

func TestPingCmd_Count(t *testing.T) {
	fr := &fakePingRunner{stdout: "x"}
	_, err := runPingTest(t, fr, nil, "linux", "-c", "3", "example.com")
	if err != nil {
		t.Fatal(err)
	}
	if strings.Join(fr.args, " ") != "-c 3 example.com" {
		t.Errorf("args = %v, want -c 3 example.com", fr.args)
	}
}

func TestPingCmd_Passthrough(t *testing.T) {
	fr := &fakePingRunner{stdout: "x"}
	_, err := runPingTest(t, fr, nil, "linux", "example.com", "--", "-i", "0.2", "-s", "64")
	if err != nil {
		t.Fatal(err)
	}
	got := strings.Join(fr.args, " ")
	if got != "-i 0.2 -s 64 example.com" {
		t.Errorf("passthrough wrong: %q", got)
	}
}

func TestPingCmd_HostIsIP_NoResolve(t *testing.T) {
	fr := &fakePingRunner{stdout: "x"}
	res := &fakePingResolver{ip: "1.1.1.1"}
	out, err := runPingTest(t, fr, res, "linux", "--dns", "8.8.8.8", "9.9.9.9")
	if err != nil {
		t.Fatal(err)
	}
	// host 本身是 IP → 不解析，直接 ping，无解析头
	if fr.args[len(fr.args)-1] != "9.9.9.9" {
		t.Errorf("should ping the literal IP: %v", fr.args)
	}
	if strings.Contains(out, "→") {
		t.Errorf("IP host should not resolve:\n%s", out)
	}
}

func TestPingCmd_JSON(t *testing.T) {
	summary := `4 packets transmitted, 4 received, 0% packet loss
rtt min/avg/max/mdev = 1.1/2.2/3.3/0.4 ms`
	fr := &fakePingRunner{stdout: summary}
	res := &fakePingResolver{ip: "93.184.216.34"}
	out, err := runPingTest(t, fr, res, "linux", "--dns", "8.8.8.8", "example.com", "--json")
	if err != nil {
		t.Fatal(err)
	}
	var rec pingJSON
	if err := json.Unmarshal([]byte(out), &rec); err != nil {
		t.Fatalf("invalid JSON: %v\n%s", err, out)
	}
	if rec.Host != "example.com" || rec.ResolvedIP != "93.184.216.34" || rec.DNSServer != "8.8.8.8" {
		t.Errorf("json facts wrong: %+v", rec)
	}
	if rec.RTTAvg == nil || *rec.RTTAvg != 2.2 {
		t.Errorf("rtt_avg wrong: %+v", rec.RTTAvg)
	}
	if rec.Received == nil || *rec.Received != 4 {
		t.Errorf("received wrong")
	}
	// JSON 模式无 -c 时应自动补 count（否则 ping 不退出），argv 应含 -c
	if !strings.Contains(strings.Join(fr.args, " "), "-c") {
		t.Errorf("json mode should default a count: %v", fr.args)
	}
}

func TestPingCmd_JSON_NoResolveFacts(t *testing.T) {
	// 无 --dns 时 JSON 不应带 dns_server / resolved_ip
	fr := &fakePingRunner{stdout: "0 packets transmitted, 0 received"}
	out, _ := runPingTest(t, fr, nil, "linux", "example.com", "--json")
	if strings.Contains(out, "dns_server") || strings.Contains(out, "resolved_ip") {
		t.Errorf("no --dns → no dns fields:\n%s", out)
	}
}

func TestPingCmd_V6Darwin(t *testing.T) {
	fr := &fakePingRunner{stdout: "x"}
	res := &fakePingResolver{ip: "2606:2800:220:1:248:1893:25c8:1946"}
	_, err := runPingTest(t, fr, res, "darwin", "--dns", "8.8.8.8", "-6", "example.com")
	if err != nil {
		t.Fatal(err)
	}
	if fr.bin != "ping6" {
		t.Errorf("darwin v6 should use ping6, got %q", fr.bin)
	}
}

func TestPingCmd_V4V6Conflict(t *testing.T) {
	fr := &fakePingRunner{}
	if _, err := runPingTest(t, fr, nil, "linux", "-4", "-6", "example.com"); err == nil {
		t.Error("-4 and -6 together should error")
	}
	if fr.calls != 0 {
		t.Error("should not run ping on arg error")
	}
}

func TestPingCmd_MultipleHostsError(t *testing.T) {
	fr := &fakePingRunner{}
	if _, err := runPingTest(t, fr, nil, "linux", "a.com", "b.com"); err == nil {
		t.Error("two hosts (before --) should error")
	}
}

func TestPingCmd_NonzeroExit(t *testing.T) {
	fr := &fakePingRunner{stdout: "x", code: 2}
	if _, err := runPingTest(t, fr, nil, "linux", "example.com"); err == nil {
		t.Error("nonzero ping exit should surface as error")
	}
}
