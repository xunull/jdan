package cli

import (
	"bytes"
	"context"
	"errors"
	"strings"
	"testing"
	"time"

	"github.com/xunull/jdan/internal/whois"
)

// 测试用固定 "now" 时间，让 humanizeAgo 输出可预测
var testNow = func() time.Time {
	return time.Date(2026, 6, 14, 12, 0, 0, 0, time.UTC)
}

func fakeLookup(res *whois.Result, err error) func(context.Context, string, time.Duration) (*whois.Result, error) {
	return func(_ context.Context, _ string, _ time.Duration) (*whois.Result, error) {
		return res, err
	}
}

// makeDomainResult 构造带 parsed 字段的 domain Result（fake 数据）
func makeDomainResult() *whois.Result {
	return &whois.Result{
		Target: "example.com",
		Kind:   whois.KindDomain,
		Server: "whois.verisign-grs.com",
		RawText: `   Domain Name: EXAMPLE.COM
   Registrar: Mock Registrar
`,
		Parsed: &whois.Parsed{
			DomainName:       "EXAMPLE.COM",
			Registrar:        "Mock Registrar",
			CreationDate:     time.Date(1995, 8, 14, 4, 0, 0, 0, time.UTC),
			ExpiryDate:       time.Date(2026, 8, 13, 4, 0, 0, 0, time.UTC),
			Status:           []string{"clientDeleteProhibited"},
			Nameservers:      []string{"a.iana-servers.net"},
			DNSSEC:           "signedDelegation",
			RegistryDomainID: "MOCK-123",
		},
	}
}

func TestWhois_Default_ParsedTable(t *testing.T) {
	var buf bytes.Buffer
	cmd := newWhoisCommand(whoisCmdDeps{
		out:    &buf,
		lookup: fakeLookup(makeDomainResult(), nil),
		now:    testNow,
	})
	cmd.SetArgs([]string{"example.com"})
	if err := cmd.Execute(); err != nil {
		t.Fatal(err)
	}
	out := buf.String()
	for _, want := range []string{
		"Target:    example.com (domain)",
		"Server:    whois.verisign-grs.com",
		"Domain:         EXAMPLE.COM",
		"Registrar:      Mock Registrar",
		"Created:        1995-08-14",
		"Expires:        2026-08-13",
		"DNSSEC:         signedDelegation",
		"Status:         clientDeleteProhibited",
		"Nameservers:    a.iana-servers.net",
	} {
		if !strings.Contains(out, want) {
			t.Errorf("missing %q in:\n%s", want, out)
		}
	}
}

func TestWhois_Default_FallsBackToRawWhenParsedEmpty(t *testing.T) {
	var buf bytes.Buffer
	// Parsed 为 nil → 应该走 raw
	res := &whois.Result{
		Target: "weird.tld", Kind: whois.KindDomain,
		Server: "whois.unknown", RawText: "% unknown schema\nweird: stuff\n",
		Parsed: nil,
	}
	cmd := newWhoisCommand(whoisCmdDeps{
		out:    &buf,
		lookup: fakeLookup(res, nil),
		now:    testNow,
	})
	cmd.SetArgs([]string{"weird.tld"})
	if err := cmd.Execute(); err != nil {
		t.Fatal(err)
	}
	out := buf.String()
	// raw header 风格用 '%'
	if !strings.Contains(out, "% Target:") {
		t.Errorf("expected raw header, got:\n%s", out)
	}
	if !strings.Contains(out, "weird: stuff") {
		t.Errorf("raw body missing:\n%s", out)
	}
}

func TestWhois_RawFlag_AlwaysRaw(t *testing.T) {
	var buf bytes.Buffer
	cmd := newWhoisCommand(whoisCmdDeps{
		out:    &buf,
		lookup: fakeLookup(makeDomainResult(), nil),
		now:    testNow,
	})
	cmd.SetArgs([]string{"example.com", "--raw"})
	if err := cmd.Execute(); err != nil {
		t.Fatal(err)
	}
	out := buf.String()
	// 即使有 Parsed，--raw 也只输出 raw（带 '%' header）
	if !strings.Contains(out, "% Target:") {
		t.Errorf("--raw should use raw header style:\n%s", out)
	}
	// 不应该出现 parsed table 的 "Domain:" 标签
	if strings.Contains(out, "Domain:         EXAMPLE.COM") {
		t.Errorf("--raw should not render parsed table:\n%s", out)
	}
}

func TestWhois_FullFlag_BothParsedAndRaw(t *testing.T) {
	var buf bytes.Buffer
	cmd := newWhoisCommand(whoisCmdDeps{
		out:    &buf,
		lookup: fakeLookup(makeDomainResult(), nil),
		now:    testNow,
	})
	cmd.SetArgs([]string{"example.com", "--full"})
	if err := cmd.Execute(); err != nil {
		t.Fatal(err)
	}
	out := buf.String()
	if !strings.Contains(out, "Domain:         EXAMPLE.COM") {
		t.Errorf("--full should render parsed table:\n%s", out)
	}
	if !strings.Contains(out, "--- Raw WHOIS response ---") {
		t.Errorf("--full should append raw separator:\n%s", out)
	}
}

