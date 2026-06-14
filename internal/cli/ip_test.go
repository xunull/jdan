package cli

import (
	"bytes"
	"strings"
	"testing"
)

func TestIPInfo_IPv4Address(t *testing.T) {
	var buf bytes.Buffer
	cmd := newIPCommand(ipCmdDeps{out: &buf})
	cmd.SetArgs([]string{"info", "192.168.1.42"})
	if err := cmd.Execute(); err != nil {
		t.Fatal(err)
	}
	out := buf.String()
	for _, want := range []string{
		"Address:        192.168.1.42",
		"Version:        IPv4",
		"Hex:            0xC0A8012A",
		"Decimal:        3232235818",
		"Binary:         11000000.10101000.00000001.00101010",
		"Reverse DNS:    42.1.168.192.in-addr.arpa",
		"Private:        yes",
	} {
		if !strings.Contains(out, want) {
			t.Errorf("missing %q in:\n%s", want, out)
		}
	}
}

func TestIPInfo_IPv4CIDR(t *testing.T) {
	var buf bytes.Buffer
	cmd := newIPCommand(ipCmdDeps{out: &buf})
	cmd.SetArgs([]string{"info", "192.168.1.0/24"})
	if err := cmd.Execute(); err != nil {
		t.Fatal(err)
	}
	out := buf.String()
	for _, want := range []string{
		"CIDR:           192.168.1.0/24",
		"Network:        192.168.1.0",
		"Broadcast:      192.168.1.255",
		"First host:     192.168.1.1",
		"Last host:      192.168.1.254",
		"Netmask:        255.255.255.0",
		"Wildcard:       0.0.0.255",
		"Total IPs:      256",
		"Usable:         254",
	} {
		if !strings.Contains(out, want) {
			t.Errorf("missing %q in:\n%s", want, out)
		}
	}
}

func TestIPInfo_IPv6(t *testing.T) {
	var buf bytes.Buffer
	cmd := newIPCommand(ipCmdDeps{out: &buf})
	cmd.SetArgs([]string{"info", "2001:db8::1"})
	if err := cmd.Execute(); err != nil {
		t.Fatal(err)
	}
	out := buf.String()
	for _, want := range []string{
		"Version:        IPv6",
		"Compact:        2001:db8::1",
		"Expanded:       2001:0db8:0000:0000:0000:0000:0000:0001",
		"Doc range:      yes",
	} {
		if !strings.Contains(out, want) {
			t.Errorf("missing %q in:\n%s", want, out)
		}
	}
}

func TestIPInfo_JSON(t *testing.T) {
	var buf bytes.Buffer
	cmd := newIPCommand(ipCmdDeps{out: &buf})
	cmd.SetArgs([]string{"info", "192.168.1.0/24", "--json"})
	if err := cmd.Execute(); err != nil {
		t.Fatal(err)
	}
	out := buf.String()
	for _, want := range []string{
		`"prefix": "192.168.1.0/24"`,
		`"version": 4`,
		`"network": "192.168.1.0"`,
		`"broadcast": "192.168.1.255"`,
		`"total_addrs": "256"`,
		`"usable_addrs": "254"`,
	} {
		if !strings.Contains(out, want) {
			t.Errorf("missing %q in:\n%s", want, out)
		}
	}
}

func TestIPInfo_InvalidInput(t *testing.T) {
	cmd := newIPCommand(ipCmdDeps{out: &bytes.Buffer{}})
	cmd.SetArgs([]string{"info", "not-an-ip"})
	if err := cmd.Execute(); err == nil {
		t.Error("expected error for invalid input")
	}
}

func TestIPContains_True_ExitOK(t *testing.T) {
	cmd := newIPCommand(ipCmdDeps{out: &bytes.Buffer{}})
	cmd.SetArgs([]string{"contains", "10.0.0.0/8", "10.5.1.2"})
	if err := cmd.Execute(); err != nil {
		t.Errorf("expected nil error for in-net, got %v", err)
	}
}

