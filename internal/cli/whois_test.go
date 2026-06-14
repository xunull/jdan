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

// fakeLookup 实现 deps.lookup 接口，回返 stub Result。
func fakeLookup(res *whois.Result, err error) func(context.Context, string, time.Duration) (*whois.Result, error) {
	return func(_ context.Context, _ string, _ time.Duration) (*whois.Result, error) {
		return res, err
	}
}

func fakeLookupWithServer(res *whois.Result, err error) func(context.Context, string, string, time.Duration) (*whois.Result, error) {
	return func(_ context.Context, _, _ string, _ time.Duration) (*whois.Result, error) {
		return res, err
	}
}

func TestWhois_Default_RawOutput(t *testing.T) {
	var buf bytes.Buffer
	res := &whois.Result{
		Target:  "example.com",
		Kind:    whois.KindDomain,
		Server:  "whois.verisign-grs.com",
		RawText: "Domain Name: EXAMPLE.COM\nRegistrar: Mock\n",
	}
	cmd := newWhoisCommand(whoisCmdDeps{
		out:    &buf,
		lookup: fakeLookup(res, nil),
	})
	cmd.SetArgs([]string{"example.com"})
	if err := cmd.Execute(); err != nil {
		t.Fatal(err)
	}
	out := buf.String()
	for _, want := range []string{
		"Target: example.com (domain)",
		"Server: whois.verisign-grs.com",
		"Domain Name: EXAMPLE.COM",
		"Registrar: Mock",
	} {
		if !strings.Contains(out, want) {
			t.Errorf("missing %q in:\n%s", want, out)
		}
	}
}

func TestWhois_HopsRendered(t *testing.T) {
	var buf bytes.Buffer
	res := &whois.Result{
		Target:  "example.weird",
		Kind:    whois.KindDomain,
		Server:  "whois.real-tld.example",
		Hops:    []whois.Hop{{Server: "whois.iana.org"}},
		RawText: "Domain Name: example.weird\n",
	}
	cmd := newWhoisCommand(whoisCmdDeps{
		out:    &buf,
		lookup: fakeLookup(res, nil),
	})
	cmd.SetArgs([]string{"example.weird"})
	if err := cmd.Execute(); err != nil {
		t.Fatal(err)
	}
	out := buf.String()
	if !strings.Contains(out, "Chain:") {
		t.Errorf("hops missing 'Chain' header:\n%s", out)
	}
	if !strings.Contains(out, "whois.iana.org -> whois.real-tld.example") {
		t.Errorf("chain order wrong:\n%s", out)
	}
}

func TestWhois_JSONOutput(t *testing.T) {
	var buf bytes.Buffer
	res := &whois.Result{
		Target:  "example.com",
		Kind:    whois.KindDomain,
		Server:  "whois.verisign-grs.com",
		RawText: "x: y\n",
	}
	cmd := newWhoisCommand(whoisCmdDeps{
		out:    &buf,
		lookup: fakeLookup(res, nil),
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
		`"raw": "x: y\n"`,
	} {
		if !strings.Contains(out, want) {
			t.Errorf("missing %q in:\n%s", want, out)
		}
	}
}

func TestWhois_ServerFlag_BypassesRouting(t *testing.T) {
	var buf bytes.Buffer
	called := false
	res := &whois.Result{
		Target: "example.com", Kind: whois.KindDomain,
		Server: "custom.whois.example", RawText: "ok\n",
	}
	cmd := newWhoisCommand(whoisCmdDeps{
		out: &buf,
		lookupWithServer: func(_ context.Context, target, server string, _ time.Duration) (*whois.Result, error) {
			called = true
			if server != "custom.whois.example" {
				t.Errorf("server passed = %q", server)
			}
			if target != "example.com" {
				t.Errorf("target passed = %q", target)
			}
			return res, nil
		},
		// lookup 应当不被调用
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
		lookup: fakeLookup(nil, errors.New("dial: connection refused")),
	})
	cmd.SetArgs([]string{"example.com"})
	err := cmd.Execute()
	if err == nil || !strings.Contains(err.Error(), "connection refused") {
		t.Errorf("expected connection refused error, got %v", err)
	}
}

func TestWhois_IPv4Target(t *testing.T) {
	var buf bytes.Buffer
	res := &whois.Result{
		Target: "8.8.8.8", Kind: whois.KindIPv4,
		Server: "whois.arin.net", RawText: "NetRange: 8.8.8.0/24\n",
	}
	cmd := newWhoisCommand(whoisCmdDeps{
		out:    &buf,
		lookup: fakeLookup(res, nil),
	})
	cmd.SetArgs([]string{"8.8.8.8"})
	if err := cmd.Execute(); err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(buf.String(), "(ipv4)") {
		t.Errorf("ipv4 kind missing:\n%s", buf.String())
	}
}

// 测试 LookupWithServer 走 ServerFlag 时 lookup 不会被调用是上面 TestWhois_ServerFlag_BypassesRouting
// 已经覆盖。本测试再保证：默认（无 --server）走 Lookup 不走 LookupWithServer。
func TestWhois_NoServerFlag_GoesThroughLookup(t *testing.T) {
	var buf bytes.Buffer
	called := false
	res := &whois.Result{
		Target: "example.com", Kind: whois.KindDomain,
		Server: "whois.verisign-grs.com", RawText: "ok\n",
	}
	cmd := newWhoisCommand(whoisCmdDeps{
		out: &buf,
		lookup: func(_ context.Context, _ string, _ time.Duration) (*whois.Result, error) {
			called = true
			return res, nil
		},
		lookupWithServer: func(_ context.Context, _, _ string, _ time.Duration) (*whois.Result, error) {
			t.Error("LookupWithServer should not be called without --server")
			return nil, errors.New("should not be called")
		},
	})
	cmd.SetArgs([]string{"example.com"})
	if err := cmd.Execute(); err != nil {
		t.Fatal(err)
	}
	if !called {
		t.Error("Lookup was not called")
	}
}
