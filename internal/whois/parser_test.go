package whois

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"
)

func readFixture(t *testing.T, name string) string {
	t.Helper()
	b, err := os.ReadFile(filepath.Join("testdata", name))
	if err != nil {
		t.Fatal(err)
	}
	return string(b)
}

func TestParseDomain_VerisignCom(t *testing.T) {
	raw := readFixture(t, "verisign-com.txt")
	p := ParseDomain(raw)

	if p.DomainName != "EXAMPLE.COM" {
		t.Errorf("DomainName = %q", p.DomainName)
	}
	if p.Registrar != "RESERVED-Internet Assigned Numbers Authority" {
		t.Errorf("Registrar = %q", p.Registrar)
	}
	wantCreate := time.Date(1995, 8, 14, 4, 0, 0, 0, time.UTC)
	if !p.CreationDate.Equal(wantCreate) {
		t.Errorf("CreationDate = %v, want %v", p.CreationDate, wantCreate)
	}
	wantExpiry := time.Date(2026, 8, 13, 4, 0, 0, 0, time.UTC)
	if !p.ExpiryDate.Equal(wantExpiry) {
		t.Errorf("ExpiryDate = %v, want %v", p.ExpiryDate, wantExpiry)
	}
	if p.RegistryDomainID != "2336799_DOMAIN_COM-VRSN" {
		t.Errorf("RegistryDomainID = %q", p.RegistryDomainID)
	}
	if p.DNSSEC != "signedDelegation" {
		t.Errorf("DNSSEC = %q", p.DNSSEC)
	}
	wantStatus := []string{
		"clientDeleteProhibited",
		"clientTransferProhibited",
		"clientUpdateProhibited",
	}
	if len(p.Status) != 3 {
		t.Fatalf("Status len = %d, want 3", len(p.Status))
	}
	for i, w := range wantStatus {
		if p.Status[i] != w {
			t.Errorf("Status[%d] = %q, want %q", i, p.Status[i], w)
		}
	}
	if len(p.Nameservers) != 2 {
		t.Fatalf("Nameservers len = %d, want 2", len(p.Nameservers))
	}
	// Nameservers 应当 lowercase
	if p.Nameservers[0] != "elliott.ns.cloudflare.com" {
		t.Errorf("Nameservers[0] = %q", p.Nameservers[0])
	}
}

func TestParseDomain_StatusURLStripped(t *testing.T) {
	// Verisign status 行尾的 URL 不该出现在 Status 字段
	raw := readFixture(t, "verisign-com.txt")
	p := ParseDomain(raw)
	for _, s := range p.Status {
		if strings.Contains(s, "icann.org") || strings.Contains(s, "http") {
			t.Errorf("status URL not stripped: %q", s)
		}
	}
}

func TestParseDomain_DENIC(t *testing.T) {
	// DENIC 用 "changed:" 而不是 "updated date:"；schema 简化版
	raw := readFixture(t, "denic-de.txt")
	p := ParseDomain(raw)

	if p.DomainName != "example.de" {
		t.Errorf("DomainName = %q", p.DomainName)
	}
	if len(p.Nameservers) != 2 {
		t.Errorf("Nameservers = %v", p.Nameservers)
	}
	if len(p.Status) != 1 || p.Status[0] != "connect" {
		t.Errorf("Status = %v", p.Status)
	}
}

func TestParseIP_ARIN(t *testing.T) {
	raw := readFixture(t, "arin-ipv4.txt")
	p := ParseIP(raw)

	if p.NetRange != "8.8.8.0 - 8.8.8.255" {
		t.Errorf("NetRange = %q", p.NetRange)
	}
	if p.NetName != "LVLT-GOGL-8-8-8" {
		t.Errorf("NetName = %q", p.NetName)
	}
	if p.OrgName != "Google LLC" {
		t.Errorf("OrgName = %q", p.OrgName)
	}
	if p.Country != "US" {
		t.Errorf("Country = %q", p.Country)
	}
	if p.AbuseEmail != "network-abuse@google.com" {
		t.Errorf("AbuseEmail = %q", p.AbuseEmail)
	}
}