func TestIPContains_False_ExitErr(t *testing.T) {
	cmd := newIPCommand(ipCmdDeps{out: &bytes.Buffer{}})
	cmd.SetArgs([]string{"contains", "10.0.0.0/8", "192.168.1.1"})
	err := cmd.Execute()
	if err == nil {
		t.Error("expected error for not-in-net")
	}
	if _, ok := err.(*ipCmdExitErr); !ok {
		t.Errorf("expected *ipCmdExitErr, got %T", err)
	}
}

func TestIPContains_VerboseYes(t *testing.T) {
	var buf bytes.Buffer
	cmd := newIPCommand(ipCmdDeps{out: &buf})
	cmd.SetArgs([]string{"contains", "10.0.0.0/8", "10.5.1.2", "--verbose"})
	_ = cmd.Execute()
	if strings.TrimSpace(buf.String()) != "yes" {
		t.Errorf("got %q, want yes", buf.String())
	}
}

func TestIPContains_VerboseNo(t *testing.T) {
	var buf bytes.Buffer
	cmd := newIPCommand(ipCmdDeps{out: &buf})
	cmd.SetArgs([]string{"contains", "10.0.0.0/8", "192.168.1.1", "--verbose"})
	_ = cmd.Execute()
	if strings.TrimSpace(buf.String()) != "no" {
		t.Errorf("got %q, want no", buf.String())
	}
}

func TestIPContains_FamilyMismatch(t *testing.T) {
	cmd := newIPCommand(ipCmdDeps{out: &bytes.Buffer{}})
	cmd.SetArgs([]string{"contains", "10.0.0.0/8", "2001:db8::1"})
	err := cmd.Execute()
	if err == nil || !strings.Contains(err.Error(), "family mismatch") {
		t.Errorf("expected family mismatch error, got %v", err)
	}
}

func TestIPRange_Small(t *testing.T) {
	var buf bytes.Buffer
	cmd := newIPCommand(ipCmdDeps{out: &buf})
	cmd.SetArgs([]string{"range", "192.168.1.0/29"})
	if err := cmd.Execute(); err != nil {
		t.Fatal(err)
	}
	out := buf.String()
	for _, want := range []string{
		"192.168.1.0", "192.168.1.1", "192.168.1.7",
		"(8 total)",
	} {
		if !strings.Contains(out, want) {
			t.Errorf("missing %q in:\n%s", want, out)
		}
	}
}

func TestIPRange_LargeWithLimit(t *testing.T) {
	var buf bytes.Buffer
	cmd := newIPCommand(ipCmdDeps{out: &buf})
	cmd.SetArgs([]string{"range", "10.0.0.0/8", "--limit", "3"})
	if err := cmd.Execute(); err != nil {
		t.Fatal(err)
	}
	out := buf.String()
	if !strings.Contains(out, "16777216 total") {
		t.Errorf("missing 'total' summary:\n%s", out)
	}
	if !strings.Contains(out, "showing first 3") {
		t.Errorf("missing 'showing first 3':\n%s", out)
	}
}

func TestIPRange_JSON(t *testing.T) {
	var buf bytes.Buffer
	cmd := newIPCommand(ipCmdDeps{out: &buf})
	cmd.SetArgs([]string{"range", "192.168.1.0/30", "--json"})
	if err := cmd.Execute(); err != nil {
		t.Fatal(err)
	}
	out := buf.String()
	for _, want := range []string{
		`"cidr": "192.168.1.0/30"`,
		`"returned": 4`,
		`"total": "4"`,
		`"192.168.1.0"`,
		`"192.168.1.3"`,
	} {
		if !strings.Contains(out, want) {
			t.Errorf("missing %q in:\n%s", want, out)
		}
	}
}

