package cli

import (
	"bytes"
	"context"
	"reflect"
	"strings"
	"testing"

	"github.com/miekg/dns"
	"github.com/spf13/cobra"

	"github.com/xunull/jdan/internal/dnslookup"
)

// 用闭包捕获最后一次 Lookup 调用收到的 opts，便于断言 flag → opts 映射。
type capturedLookup struct {
	calls []dnslookup.Options
	stub  *dnslookup.Result
}

func (c *capturedLookup) fn(ctx context.Context, opts dnslookup.Options) (*dnslookup.Result, error) {
	c.calls = append(c.calls, opts)
	if c.stub != nil {
		return c.stub, nil
	}
	return &dnslookup.Result{
		Domain: opts.Domain, Server: opts.Server,
		Results: []dnslookup.TypeResult{
			{Type: "A", Rcode: "NOERROR", TTL: 60, Values: []string{"1.2.3.4"}},
		},
	}, nil
}

type exitTracker struct{ codes []int }

func (e *exitTracker) fn(c int) { e.codes = append(e.codes, c) }

func newTestCmd(cap *capturedLookup, ex *exitTracker, out *bytes.Buffer) *cobra.Command {
	return newDNSCommand(dnsCmdDeps{
		out:          out,
		lookup:       cap.fn,
		detectServer: func() string { return "9.9.9.9:53" },
		exit:         ex.fn,
	})
}

func TestDNSCommand_DefaultTypesAndDetectedServer(t *testing.T) {
	cap := &capturedLookup{}
	ex := &exitTracker{}
	var buf bytes.Buffer
	cmd := newTestCmd(cap, ex, &buf)
	cmd.SetArgs([]string{"lookup", "example.com"})
	if err := cmd.Execute(); err != nil {
		t.Fatalf("Execute: %v", err)
	}
	if len(cap.calls) != 1 {
		t.Fatalf("expected 1 lookup call, got %d", len(cap.calls))
	}
	got := cap.calls[0]
	if !reflect.DeepEqual(got.Types, dnslookup.DefaultTypes()) {
		t.Errorf("default Types wrong: %v", got.Types)
	}
	if got.Server != "9.9.9.9:53" {
		t.Errorf("expected detected server, got %q", got.Server)
	}
	if got.Domain != "example.com" {
		t.Errorf("domain mismatch: %q", got.Domain)
	}
}

func TestDNSCommand_ExplicitTypesAndServer(t *testing.T) {
	cap := &capturedLookup{}
	ex := &exitTracker{}
	var buf bytes.Buffer
	cmd := newTestCmd(cap, ex, &buf)
	cmd.SetArgs([]string{"lookup", "example.com", "-t", "A,MX", "-s", "1.1.1.1:53"})
	if err := cmd.Execute(); err != nil {
		t.Fatalf("Execute: %v", err)
	}
	got := cap.calls[0]
	want := []uint16{dns.TypeA, dns.TypeMX}
	if !reflect.DeepEqual(got.Types, want) {
		t.Errorf("Types = %v, want %v", got.Types, want)
	}
	if got.Server != "1.1.1.1:53" {
		t.Errorf("Server = %q, want 1.1.1.1:53", got.Server)
	}
}

func TestDNSCommand_TypeAll(t *testing.T) {
	cap := &capturedLookup{}
	ex := &exitTracker{}
	var buf bytes.Buffer
	cmd := newTestCmd(cap, ex, &buf)
	cmd.SetArgs([]string{"lookup", "example.com", "-t", "all"})
	if err := cmd.Execute(); err != nil {
		t.Fatalf("Execute: %v", err)
	}
	if len(cap.calls[0].Types) != 9 {
		t.Errorf("expected 9 types for 'all', got %d", len(cap.calls[0].Types))
	}
}

func TestDNSCommand_InvalidTypeReturnsError(t *testing.T) {
	cap := &capturedLookup{}
	ex := &exitTracker{}
	var buf bytes.Buffer
	cmd := newTestCmd(cap, ex, &buf)
	cmd.SilenceUsage = true
	cmd.SetArgs([]string{"lookup", "example.com", "-t", "INVALID"})
	err := cmd.Execute()
	if err == nil {
		t.Fatal("expected error for invalid type")
	}
	if len(cap.calls) != 0 {
		t.Error("lookup should not be called when ParseTypes fails")
	}
}

