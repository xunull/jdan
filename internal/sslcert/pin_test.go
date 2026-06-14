package sslcert

import (
	"crypto/ecdsa"
	"crypto/ed25519"
	"crypto/elliptic"
	"crypto/rand"
	"crypto/rsa"
	"crypto/sha256"
	"crypto/x509"
	"crypto/x509/pkix"
	"encoding/base64"
	"math/big"
	"strings"
	"testing"
	"time"
)

// makePinTestCert 生成一个 self-signed cert，返回 *x509.Certificate + 公钥 marshal bytes
// （后者用来独立计算 expected SPKI hash，验证我们 SPKIHash 算法对齐 standard）。
func makePinTestCert(t *testing.T, keyType string) (*x509.Certificate, []byte) {
	t.Helper()
	tmpl := &x509.Certificate{
		SerialNumber: big.NewInt(1),
		Subject:      pkix.Name{CommonName: "pin-test"},
		NotBefore:    time.Now(),
		NotAfter:     time.Now().Add(time.Hour),
	}
	var (
		der    []byte
		err    error
		pubKey interface{}
	)
	switch keyType {
	case "rsa":
		k, e := rsa.GenerateKey(rand.Reader, 2048)
		if e != nil {
			t.Fatal(e)
		}
		pubKey = &k.PublicKey
		der, err = x509.CreateCertificate(rand.Reader, tmpl, tmpl, &k.PublicKey, k)
	case "ec":
		k, e := ecdsa.GenerateKey(elliptic.P256(), rand.Reader)
		if e != nil {
			t.Fatal(e)
		}
		pubKey = &k.PublicKey
		der, err = x509.CreateCertificate(rand.Reader, tmpl, tmpl, &k.PublicKey, k)
	case "ed25519":
		pub, priv, e := ed25519.GenerateKey(rand.Reader)
		if e != nil {
			t.Fatal(e)
		}
		pubKey = pub
		der, err = x509.CreateCertificate(rand.Reader, tmpl, tmpl, pub, priv)
	default:
		t.Fatalf("unknown keyType %q", keyType)
	}
	if err != nil {
		t.Fatal(err)
	}
	cert, err := x509.ParseCertificate(der)
	if err != nil {
		t.Fatal(err)
	}
	// Independent SPKI marshalling using stdlib MarshalPKIXPublicKey → DER bytes
	// 这就是 SubjectPublicKeyInfo 的标准形式；OpenSSL 的 `openssl pkey -pubin -outform DER`
	// 出来的也是这个。所以 sha256 of this == 真正"对齐 standard"的 SPKI hash。
	pkixBytes, err := x509.MarshalPKIXPublicKey(pubKey)
	if err != nil {
		t.Fatal(err)
	}
	return cert, pkixBytes
}

// TestSPKIHash_MatchesIndependentComputation 是核心 alignment 测试：
//
//	openssl x509 -in cert.pem -pubkey -noout |
//	  openssl pkey -pubin -outform DER |
//	  openssl dgst -sha256 -binary | base64
//
// 等价于 sha256(MarshalPKIXPublicKey(pub))。我们的 SPKIHash 用
// cert.RawSubjectPublicKeyInfo（cert 内部的 ASN.1 SPKI 字段，等价表示），
// 两个结果必须 bit 相同。
func TestSPKIHash_MatchesIndependentComputation_RSA(t *testing.T) {
	cert, pkixBytes := makePinTestCert(t, "rsa")
	expected := sha256.Sum256(pkixBytes)
	got := SPKIHash(cert)
	if string(got) != string(expected[:]) {
		t.Errorf("SPKIHash mismatch:\n  got      %x\n  expected %x\n  (SPKI must equal sha256(MarshalPKIXPublicKey(pub)))",
			got, expected)
	}
}

func TestSPKIHash_MatchesIndependentComputation_EC(t *testing.T) {
	cert, pkixBytes := makePinTestCert(t, "ec")
	expected := sha256.Sum256(pkixBytes)
	if string(SPKIHash(cert)) != string(expected[:]) {
		t.Error("EC SPKI hash does not match independent computation")
	}
}