func TestWhois_HopsRendered(t *testing.T) {
	var buf bytes.Buffer
	res := makeDomainResult()
	res.Hops = []whois.Hop{{Server: "whois.iana.org"}}
	cmd := newWhoisCommand(whoisCmdDeps{
		out: &buf, lookup: fakeLookup(res, nil), now: testNow,
	})
	cmd.SetArgs([]string{"example.com"})
	if err := cmd.Execute(); err != nil {
		t.Fatal(err)
	}
	out := buf.String()
	if !strings.Contains(out, "Chain:") {
		t.Errorf("hops missing 'Chain' header:\n%s", out)
	}
	if !strings.Contains(out, "whois.iana.org -> whois.verisign-grs.com") {
		t.Errorf("chain wrong:\n%s", out)
	}
}

func TestWhois_JSONOutput_IncludesParsed(t *testing.T) {
	var buf bytes.Buffer
	cmd := newWhoisCommand(whoisCmdDeps{
		out: &buf, lookup: fakeLookup(makeDomainResult(), nil), now: testNow,
	})
	cmd.SetArgs([]string{"example.com", "--json"})
	if err := cmd.Execute(); err != nil {
		t.Fatal(err)
	}
	out := buf.String()
	for _, want := range []string{
		`"target": "example.com"`,
		`"kind": "domain"`,
		`"server": "whois.verisign-grs.com"`,
		`"parsed": {`,
		`"domain_name": "EXAMPLE.COM"`,
		`"registrar": "Mock Registrar"`,
		`"dnssec": "signedDelegation"`,
	} {
		if !strings.Contains(out, want) {
			t.Errorf("missing %q in:\n%s", want, out)
		}
	}
}

func TestWhois_ServerFlag_BypassesRouting(t *testing.T) {
	var buf bytes.Buffer
	called := false
	cmd := newWhoisCommand(whoisCmdDeps{
		out: &buf,
		now: testNow,
		lookupWithServer: func(_ context.Context, target, server string, _ time.Duration) (*whois.Result, error) {
			called = true
			if server != "custom.whois.example" {
				t.Errorf("server passed = %q", server)
			}
			if target != "example.com" {
				t.Errorf("target passed = %q", target)
			}
			return makeDomainResult(), nil
		},
		lookup: func(_ context.Context, _ string, _ time.Duration) (*whois.Result, error) {
			t.Error("Lookup should not be called when --server is given")
			return nil, errors.New("should not be called")
		},
	})
	cmd.SetArgs([]string{"example.com", "--server", "custom.whois.example"})
	if err := cmd.Execute(); err != nil {
		t.Fatal(err)
	}
	if !called {
		t.Error("LookupWithServer was not called")
	}
}

func TestWhois_PropagatesLookupError(t *testing.T) {
	cmd := newWhoisCommand(whoisCmdDeps{
		out:    &bytes.Buffer{},
		now:    testNow,
		lookup: fakeLookup(nil, errors.New("dial: connection refused")),
	})
	cmd.SetArgs([]string{"example.com"})
	err := cmd.Execute()
	if err == nil || !strings.Contains(err.Error(), "connection refused") {
		t.Errorf("expected connection refused error, got %v", err)
	}
}

func TestWhois_IPv4ParsedTable(t *testing.T) {
	var buf bytes.Buffer
	res := &whois.Result{
		Target: "8.8.8.8", Kind: whois.KindIPv4,
		Server: "whois.arin.net",
		Parsed: &whois.Parsed{
			NetRange:   "8.8.8.0 - 8.8.8.255",
			NetName:    "GOOGLE",
			OrgName:    "Google LLC",
			Country:    "US",
			AbuseEmail: "abuse@google.com",
		},
	}
	cmd := newWhoisCommand(whoisCmdDeps{
		out: &buf, lookup: fakeLookup(res, nil), now: testNow,
	})
	cmd.SetArgs([]string{"8.8.8.8"})
	if err := cmd.Execute(); err != nil {
		t.Fatal(err)
	}
	out := buf.String()
	for _, want := range []string{
		"Target:    8.8.8.8 (ipv4)",
		"Range:          8.8.8.0 - 8.8.8.255",
		"Org:            Google LLC",
		"Country:        US",
		"Abuse email:    abuse@google.com",
	} {
		if !strings.Contains(out, want) {
			t.Errorf("missing %q in:\n%s", want, out)
		}
	}
}

func TestWhois_MutuallyExclusiveFlags(t *testing.T) {
	cmd := newWhoisCommand(whoisCmdDeps{
		out: &bytes.Buffer{}, lookup: fakeLookup(makeDomainResult(), nil), now: testNow,
	})
	cmd.SetArgs([]string{"example.com", "--raw", "--json"})
	if err := cmd.Execute(); err == nil {
		t.Error("--raw and --json should be mutually exclusive")
	}
}

func TestHumanizeAgo_PastAndFuture(t *testing.T) {
	now := time.Date(2026, 6, 14, 12, 0, 0, 0, time.UTC)
	cases := []struct {
		t    time.Time
		want string
	}{
		{now.Add(-30 * time.Second), "30 seconds ago"},
		{now.Add(-2 * time.Hour), "2 hours ago"},
		{now.Add(-2 * 24 * time.Hour), "2 days ago"},
		{now.Add(2 * time.Hour), "in 2 hours"},
		{now.Add(60 * 24 * time.Hour), "in 2 months"},
	}
	for _, c := range cases {
		got := humanizeAgo(c.t, now)
		if got != c.want {
			t.Errorf("humanizeAgo(%v) = %q, want %q", c.t, got, c.want)
		}
	}
}