func TestIPSplit_Basic(t *testing.T) {
	var buf bytes.Buffer
	cmd := newIPCommand(ipCmdDeps{out: &buf})
	cmd.SetArgs([]string{"split", "10.0.0.0/22", "24"})
	if err := cmd.Execute(); err != nil {
		t.Fatal(err)
	}
	out := buf.String()
	for _, want := range []string{
		"10.0.0.0/24",
		"10.0.1.0/24",
		"10.0.2.0/24",
		"10.0.3.0/24",
		"(4 subnets)",
	} {
		if !strings.Contains(out, want) {
			t.Errorf("missing %q in:\n%s", want, out)
		}
	}
}

func TestIPSplit_InvalidNewBits(t *testing.T) {
	cmd := newIPCommand(ipCmdDeps{out: &bytes.Buffer{}})
	cmd.SetArgs([]string{"split", "10.0.0.0/22", "20"}) // 20 < 22
	if err := cmd.Execute(); err == nil {
		t.Error("expected error when newBits < parent bits")
	}
}

func TestIPSplit_JSON(t *testing.T) {
	var buf bytes.Buffer
	cmd := newIPCommand(ipCmdDeps{out: &buf})
	cmd.SetArgs([]string{"split", "10.0.0.0/22", "24", "--json"})
	if err := cmd.Execute(); err != nil {
		t.Fatal(err)
	}
	out := buf.String()
	for _, want := range []string{
		`"parent": "10.0.0.0/22"`,
		`"new_len": 24`,
		`"count": 4`,
	} {
		if !strings.Contains(out, want) {
			t.Errorf("missing %q in:\n%s", want, out)
		}
	}
}

func TestIPNormalize_Default_Compact(t *testing.T) {
	var buf bytes.Buffer
	cmd := newIPCommand(ipCmdDeps{out: &buf})
	cmd.SetArgs([]string{"normalize", "2001:0db8:0000:0000:0000:0000:0000:0001"})
	if err := cmd.Execute(); err != nil {
		t.Fatal(err)
	}
	got := strings.TrimSpace(buf.String())
	if got != "2001:db8::1" {
		t.Errorf("got %q", got)
	}
}

func TestIPNormalize_Expand(t *testing.T) {
	var buf bytes.Buffer
	cmd := newIPCommand(ipCmdDeps{out: &buf})
	cmd.SetArgs([]string{"normalize", "2001:db8::1", "--expand"})
	if err := cmd.Execute(); err != nil {
		t.Fatal(err)
	}
	got := strings.TrimSpace(buf.String())
	if got != "2001:0db8:0000:0000:0000:0000:0000:0001" {
		t.Errorf("got %q", got)
	}
}

func TestIPNormalize_MutexFlags(t *testing.T) {
	cmd := newIPCommand(ipCmdDeps{out: &bytes.Buffer{}})
	cmd.SetArgs([]string{"normalize", "2001:db8::1", "--expand", "--compact"})
	if err := cmd.Execute(); err == nil {
		t.Error("expected mutex flags error")
	}
}

func TestIPNormalize_IPv4NoOp(t *testing.T) {
	var buf bytes.Buffer
	cmd := newIPCommand(ipCmdDeps{out: &buf})
	cmd.SetArgs([]string{"normalize", "192.168.1.1", "--expand"})
	if err := cmd.Execute(); err != nil {
		t.Fatal(err)
	}
	got := strings.TrimSpace(buf.String())
	if got != "192.168.1.1" {
		t.Errorf("got %q", got)
	}
}

func TestParsePositiveInt(t *testing.T) {
	cases := []struct {
		in   string
		want int
		ok   bool
	}{
		{"0", 0, true},
		{"24", 24, true},
		{"128", 128, true},
		{"129", 0, false},
		{"-1", 0, false},
		{"abc", 0, false},
		{"", 0, false},
	}
	for _, c := range cases {
		got, err := parsePositiveInt(c.in)
		if c.ok && err != nil {
			t.Errorf("parsePositiveInt(%q) errored: %v", c.in, err)
		}
		if !c.ok && err == nil {
			t.Errorf("parsePositiveInt(%q) should error", c.in)
		}
		if c.ok && got != c.want {
			t.Errorf("parsePositiveInt(%q) = %d, want %d", c.in, got, c.want)
		}
	}
}