func TestSPKIHash_MatchesIndependentComputation_Ed25519(t *testing.T) {
	cert, pkixBytes := makePinTestCert(t, "ed25519")
	expected := sha256.Sum256(pkixBytes)
	if string(SPKIHash(cert)) != string(expected[:]) {
		t.Error("Ed25519 SPKI hash does not match independent computation")
	}
}

func TestSPKIHash_NotEqualToCertFingerprint(t *testing.T) {
	// 关键防回归：SPKI hash != cert fingerprint。
	// 用户/未来开发者可能想"就用 Describe().SHA256 不就够了"——这测试拦截这种错误。
	cert, _ := makePinTestCert(t, "rsa")
	certFingerprint := sha256.Sum256(cert.Raw)
	spki := SPKIHash(cert)
	if string(spki) == string(certFingerprint[:]) {
		t.Error("SPKI hash should NOT equal cert.Raw fingerprint")
	}
}

func TestSPKIHashBase64_Format(t *testing.T) {
	cert, _ := makePinTestCert(t, "ec")
	got := SPKIHashBase64(cert)
	// base64 of 32 bytes = 44 chars (含 padding '=')
	if len(got) != 44 {
		t.Errorf("base64 length = %d, want 44", len(got))
	}
	// 末尾必须有 padding `=`
	if !strings.HasSuffix(got, "=") {
		t.Errorf("expected base64 standard padding, got %q", got)
	}
	// 不应包含 base64url 专有字符
	if strings.ContainsAny(got, "-_") {
		t.Errorf("base64url chars leaked into standard encoding: %q", got)
	}
	// roundtrip decode
	if _, err := base64.StdEncoding.DecodeString(got); err != nil {
		t.Errorf("invalid base64 standard encoding: %v", err)
	}
}

func TestSPKIHashBase64_NilCert(t *testing.T) {
	if SPKIHashBase64(nil) != "" {
		t.Error("nil cert should give empty string")
	}
	if SPKIHash(nil) != nil {
		t.Error("nil cert should give nil hash")
	}
}

// ── 格式器测试 ────────────────────────────────────────────────────────────

func mkPinEntries(t *testing.T) []PinEntry {
	t.Helper()
	leaf, _ := makePinTestCert(t, "ec")
	inter, _ := makePinTestCert(t, "rsa")
	return []PinEntry{
		{Role: "leaf", SubjectCN: "leaf.example", SPKISha256: SPKIHashBase64(leaf)},
		{Role: "intermediate", SubjectCN: "Inter CA", SPKISha256: SPKIHashBase64(inter)},
	}
}

func TestFormatOkHttp_HasAddCalls(t *testing.T) {
	entries := mkPinEntries(t)
	got := FormatOkHttp("github.com", entries)
	if !strings.Contains(got, "CertificatePinner.Builder()") {
		t.Errorf("missing builder header: %s", got)
	}
	for _, e := range entries {
		want := `.add("github.com", "sha256/` + e.SPKISha256 + `")`
		if !strings.Contains(got, want) {
			t.Errorf("missing %q in output:\n%s", want, got)
		}
	}
}

func TestFormatOkHttp_PlaceholderHostWhenEmpty(t *testing.T) {
	entries := mkPinEntries(t)
	got := FormatOkHttp("", entries)
	if !strings.Contains(got, "your-host.com") {
		t.Errorf("empty host should yield placeholder: %s", got)
	}
}

func TestFormatHPKP_HeaderShape(t *testing.T) {
	entries := mkPinEntries(t)
	got := FormatHPKP("github.com", entries)
	if !strings.HasPrefix(got, "Public-Key-Pins:") {
		t.Errorf("missing HPKP header prefix: %s", got)
	}
	for _, e := range entries {
		want := `pin-sha256="` + e.SPKISha256 + `"`
		if !strings.Contains(got, want) {
			t.Errorf("missing %q:\n%s", want, got)
		}
	}
	if !strings.Contains(got, "max-age=") {
		t.Errorf("missing max-age directive")
	}
	if !strings.Contains(got, "includeSubDomains") {
		t.Errorf("missing includeSubDomains directive")
	}
}

