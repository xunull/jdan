package cli

import (
	"bytes"
	"net"
	"strings"
	"testing"

	"github.com/miekg/dns"
	"github.com/spf13/cobra"
)

// --- parseReverseIP（纯单元测试） ---

func TestParseReverseIP_IPv4Valid(t *testing.T) {
	cases := []string{"8.8.8.8", "0.0.0.0", "127.0.0.1", "192.168.1.1", "255.255.255.255"}
	for _, c := range cases {
		t.Run(c, func(t *testing.T) {
			ip, err := parseReverseIP(c)
			if err != nil {
				t.Fatalf("parseReverseIP(%q) returned error: %v", c, err)
			}
			if ip.String() != c {
				t.Errorf("got %q, want %q", ip.String(), c)
			}
		})
	}
}

func TestParseReverseIP_IPv6Valid(t *testing.T) {
	cases := []string{"::1", "2001:db8::1", "2001:4860:4860::8888", "fe80::1"}
	for _, c := range cases {
		t.Run(c, func(t *testing.T) {
			ip, err := parseReverseIP(c)
			if err != nil {
				t.Fatalf("parseReverseIP(%q) returned error: %v", c, err)
			}
			if ip.To4() != nil {
				t.Errorf("%q should parse as IPv6, got IPv4 form", c)
			}
		})
	}
}

func TestParseReverseIP_TrimsWhitespace(t *testing.T) {
	ip, err := parseReverseIP("   8.8.8.8   ")
	if err != nil {
		t.Fatalf("error: %v", err)
	}
	if ip.String() != "8.8.8.8" {
		t.Errorf("expected trimmed result, got %q", ip.String())
	}
}

func TestParseReverseIP_RejectsEmpty(t *testing.T) {
	if _, err := parseReverseIP(""); err == nil {
		t.Error("empty input should error")
	}
	if _, err := parseReverseIP("   "); err == nil {
		t.Error("whitespace-only input should error")
	}
}

func TestParseReverseIP_RejectsCIDR(t *testing.T) {
	cases := []string{"8.8.8.8/32", "192.168.0.0/16", "2001:db8::/32"}
	for _, c := range cases {
		t.Run(c, func(t *testing.T) {
			_, err := parseReverseIP(c)
			if err == nil {
				t.Errorf("CIDR %q should be rejected", c)
				return
			}
			if !strings.Contains(err.Error(), "CIDR") {
				t.Errorf("error should mention CIDR for %q, got: %v", c, err)
			}
		})
	}
}

func TestParseReverseIP_RejectsHostPort(t *testing.T) {
	cases := []string{"8.8.8.8:53", "[::1]:53", "[2001:db8::1]:5353"}
	for _, c := range cases {
		t.Run(c, func(t *testing.T) {
			_, err := parseReverseIP(c)
			if err == nil {
				t.Errorf("host:port %q should be rejected", c)
				return
			}
			if !strings.Contains(err.Error(), "端口") && !strings.Contains(err.Error(), "port") {
				t.Errorf("error should mention port for %q, got: %v", c, err)
			}
		})
	}
}

func TestParseReverseIP_DomainSuggestsLookup(t *testing.T) {
	cases := []string{"google.com", "example.com", "dns.google"}
	for _, c := range cases {
		t.Run(c, func(t *testing.T) {
			_, err := parseReverseIP(c)
			if err == nil {
				t.Errorf("domain %q should be rejected", c)
				return
			}
			if !strings.Contains(err.Error(), "jdan dns lookup") {
				t.Errorf("error should suggest `jdan dns lookup` for domain %q, got: %v", c, err)
			}
		})
	}
}

func TestParseReverseIP_RejectsGarbage(t *testing.T) {
	cases := []string{
		"not-an-ip",
		"fe80::1%en0",          // link-local with zone-id：ParseIP 返回 nil
		"999.999.999.999",      // out-of-range IPv4
		"::g",                  // 含非法 hex 字符的 IPv6
	}
	for _, c := range cases {
		t.Run(c, func(t *testing.T) {
			_, err := parseReverseIP(c)
			if err == nil {
				t.Errorf("garbage %q should be rejected", c)
			}
		})
	}
}

// --- dns reverse CLI 命令（用 mock lookup 验证 wiring） ---

func newReverseTestCmd(cap *capturedLookup, ex *exitTracker, out *bytes.Buffer) *cobra.Command {
	return newDNSCommand(dnsCmdDeps{
		out:          out,
		lookup:       cap.fn,
		detectServer: func() string { return "9.9.9.9:53" },
		exit:         ex.fn,
	})
}

func TestDNSReverse_IPv4ConvertsToInAddrArpa(t *testing.T) {
	cap := &capturedLookup{}
	ex := &exitTracker{}
	var buf bytes.Buffer
	cmd := newReverseTestCmd(cap, ex, &buf)
	cmd.SetArgs([]string{"reverse", "8.8.8.8"})
	if err := cmd.Execute(); err != nil {
		t.Fatalf("Execute: %v", err)
	}
	if len(cap.calls) != 1 {
		t.Fatalf("expected 1 lookup call, got %d", len(cap.calls))
	}
	got := cap.calls[0]
	if got.Domain != "8.8.8.8.in-addr.arpa." {
		t.Errorf("Domain = %q, want 8.8.8.8.in-addr.arpa.", got.Domain)
	}
	if got.DisplayName != "8.8.8.8" {
		t.Errorf("DisplayName = %q, want 8.8.8.8 (原始 IP)", got.DisplayName)
	}
	if len(got.Types) != 1 || got.Types[0] != dns.TypePTR {
		t.Errorf("Types = %v, want [TypePTR]", got.Types)
	}
}

