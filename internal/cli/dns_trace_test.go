package cli

import (
	"bytes"
	"context"
	"errors"
	"strings"
	"testing"
	"time"

	"github.com/miekg/dns"
	"github.com/spf13/cobra"

	"github.com/xunull/jdan/internal/dnstrace"
)

// capturedTrace 是 trace 测试的 mock：捕获每次 deps.trace 被调时的参数。
type capturedTrace struct {
	calls    []traceCall
	stubResp *dnstrace.Result
	stubErr  error
}

type traceCall struct {
	Domain string
	Qtype  uint16
}

func (c *capturedTrace) fn(ctx context.Context, domain string, qtype uint16) (*dnstrace.Result, error) {
	c.calls = append(c.calls, traceCall{Domain: domain, Qtype: qtype})
	if c.stubErr != nil {
		return nil, c.stubErr
	}
	if c.stubResp != nil {
		return c.stubResp, nil
	}
	// 默认 stub：3 跳成功
	hop := dnstrace.Hop{
		Zone: "example.com.", Type: dnstrace.HopAnswer,
		Answers: []string{"1.2.3.4"},
	}
	return &dnstrace.Result{
		Domain:    dns.Fqdn(domain),
		QueryType: dns.TypeToString[qtype],
		Hops:      []dnstrace.Hop{hop},
		Final:     &hop,
	}, nil
}

func newTraceTestCmd(tr *capturedTrace, ex *exitTracker, out *bytes.Buffer) *cobra.Command {
	return newDNSCommand(dnsCmdDeps{
		out:          out,
		trace:        tr.fn,
		detectServer: func() string { return "9.9.9.9:53" },
		exit:         ex.fn,
	})
}

// ------ 参数解析与默认行为 ------

func TestDNSTrace_DefaultTypeA(t *testing.T) {
	tr := &capturedTrace{}
	ex := &exitTracker{}
	var buf bytes.Buffer
	cmd := newTraceTestCmd(tr, ex, &buf)
	cmd.SetArgs([]string{"trace", "example.com"})
	if err := cmd.Execute(); err != nil {
		t.Fatalf("Execute: %v", err)
	}
	if len(tr.calls) != 1 {
		t.Fatalf("expected 1 trace call, got %d", len(tr.calls))
	}
	if tr.calls[0].Qtype != dns.TypeA {
		t.Errorf("default qtype should be A, got %d", tr.calls[0].Qtype)
	}
	if tr.calls[0].Domain != "example.com" {
		t.Errorf("Domain = %q", tr.calls[0].Domain)
	}
}

func TestDNSTrace_TypeOverride(t *testing.T) {
	cases := []struct {
		flag string
		want uint16
	}{
		{"NS", dns.TypeNS},
		{"MX", dns.TypeMX},
		{"TXT", dns.TypeTXT},
		{"AAAA", dns.TypeAAAA},
		{"ns", dns.TypeNS}, // case insensitive
	}
	for _, c := range cases {
		t.Run(c.flag, func(t *testing.T) {
			tr := &capturedTrace{}
			ex := &exitTracker{}
			var buf bytes.Buffer
			cmd := newTraceTestCmd(tr, ex, &buf)
			cmd.SetArgs([]string{"trace", "example.com", "-t", c.flag})
			if err := cmd.Execute(); err != nil {
				t.Fatalf("Execute: %v", err)
			}
			if tr.calls[0].Qtype != c.want {
				t.Errorf("Qtype = %d, want %d", tr.calls[0].Qtype, c.want)
			}
		})
	}
}

func TestDNSTrace_InvalidType(t *testing.T) {
	tr := &capturedTrace{}
	ex := &exitTracker{}
	var buf bytes.Buffer
	cmd := newTraceTestCmd(tr, ex, &buf)
	cmd.SilenceUsage = true
	cmd.SetArgs([]string{"trace", "example.com", "-t", "INVALID"})
	if err := cmd.Execute(); err == nil {
		t.Error("expected error for invalid type")
	}
	if len(tr.calls) != 0 {
		t.Error("trace must not be called when type is invalid")
	}
}

// ------ --doh & --server interaction ------

func TestDNSTrace_DoHAndServerCoexist(t *testing.T) {
	// trace 中 --doh 与 --server 角色不同（DoH 用于 glueless bootstrap，
	// server 用于起步 NS），可共存。
	tr := &capturedTrace{}
	ex := &exitTracker{}
	var buf bytes.Buffer
	cmd := newTraceTestCmd(tr, ex, &buf)
	cmd.SetArgs([]string{"trace", "example.com", "--doh", "google", "-s", "1.1.1.1"})
	if err := cmd.Execute(); err != nil {
		t.Errorf("--doh + --server should coexist in trace, got error: %v", err)
	}
}