func TestFormatIOS_PlistKeys(t *testing.T) {
	entries := mkPinEntries(t)
	got := FormatIOS("github.com", entries)
	for _, want := range []string{
		"<key>NSAppTransportSecurity</key>",
		"<key>NSPinnedDomains</key>",
		"<key>NSPinnedCAIdentities</key>",
		"<key>SPKI-SHA256-BASE64</key>",
		"github.com",
	} {
		if !strings.Contains(got, want) {
			t.Errorf("iOS plist missing %q", want)
		}
	}
	// 每个 entry 都应在 plist 里出现 hash
	for _, e := range entries {
		if !strings.Contains(got, e.SPKISha256) {
			t.Errorf("iOS plist missing hash %q", e.SPKISha256)
		}
	}
}

func TestFormatNSS_LineFormat(t *testing.T) {
	entries := mkPinEntries(t)
	got := FormatNSS("ignored", entries)
	lines := strings.Split(got, "\n")
	if len(lines) != len(entries) {
		t.Errorf("expected %d lines, got %d", len(entries), len(lines))
	}
	for i, line := range lines {
		want := "pin-sha256:" + entries[i].SPKISha256
		if line != want {
			t.Errorf("line %d: got %q, want %q", i, line, want)
		}
	}
}

func TestFormatCurl_HasFlagAndPins(t *testing.T) {
	entries := mkPinEntries(t)
	got := FormatCurl("github.com", entries)
	if !strings.Contains(got, "--pinnedpubkey") {
		t.Errorf("missing --pinnedpubkey flag")
	}
	for _, e := range entries {
		if !strings.Contains(got, "sha256//"+e.SPKISha256) {
			t.Errorf("missing sha256// pin for entry: %s", got)
		}
	}
	// 多个 pin 用 ; 分隔
	if !strings.Contains(got, ";sha256//") {
		t.Errorf("expected ';sha256//' separator: %s", got)
	}
	if !strings.Contains(got, "https://github.com/") {
		t.Errorf("missing target URL: %s", got)
	}
}

func TestFormatRaw_OneHashPerLine(t *testing.T) {
	entries := mkPinEntries(t)
	got := FormatRaw("ignored", entries)
	lines := strings.Split(got, "\n")
	if len(lines) != len(entries) {
		t.Errorf("raw format should be one line per entry, got %d lines", len(lines))
	}
	for i, line := range lines {
		if line != entries[i].SPKISha256 {
			t.Errorf("line %d: got %q, want %q", i, line, entries[i].SPKISha256)
		}
	}
}

func TestPinFormatters_MapCoverage(t *testing.T) {
	for _, name := range PinFormatNames {
		f, ok := PinFormatters[name]
		if !ok {
			t.Errorf("PinFormatters[%q] missing", name)
			continue
		}
		// 调用一次确保不 panic
		entries := mkPinEntries(t)
		if got := f("test.com", entries); got == "" {
			t.Errorf("formatter %q returned empty", name)
		}
	}
}

func TestEntryFromCert(t *testing.T) {
	cert, _ := makePinTestCert(t, "ec")
	e := EntryFromCert(cert)
	if e.SubjectCN != "pin-test" {
		t.Errorf("SubjectCN: %s", e.SubjectCN)
	}
	if e.SPKISha256 == "" {
		t.Error("SPKI hash should be populated")
	}
	if len(e.SPKISha256) != 44 {
		t.Errorf("SPKI base64 length: %d", len(e.SPKISha256))
	}
}

func TestEntryFromCert_Nil(t *testing.T) {
	if (EntryFromCert(nil) != PinEntry{}) {
		t.Error("nil cert should give zero PinEntry")
	}
}