func TestDNSReverse_IPv6ConvertsToIp6Arpa(t *testing.T) {
	cap := &capturedLookup{}
	ex := &exitTracker{}
	var buf bytes.Buffer
	cmd := newReverseTestCmd(cap, ex, &buf)
	cmd.SetArgs([]string{"reverse", "2001:4860:4860::8888"})
	if err := cmd.Execute(); err != nil {
		t.Fatalf("Execute: %v", err)
	}
	got := cap.calls[0]
	if !strings.HasSuffix(got.Domain, ".ip6.arpa.") {
		t.Errorf("IPv6 Domain should end in .ip6.arpa., got %q", got.Domain)
	}
	if got.DisplayName != net.ParseIP("2001:4860:4860::8888").String() {
		t.Errorf("DisplayName should be normalized IPv6, got %q", got.DisplayName)
	}
}

func TestDNSReverse_RejectionShortCircuitsLookup(t *testing.T) {
	// 每个拒绝用例都应在 lookup 被调用前失败
	cases := []struct {
		name, arg string
	}{
		{"domain", "google.com"},
		{"cidr", "8.8.8.8/32"},
		{"port", "8.8.8.8:53"},
		{"garbage", "not-an-ip"},
		{"zone-id", "fe80::1%en0"},
	}
	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			cap := &capturedLookup{}
			ex := &exitTracker{}
			var buf bytes.Buffer
			cmd := newReverseTestCmd(cap, ex, &buf)
			cmd.SilenceUsage = true
			cmd.SetArgs([]string{"reverse", c.arg})
			err := cmd.Execute()
			if err == nil {
				t.Errorf("expected error for %q", c.arg)
				return
			}
			if len(cap.calls) != 0 {
				t.Errorf("lookup must not be called when input is invalid, got %d calls", len(cap.calls))
			}
		})
	}
}

func TestDNSReverse_ReusesDoHFlag(t *testing.T) {
	cap := &capturedLookup{}
	ex := &exitTracker{}
	var buf bytes.Buffer
	cmd := newReverseTestCmd(cap, ex, &buf)
	cmd.SetArgs([]string{"reverse", "8.8.8.8", "--doh", "google"})
	if err := cmd.Execute(); err != nil {
		t.Fatalf("Execute: %v", err)
	}
	got := cap.calls[0]
	if got.Server != "https://dns.google/dns-query" {
		t.Errorf("Server = %q, want DoH URL", got.Server)
	}
}

func TestDNSReverse_ReusesExplicitServer(t *testing.T) {
	cap := &capturedLookup{}
	ex := &exitTracker{}
	var buf bytes.Buffer
	cmd := newReverseTestCmd(cap, ex, &buf)
	cmd.SetArgs([]string{"reverse", "8.8.8.8", "-s", "1.1.1.1:53"})
	if err := cmd.Execute(); err != nil {
		t.Fatalf("Execute: %v", err)
	}
	if cap.calls[0].Server != "1.1.1.1:53" {
		t.Errorf("Server = %q, want 1.1.1.1:53", cap.calls[0].Server)
	}
}

func TestDNSReverse_DoHAndServerMutuallyExclusive(t *testing.T) {
	cap := &capturedLookup{}
	ex := &exitTracker{}
	var buf bytes.Buffer
	cmd := newReverseTestCmd(cap, ex, &buf)
	cmd.SilenceUsage = true
	cmd.SetArgs([]string{"reverse", "8.8.8.8", "--doh", "google", "-s", "1.1.1.1"})
	if err := cmd.Execute(); err == nil {
		t.Error("expected mutex error")
	}
	if len(cap.calls) != 0 {
		t.Error("lookup should not run on conflict")
	}
}

func TestDNSReverse_HasNoTypeFlag(t *testing.T) {
	// reverse 命令不应注册 --type flag（只查 PTR）
	cap := &capturedLookup{}
	ex := &exitTracker{}
	var buf bytes.Buffer
	cmd := newReverseTestCmd(cap, ex, &buf)
	cmd.SilenceUsage = true
	cmd.SetArgs([]string{"reverse", "8.8.8.8", "-t", "A"})
	if err := cmd.Execute(); err == nil {
		t.Error("expected unknown flag error for -t on reverse")
	}
}

func TestDNSReverse_RejectsNonpositiveTimeout(t *testing.T) {
	cap := &capturedLookup{}
	ex := &exitTracker{}
	var buf bytes.Buffer
	cmd := newReverseTestCmd(cap, ex, &buf)
	cmd.SilenceUsage = true
	cmd.SetArgs([]string{"reverse", "8.8.8.8", "--timeout", "0s"})
	if err := cmd.Execute(); err == nil {
		t.Error("expected timeout error")
	}
}
