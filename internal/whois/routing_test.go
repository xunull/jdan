package whois

import (
	"testing"
)

func TestDetectKind_Domain(t *testing.T) {
	cases := []string{"example.com", "x.example.co.uk", "google.de"}
	for _, c := range cases {
		k, err := detectKind(c)
		if err != nil {
			t.Errorf("detectKind(%q) errored: %v", c, err)
			continue
		}
		if k != KindDomain {
			t.Errorf("detectKind(%q) = %q, want domain", c, k)
		}
	}
}

func TestDetectKind_IPv4(t *testing.T) {
	cases := []string{"8.8.8.8", "192.168.1.1", "0.0.0.0", "255.255.255.255"}
	for _, c := range cases {
		k, err := detectKind(c)
		if err != nil {
			t.Errorf("detectKind(%q) errored: %v", c, err)
			continue
		}
		if k != KindIPv4 {
			t.Errorf("detectKind(%q) = %q, want ipv4", c, k)
		}
	}
}

func TestDetectKind_IPv6(t *testing.T) {
	cases := []string{"2001:db8::1", "::1", "fe80::1"}
	for _, c := range cases {
		k, err := detectKind(c)
		if err != nil {
			t.Errorf("detectKind(%q) errored: %v", c, err)
			continue
		}
		if k != KindIPv6 {
			t.Errorf("detectKind(%q) = %q, want ipv6", c, k)
		}
	}
}

func TestDetectKind_BareHostname_Errors(t *testing.T) {
	// 无 dot 的 hostname 不能 WHOIS（不是 TLD）
	if _, err := detectKind("localhost"); err == nil {
		t.Error("bare hostname should error")
	}
}

func TestExtractTLD(t *testing.T) {
	cases := map[string]string{
		"example.com":      "com",
		"x.example.co.uk":  "uk",
		"GOOGLE.DE":        "de",
		"trailing.dot.io.": "io",
		" leading.space ":  "space", // 注意: leading-space 这种异常输入会得 "space" 不是 expected
	}
	// 修正：" leading.space " trim 后是 "leading.space"，TLD = "space"
	for in, want := range cases {
		got := extractTLD(in)
		if got != want {
			t.Errorf("extractTLD(%q) = %q, want %q", in, got, want)
		}
	}
}

func TestRoutingFor_KnownTLD(t *testing.T) {
	cases := map[string]string{
		"example.com":  "whois.verisign-grs.com",
		"example.io":   "whois.nic.io",
		"example.de":   "whois.denic.de",
		"example.cn":   "whois.cnnic.cn",
		"example.app":  "whois.nic.google",
	}
	for target, wantServer := range cases {
		got, kind, err := RoutingFor(target)
		if err != nil {
			t.Errorf("RoutingFor(%q) errored: %v", target, err)
			continue
		}
		if kind != KindDomain {
			t.Errorf("RoutingFor(%q) kind = %q, want domain", target, kind)
		}
		if got != wantServer {
			t.Errorf("RoutingFor(%q) server = %q, want %q", target, got, wantServer)
		}
	}
}

func TestRoutingFor_UnknownTLD_FallsBackToIANA(t *testing.T) {
	got, kind, err := RoutingFor("example.invalidtld-xyz-12345")
	if err != nil {
		t.Fatal(err)
	}
	if kind != KindDomain {
		t.Errorf("kind = %q, want domain", kind)
	}
	if got != IANARoot {
		t.Errorf("unknown TLD should fall back to IANA, got %q", got)
	}
}

func TestRoutingFor_IP_GoesToARIN(t *testing.T) {
	for _, ip := range []string{"8.8.8.8", "2001:db8::1"} {
		got, _, err := RoutingFor(ip)
		if err != nil {
			t.Fatal(err)
		}
		if got != ARINRoot {
			t.Errorf("RoutingFor(%q) = %q, want ARIN root", ip, got)
		}
	}
}

func TestParseIANAReferral_WhoisLine(t *testing.T) {
	raw := `% IANA WHOIS server
domain:       COM
organisation: VeriSign Global Registry Services
whois:        whois.verisign-grs.com
status:       ACTIVE
`
	got := ParseIANAReferral(raw)
	if got != "whois.verisign-grs.com" {
		t.Errorf("got %q", got)
	}
}

func TestParseIANAReferral_ReferLine(t *testing.T) {
	raw := "refer: whois.example.net\n"
	if got := ParseIANAReferral(raw); got != "whois.example.net" {
		t.Errorf("got %q", got)
	}
}

func TestParseIANAReferral_NoReferral(t *testing.T) {
	raw := "% no whois server mentioned\nstatus: ACTIVE\n"
	if got := ParseIANAReferral(raw); got != "" {
		t.Errorf("expected empty, got %q", got)
	}
}

func TestParseIANAReferral_CaseInsensitive(t *testing.T) {
	raw := "WHOIS: WHOIS.example.com\n"
	if got := ParseIANAReferral(raw); got != "WHOIS.example.com" {
		t.Errorf("got %q", got)
	}
}

func TestParseReferral_ARINFormat(t *testing.T) {
	raw := `NetRange:       195.0.0.0 - 195.255.255.255
ReferralServer: whois://whois.ripe.net
`
	got := ParseReferral(raw)
	if got != "whois.ripe.net" {
		t.Errorf("got %q", got)
	}
}

func TestParseReferral_RWhoisStrips(t *testing.T) {
	raw := "ReferralServer: rwhois://rwhois.example.com:4321\n"
	got := ParseReferral(raw)
	// Port 不剥（rwhois 用其他端口）
	if got != "rwhois.example.com:4321" {
		t.Errorf("got %q", got)
	}
}

func TestParseReferral_TrailingSlash(t *testing.T) {
	raw := "ReferralServer: whois://whois.ripe.net/\n"
	got := ParseReferral(raw)
	if got != "whois.ripe.net" {
		t.Errorf("got %q", got)
	}
}