func TestDNSTrace_InvalidDoH(t *testing.T) {
	tr := &capturedTrace{}
	ex := &exitTracker{}
	var buf bytes.Buffer
	cmd := newTraceTestCmd(tr, ex, &buf)
	cmd.SilenceUsage = true
	// http:// scheme 被 ResolveDoHTarget 拒绝
	cmd.SetArgs([]string{"trace", "example.com", "--doh", "http://insecure.example/dns-query"})
	if err := cmd.Execute(); err == nil {
		t.Error("expected error for invalid DoH scheme")
	}
}

// ------ 输出格式 flag ------

func TestDNSTrace_JSONFlag(t *testing.T) {
	tr := &capturedTrace{}
	ex := &exitTracker{}
	var buf bytes.Buffer
	cmd := newTraceTestCmd(tr, ex, &buf)
	cmd.SetArgs([]string{"trace", "example.com", "--json"})
	if err := cmd.Execute(); err != nil {
		t.Fatalf("Execute: %v", err)
	}
	out := buf.String()
	if !strings.Contains(out, `"domain"`) || !strings.Contains(out, `"hops"`) {
		t.Errorf("expected JSON output with domain & hops, got:\n%s", out)
	}
}

func TestDNSTrace_ShortFlag(t *testing.T) {
	tr := &capturedTrace{}
	ex := &exitTracker{}
	var buf bytes.Buffer
	cmd := newTraceTestCmd(tr, ex, &buf)
	cmd.SetArgs([]string{"trace", "example.com", "--short"})
	if err := cmd.Execute(); err != nil {
		t.Fatalf("Execute: %v", err)
	}
	if strings.TrimSpace(buf.String()) != "1.2.3.4" {
		t.Errorf("--short should output just final answer, got: %q", buf.String())
	}
}

func TestDNSTrace_ConflictingOutputFlags(t *testing.T) {
	cases := [][]string{
		{"--json", "--short"},
		{"--json", "--verbose"},
		{"--short", "--verbose"},
	}
	for _, c := range cases {
		t.Run(strings.Join(c, " "), func(t *testing.T) {
			tr := &capturedTrace{}
			ex := &exitTracker{}
			var buf bytes.Buffer
			cmd := newTraceTestCmd(tr, ex, &buf)
			cmd.SilenceUsage = true
			args := append([]string{"trace", "example.com"}, c...)
			cmd.SetArgs(args)
			if err := cmd.Execute(); err == nil {
				t.Error("expected conflict error")
			}
		})
	}
}

// ------ Exit code (strict / default) ------

func TestDNSTrace_DefaultExitZero_OnSuccess(t *testing.T) {
	tr := &capturedTrace{} // 默认 stub 返回 success
	ex := &exitTracker{}
	var buf bytes.Buffer
	cmd := newTraceTestCmd(tr, ex, &buf)
	cmd.SetArgs([]string{"trace", "example.com"})
	_ = cmd.Execute()
	if len(ex.codes) != 0 {
		t.Errorf("success path should not exit, got %v", ex.codes)
	}
}

func TestDNSTrace_DefaultExitOne_OnNoFinal(t *testing.T) {
	tr := &capturedTrace{stubResp: &dnstrace.Result{
		Hops: []dnstrace.Hop{
			{Type: dnstrace.HopError, Error: "TIMEOUT"},
		},
	}}
	ex := &exitTracker{}
	var buf bytes.Buffer
	cmd := newTraceTestCmd(tr, ex, &buf)
	cmd.SetArgs([]string{"trace", "example.com"})
	_ = cmd.Execute()
	if len(ex.codes) != 1 || ex.codes[0] != 1 {
		t.Errorf("no final answer should exit(1), got %v", ex.codes)
	}
}

func TestDNSTrace_StrictExitsOne_OnAnyHopError(t *testing.T) {
	// 即使最终拿到 final answer，--strict 模式下任一 ERROR hop 即 exit 1
	finalHop := dnstrace.Hop{Type: dnstrace.HopAnswer, Answers: []string{"1.2.3.4"}}
	tr := &capturedTrace{stubResp: &dnstrace.Result{
		Hops: []dnstrace.Hop{
			{Type: dnstrace.HopError, Error: "TIMEOUT", ServerIP: "198.41.0.4"}, // 中途 fallback
			{Type: dnstrace.HopReferral},
			finalHop,
		},
		Final: &finalHop,
	}}
	ex := &exitTracker{}
	var buf bytes.Buffer
	cmd := newTraceTestCmd(tr, ex, &buf)
	cmd.SetArgs([]string{"trace", "example.com", "--strict"})
	_ = cmd.Execute()
	if len(ex.codes) != 1 || ex.codes[0] != 1 {
		t.Errorf("strict + any error should exit(1), got %v", ex.codes)
	}
}