func TestDNSCommand_ConflictingOutputFlags(t *testing.T) {
	cases := [][]string{
		{"--json", "--short"},
		{"--json", "--verbose"},
		{"--short", "--verbose"},
	}
	for _, c := range cases {
		t.Run(strings.Join(c, " "), func(t *testing.T) {
			cap := &capturedLookup{}
			ex := &exitTracker{}
			var buf bytes.Buffer
			cmd := newTestCmd(cap, ex, &buf)
			cmd.SilenceUsage = true
			args := append([]string{"lookup", "example.com"}, c...)
			cmd.SetArgs(args)
			if err := cmd.Execute(); err == nil {
				t.Error("expected conflict error")
			}
		})
	}
}

func TestDNSCommand_AllFailedExitsOne(t *testing.T) {
	cap := &capturedLookup{
		stub: &dnslookup.Result{
			Domain: "x.invalid", Server: "9.9.9.9:53",
			Results: []dnslookup.TypeResult{
				{Type: "A", Rcode: "NXDOMAIN", Values: []string{}},
				{Type: "AAAA", Rcode: "NXDOMAIN", Values: []string{}},
			},
		},
	}
	ex := &exitTracker{}
	var buf bytes.Buffer
	cmd := newTestCmd(cap, ex, &buf)
	cmd.SetArgs([]string{"lookup", "x.invalid"})
	if err := cmd.Execute(); err != nil {
		t.Fatalf("Execute: %v", err)
	}
	if len(ex.codes) != 1 || ex.codes[0] != 1 {
		t.Errorf("expected exit(1), got %v", ex.codes)
	}
}

func TestDNSCommand_PartialFailDefaultExitZero(t *testing.T) {
	cap := &capturedLookup{
		stub: &dnslookup.Result{
			Domain: "example.com", Server: "9.9.9.9:53",
			Results: []dnslookup.TypeResult{
				{Type: "A", Rcode: "NOERROR", TTL: 60, Values: []string{"1.2.3.4"}},
				{Type: "AAAA", Err: "TIMEOUT", Values: []string{}},
			},
		},
	}
	ex := &exitTracker{}
	var buf bytes.Buffer
	cmd := newTestCmd(cap, ex, &buf)
	cmd.SetArgs([]string{"lookup", "example.com"})
	if err := cmd.Execute(); err != nil {
		t.Fatalf("Execute: %v", err)
	}
	if len(ex.codes) != 0 {
		t.Errorf("default mode partial fail should not exit, got %v", ex.codes)
	}
}

func TestDNSCommand_StrictExitsOnPartialFail(t *testing.T) {
	cap := &capturedLookup{
		stub: &dnslookup.Result{
			Domain: "example.com", Server: "9.9.9.9:53",
			Results: []dnslookup.TypeResult{
				{Type: "A", Rcode: "NOERROR", TTL: 60, Values: []string{"1.2.3.4"}},
				{Type: "AAAA", Err: "TIMEOUT", Values: []string{}},
			},
		},
	}
	ex := &exitTracker{}
	var buf bytes.Buffer
	cmd := newTestCmd(cap, ex, &buf)
	cmd.SetArgs([]string{"lookup", "example.com", "--strict"})
	if err := cmd.Execute(); err != nil {
		t.Fatalf("Execute: %v", err)
	}
	if len(ex.codes) != 1 || ex.codes[0] != 1 {
		t.Errorf("--strict partial fail should exit(1), got %v", ex.codes)
	}
}

func TestDNSCommand_JSONOutput(t *testing.T) {
	cap := &capturedLookup{}
	ex := &exitTracker{}
	var buf bytes.Buffer
	cmd := newTestCmd(cap, ex, &buf)
	cmd.SetArgs([]string{"lookup", "example.com", "--json"})
	if err := cmd.Execute(); err != nil {
		t.Fatalf("Execute: %v", err)
	}
	out := buf.String()
	if !strings.Contains(out, `"domain"`) || !strings.Contains(out, `"results"`) {
		t.Errorf("expected JSON output, got: %s", out)
	}
}

func TestDNSCommand_ShortOutput(t *testing.T) {
	cap := &capturedLookup{}
	ex := &exitTracker{}
	var buf bytes.Buffer
	cmd := newTestCmd(cap, ex, &buf)
	cmd.SetArgs([]string{"lookup", "example.com", "--short"})
	if err := cmd.Execute(); err != nil {
		t.Fatalf("Execute: %v", err)
	}
	out := strings.TrimSpace(buf.String())
	if out != "1.2.3.4" {
		t.Errorf("expected just IP, got %q", out)
	}
}

func TestDNSCommand_DefaultTextHasHeader(t *testing.T) {
	cap := &capturedLookup{}
	ex := &exitTracker{}
	var buf bytes.Buffer
	cmd := newTestCmd(cap, ex, &buf)
	cmd.SetArgs([]string{"lookup", "example.com"})
	if err := cmd.Execute(); err != nil {
		t.Fatalf("Execute: %v", err)
	}
	if !strings.Contains(buf.String(), "example.com — via 9.9.9.9:53") {
		t.Errorf("missing header, got:\n%s", buf.String())
	}
}