func TestParseIP_RIPE(t *testing.T) {
	raw := readFixture(t, "ripe-ipv4.txt")
	p := ParseIP(raw)

	if p.NetRange != "193.0.0.0 - 193.0.7.255" {
		t.Errorf("NetRange = %q (RIPE uses 'inetnum:')", p.NetRange)
	}
	if p.NetName != "RIPE-NCC" {
		t.Errorf("NetName = %q", p.NetName)
	}
	// RIPE 用 "org-name:" 不是 "OrgName:"
	if p.OrgName != "Reseaux IP Europeens Network Coordination Centre (RIPE NCC)" {
		t.Errorf("OrgName = %q", p.OrgName)
	}
	if p.Country != "NL" {
		t.Errorf("Country = %q", p.Country)
	}
	if p.AbuseEmail != "abuse@ripe.net" {
		t.Errorf("AbuseEmail = %q (RIPE uses abuse-mailbox)", p.AbuseEmail)
	}
}

func TestParseWhoisTime_RFC3339(t *testing.T) {
	got := parseWhoisTime("2026-08-13T04:00:00Z")
	want := time.Date(2026, 8, 13, 4, 0, 0, 0, time.UTC)
	if !got.Equal(want) {
		t.Errorf("got %v, want %v", got, want)
	}
}

func TestParseWhoisTime_SimpleDate(t *testing.T) {
	got := parseWhoisTime("2014-03-14")
	want := time.Date(2014, 3, 14, 0, 0, 0, 0, time.UTC)
	if !got.Equal(want) {
		t.Errorf("got %v, want %v", got, want)
	}
}

func TestParseWhoisTime_DDMonYYYY(t *testing.T) {
	got := parseWhoisTime("13-Aug-2026")
	want := time.Date(2026, 8, 13, 0, 0, 0, 0, time.UTC)
	if !got.Equal(want) {
		t.Errorf("got %v, want %v", got, want)
	}
}

func TestParseWhoisTime_TrailingGarbage(t *testing.T) {
	// "2026-08-13 (UTC)" - 第一个 token 取出后再解析
	got := parseWhoisTime("2026-08-13 (UTC)")
	want := time.Date(2026, 8, 13, 0, 0, 0, 0, time.UTC)
	if !got.Equal(want) {
		t.Errorf("got %v, want %v", got, want)
	}
}

func TestParseWhoisTime_Unknown_ReturnsZero(t *testing.T) {
	got := parseWhoisTime("not a date")
	if !got.IsZero() {
		t.Errorf("expected zero, got %v", got)
	}
}

func TestScan_SkipsCommentsAndEmpty(t *testing.T) {
	raw := `% comment line
# another comment

key: value
empty:
real_key: real_value
`
	kv := scan(raw)
	if v, ok := kv["key"]; !ok || v[0] != "value" {
		t.Errorf("key missing: %v", kv)
	}
	if _, ok := kv["empty"]; ok {
		t.Error("empty value should not be recorded")
	}
	if v := kv["real_key"]; len(v) != 1 || v[0] != "real_value" {
		t.Errorf("real_key = %v", v)
	}
}

func TestScan_MultiValue(t *testing.T) {
	raw := `status: a
status: b
status: c
`
	kv := scan(raw)
	if len(kv["status"]) != 3 {
		t.Errorf("status = %v", kv["status"])
	}
}

func TestParsed_IsEmpty(t *testing.T) {
	if !(&Parsed{}).IsEmpty() {
		t.Error("zero Parsed should be empty")
	}
	if (&Parsed{DomainName: "x"}).IsEmpty() {
		t.Error("Parsed with DomainName should not be empty")
	}
	if (&Parsed{NetRange: "1.0.0.0/24"}).IsEmpty() {
		t.Error("Parsed with NetRange should not be empty")
	}
}

func TestParseDomain_UnknownSchema_ReturnsEmpty(t *testing.T) {
	// 完全不认识的 schema：parser 返回 empty 而不是 nil
	raw := `Random garbage that has no key value pairs
or anything we recognize
maybe a number: 42
`
	p := ParseDomain(raw)
	if p == nil {
		t.Fatal("ParseDomain should return non-nil even on unknown schema")
	}
	// "maybe a number" 含有 colon，会被 scan 提取为 key="maybe a number" val="42"。
	// 但 domain aliases 不命中，所以所有命名字段都是零值。
	if p.DomainName != "" || p.Registrar != "" {
		t.Errorf("unknown schema leaked into known fields: %+v", p)
	}
}