func TestDNSTrace_DefaultIgnoresIntermediateErrorIfFinalSucceeds(t *testing.T) {
	finalHop := dnstrace.Hop{Type: dnstrace.HopAnswer, Answers: []string{"1.2.3.4"}}
	tr := &capturedTrace{stubResp: &dnstrace.Result{
		Hops: []dnstrace.Hop{
			{Type: dnstrace.HopError, Error: "first root timeout"},
			finalHop,
		},
		Final: &finalHop,
	}}
	ex := &exitTracker{}
	var buf bytes.Buffer
	cmd := newTraceTestCmd(tr, ex, &buf)
	cmd.SetArgs([]string{"trace", "example.com"})
	_ = cmd.Execute()
	if len(ex.codes) != 0 {
		t.Errorf("default mode + final answer should not exit, got %v", ex.codes)
	}
}

// ------ Timeout flags ------

func TestDNSTrace_RejectsNonpositiveTimeout(t *testing.T) {
	tr := &capturedTrace{}
	ex := &exitTracker{}
	var buf bytes.Buffer
	cmd := newTraceTestCmd(tr, ex, &buf)
	cmd.SilenceUsage = true
	cmd.SetArgs([]string{"trace", "example.com", "--timeout", "0s"})
	if err := cmd.Execute(); err == nil {
		t.Error("expected error")
	}
}

func TestDNSTrace_RejectsNonpositiveHopTimeout(t *testing.T) {
	tr := &capturedTrace{}
	ex := &exitTracker{}
	var buf bytes.Buffer
	cmd := newTraceTestCmd(tr, ex, &buf)
	cmd.SilenceUsage = true
	cmd.SetArgs([]string{"trace", "example.com", "--hop-timeout", "0s"})
	if err := cmd.Execute(); err == nil {
		t.Error("expected error")
	}
}

func TestDNSTrace_HopTimeoutLargerThanTotal(t *testing.T) {
	tr := &capturedTrace{}
	ex := &exitTracker{}
	var buf bytes.Buffer
	cmd := newTraceTestCmd(tr, ex, &buf)
	cmd.SilenceUsage = true
	cmd.SetArgs([]string{"trace", "example.com", "--hop-timeout", "60s", "--timeout", "10s"})
	if err := cmd.Execute(); err == nil {
		t.Error("expected error for hop-timeout > total timeout")
	}
}

// ------ runtime error from trace fn ------

func TestDNSTrace_PropagatesTraceError(t *testing.T) {
	tr := &capturedTrace{stubErr: errors.New("network down")}
	ex := &exitTracker{}
	var buf bytes.Buffer
	cmd := newTraceTestCmd(tr, ex, &buf)
	cmd.SilenceUsage = true
	cmd.SetArgs([]string{"trace", "example.com"})
	err := cmd.Execute()
	if err == nil || !strings.Contains(err.Error(), "network down") {
		t.Errorf("expected propagated error, got: %v", err)
	}
}

// ------ parseTraceType + buildTraceBootstrap （直接单元测试） ------

func TestParseTraceType(t *testing.T) {
	cases := []struct {
		in   string
		want uint16
		err  bool
	}{
		{"A", dns.TypeA, false},
		{"a", dns.TypeA, false},
		{"", dns.TypeA, false},
		{"   A   ", dns.TypeA, false},
		{"NS", dns.TypeNS, false},
		{"MX", dns.TypeMX, false},
		{"INVALID", 0, true},
	}
	for _, c := range cases {
		got, err := parseTraceType(c.in)
		if c.err {
			if err == nil {
				t.Errorf("parseTraceType(%q): expected error", c.in)
			}
			continue
		}
		if err != nil {
			t.Errorf("parseTraceType(%q) error: %v", c.in, err)
			continue
		}
		if got != c.want {
			t.Errorf("parseTraceType(%q) = %d, want %d", c.in, got, c.want)
		}
	}
}

func TestBuildTraceBootstrap_EmptyDoHReturnsOSResolver(t *testing.T) {
	r, err := buildTraceBootstrap("", 5*time.Second)
	if err != nil {
		t.Fatalf("error: %v", err)
	}
	if r == nil {
		t.Fatal("expected non-nil resolver")
	}
}

func TestBuildTraceBootstrap_ValidDoHReturnsResolver(t *testing.T) {
	r, err := buildTraceBootstrap("google", 5*time.Second)
	if err != nil {
		t.Fatalf("error: %v", err)
	}
	if r == nil {
		t.Fatal("expected non-nil resolver")
	}
}

func TestBuildTraceBootstrap_InvalidDoHErrors(t *testing.T) {
	if _, err := buildTraceBootstrap("http://insecure", 5*time.Second); err == nil {
		t.Error("expected error for http:// scheme")
	}
}