func TestDNSCommand_DoHAliasSetsURLAsServer(t *testing.T) {
	cap := &capturedLookup{}
	ex := &exitTracker{}
	var buf bytes.Buffer
	cmd := newTestCmd(cap, ex, &buf)
	cmd.SetArgs([]string{"lookup", "example.com", "--doh", "google"})
	if err := cmd.Execute(); err != nil {
		t.Fatalf("Execute: %v", err)
	}
	if len(cap.calls) != 1 {
		t.Fatalf("expected 1 lookup, got %d", len(cap.calls))
	}
	if cap.calls[0].Server != "https://dns.google/dns-query" {
		t.Errorf("Server = %q, want https://dns.google/dns-query", cap.calls[0].Server)
	}
}

func TestDNSCommand_DoHFullURLPassesThrough(t *testing.T) {
	cap := &capturedLookup{}
	ex := &exitTracker{}
	var buf bytes.Buffer
	cmd := newTestCmd(cap, ex, &buf)
	cmd.SetArgs([]string{"lookup", "example.com", "--doh", "https://custom.example/dns-query"})
	if err := cmd.Execute(); err != nil {
		t.Fatalf("Execute: %v", err)
	}
	if cap.calls[0].Server != "https://custom.example/dns-query" {
		t.Errorf("Server = %q", cap.calls[0].Server)
	}
}

func TestDNSCommand_DoHHostnameAutocompletes(t *testing.T) {
	cap := &capturedLookup{}
	ex := &exitTracker{}
	var buf bytes.Buffer
	cmd := newTestCmd(cap, ex, &buf)
	cmd.SetArgs([]string{"lookup", "example.com", "--doh", "dns.example.com"})
	if err := cmd.Execute(); err != nil {
		t.Fatalf("Execute: %v", err)
	}
	if cap.calls[0].Server != "https://dns.example.com/dns-query" {
		t.Errorf("Server = %q", cap.calls[0].Server)
	}
}

func TestDNSCommand_DoHAndServerMutuallyExclusive(t *testing.T) {
	cap := &capturedLookup{}
	ex := &exitTracker{}
	var buf bytes.Buffer
	cmd := newTestCmd(cap, ex, &buf)
	cmd.SilenceUsage = true
	cmd.SetArgs([]string{"lookup", "example.com", "--doh", "google", "-s", "8.8.8.8"})
	err := cmd.Execute()
	if err == nil {
		t.Fatal("expected error for --doh + --server combo")
	}
	if !strings.Contains(err.Error(), "--doh") || !strings.Contains(err.Error(), "--server") {
		t.Errorf("error should mention --doh and --server, got: %v", err)
	}
	if len(cap.calls) != 0 {
		t.Error("lookup should not run when flags conflict")
	}
}

func TestDNSCommand_DoHInvalidValueReturnsError(t *testing.T) {
	cap := &capturedLookup{}
	ex := &exitTracker{}
	var buf bytes.Buffer
	cmd := newTestCmd(cap, ex, &buf)
	cmd.SilenceUsage = true
	cmd.SetArgs([]string{"lookup", "example.com", "--doh", "http://insecure.example/dns-query"})
	if err := cmd.Execute(); err == nil {
		t.Error("expected error for http:// scheme")
	}
	if len(cap.calls) != 0 {
		t.Error("lookup should not run when --doh value is invalid")
	}
}

func TestDNSCommand_NoDoHFallsBackToDetectedServer(t *testing.T) {
	// regression：不传 --doh 时仍走 detectServer，行为不变
	cap := &capturedLookup{}
	ex := &exitTracker{}
	var buf bytes.Buffer
	cmd := newTestCmd(cap, ex, &buf)
	cmd.SetArgs([]string{"lookup", "example.com"})
	if err := cmd.Execute(); err != nil {
		t.Fatalf("Execute: %v", err)
	}
	if cap.calls[0].Server != "9.9.9.9:53" {
		t.Errorf("Server = %q, want detected 9.9.9.9:53", cap.calls[0].Server)
	}
}

func TestDNSCommand_RejectsNonpositiveTimeout(t *testing.T) {
	cap := &capturedLookup{}
	ex := &exitTracker{}
	var buf bytes.Buffer
	cmd := newTestCmd(cap, ex, &buf)
	cmd.SilenceUsage = true
	cmd.SetArgs([]string{"lookup", "example.com", "--timeout", "0s"})
	if err := cmd.Execute(); err == nil {
		t.Error("expected error for --timeout 0s")
	}
}
