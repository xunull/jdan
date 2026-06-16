package certgen

import (
	"crypto/tls"
	"crypto/x509"
	"encoding/pem"
	"net"
	"strings"
	"testing"
	"time"
)

func parseLeaf(t *testing.T, r *Result) *x509.Certificate {
	t.Helper()
	block, _ := pem.Decode(r.CertPEM)
	if block == nil || block.Type != "CERTIFICATE" {
		t.Fatal("cert PEM decode failed")
	}
	c, err := x509.ParseCertificate(block.Bytes)
	if err != nil {
		t.Fatal(err)
	}
	return c
}

// ---- ParseKeyType ----

func TestParseKeyType(t *testing.T) {
	cases := map[string]KeyType{
		"":        KeyEC,
		"ec":      KeyEC,
		"ecdsa":   KeyEC,
		"EC":      KeyEC,
		"rsa":     KeyRSA,
		"RSA":     KeyRSA,
		"ed25519": KeyEd25519,
	}
	for in, want := range cases {
		got, err := ParseKeyType(in)
		if err != nil {
			t.Errorf("ParseKeyType(%q) errored: %v", in, err)
			continue
		}
		if got != want {
			t.Errorf("ParseKeyType(%q) = %v, want %v", in, got, want)
		}
	}
	if _, err := ParseKeyType("dsa"); err == nil {
		t.Error("dsa should be unsupported")
	}
}

// ---- BuildSANs ----

func TestBuildSANs_PrimaryDNS(t *testing.T) {
	s := BuildSANs("localhost", nil, nil)
	if len(s.DNS) != 1 || s.DNS[0] != "localhost" {
		t.Errorf("DNS = %v", s.DNS)
	}
	if len(s.IPs) != 0 {
		t.Errorf("IPs should be empty, got %v", s.IPs)
	}
}

func TestBuildSANs_PrimaryIP(t *testing.T) {
	s := BuildSANs("127.0.0.1", nil, nil)
	if len(s.DNS) != 0 {
		t.Errorf("IP primary should not add DNS, got %v", s.DNS)
	}
	if len(s.IPs) != 1 || !s.IPs[0].Equal(net.ParseIP("127.0.0.1")) {
		t.Errorf("IPs = %v", s.IPs)
	}
}

func TestBuildSANs_ExtraAndDedup(t *testing.T) {
	s := BuildSANs("localhost", []string{"localhost", "*.local", "api.local"}, []string{"127.0.0.1", "127.0.0.1", "::1"})
	// localhost 去重 → 仍只一个；*.local + api.local
	if len(s.DNS) != 3 {
		t.Errorf("DNS dedup wrong: %v", s.DNS)
	}
	if len(s.IPs) != 2 {
		t.Errorf("IP dedup wrong: %v", s.IPs)
	}
}

// ---- GenerateSelfSigned ----

func TestGenerateSelfSigned_Parses(t *testing.T) {
	r, err := GenerateSelfSigned(Options{
		CommonName: "localhost",
		SANs:       BuildSANs("localhost", nil, []string{"127.0.0.1"}),
		Days:       30,
		KeyType:    KeyEC,
	})
	if err != nil {
		t.Fatal(err)
	}
	c := parseLeaf(t, r)
	if c.Subject.CommonName != "localhost" {
		t.Errorf("CN = %q", c.Subject.CommonName)
	}
	if len(c.DNSNames) != 1 || c.DNSNames[0] != "localhost" {
		t.Errorf("DNSNames = %v", c.DNSNames)
	}
	if len(c.IPAddresses) != 1 {
		t.Errorf("IPAddresses = %v", c.IPAddresses)
	}
}

func TestGenerateSelfSigned_AllKeyTypes(t *testing.T) {
	for _, kt := range []KeyType{KeyEC, KeyRSA, KeyEd25519} {
		r, err := GenerateSelfSigned(Options{
			CommonName: "localhost",
			SANs:       BuildSANs("localhost", nil, nil),
			KeyType:    kt,
		})
		if err != nil {
			t.Errorf("key type %s errored: %v", kt, err)
			continue
		}
		// cert + key 应当能组成 tls.Certificate（私钥配对验证）
		if _, err := tls.X509KeyPair(r.CertPEM, r.KeyPEM); err != nil {
			t.Errorf("key type %s: keypair mismatch: %v", kt, err)
		}
	}
}

func TestGenerateSelfSigned_Validity(t *testing.T) {
	fixed := time.Date(2026, 1, 1, 0, 0, 0, 0, time.UTC)
	r, err := GenerateSelfSigned(Options{
		CommonName: "x",
		SANs:       BuildSANs("x", nil, nil),
		Days:       100,
		KeyType:    KeyEC,
		now:        func() time.Time { return fixed },
	})
	if err != nil {
		t.Fatal(err)
	}
	c := parseLeaf(t, r)
	// NotAfter = now + 100 days
	wantAfter := fixed.AddDate(0, 0, 100)
	if !c.NotAfter.Equal(wantAfter) {
		t.Errorf("NotAfter = %v, want %v", c.NotAfter, wantAfter)
	}
	// NotBefore 略早于 now（防时钟漂移）
	if !c.NotBefore.Before(fixed) {
		t.Errorf("NotBefore = %v, should be before %v", c.NotBefore, fixed)
	}
}

func TestGenerateSelfSigned_DefaultDays(t *testing.T) {
	r, _ := GenerateSelfSigned(Options{CommonName: "x", SANs: BuildSANs("x", nil, nil), KeyType: KeyEC})
	c := parseLeaf(t, r)
	span := c.NotAfter.Sub(c.NotBefore)
	// 默认 825 天（含 1h 提前量）
	if span < 824*24*time.Hour || span > 826*24*time.Hour {
		t.Errorf("default validity span = %v, want ~825 days", span)
	}
}

func TestGenerateSelfSigned_HasServerAuth(t *testing.T) {
	r, _ := GenerateSelfSigned(Options{CommonName: "x", SANs: BuildSANs("x", nil, nil), KeyType: KeyEC})
	c := parseLeaf(t, r)
	found := false
	for _, eku := range c.ExtKeyUsage {
		if eku == x509.ExtKeyUsageServerAuth {
			found = true
		}
	}
	if !found {
		t.Error("leaf should have ServerAuth EKU")
	}
}

// ---- CA ----

func TestGenerateCA_IsCA(t *testing.T) {
	ca, err := GenerateCA(Options{CommonName: "test", KeyType: KeyEC})
	if err != nil {
		t.Fatal(err)
	}
	block, _ := pem.Decode(ca.CertPEM)
	c, _ := x509.ParseCertificate(block.Bytes)
	if !c.IsCA {
		t.Error("CA cert should have IsCA=true")
	}
}

func TestCA_SignLeaf_VerifiesAgainstCA(t *testing.T) {
	ca, err := GenerateCA(Options{CommonName: "test", KeyType: KeyEC})
	if err != nil {
		t.Fatal(err)
	}
	leaf, err := ca.SignLeaf(Options{
		CommonName: "localhost",
		SANs:       BuildSANs("localhost", nil, []string{"127.0.0.1"}),
		KeyType:    KeyEC,
	})
	if err != nil {
		t.Fatal(err)
	}
	// leaf 应当能被 CA 验证
	caBlock, _ := pem.Decode(ca.CertPEM)
	caCert, _ := x509.ParseCertificate(caBlock.Bytes)
	leafCert := parseLeaf(t, leaf)

	roots := x509.NewCertPool()
	roots.AddCert(caCert)
	if _, err := leafCert.Verify(x509.VerifyOptions{
		Roots:   roots,
		DNSName: "localhost",
	}); err != nil {
		t.Errorf("leaf should verify against CA: %v", err)
	}
}

// ---- 端到端：用生成的 cert 起 TLS server ----

func TestEndToEnd_TLSHandshake(t *testing.T) {
	r, err := GenerateSelfSigned(Options{
		CommonName: "localhost",
		SANs:       BuildSANs("localhost", nil, []string{"127.0.0.1"}),
		KeyType:    KeyEC,
	})
	if err != nil {
		t.Fatal(err)
	}
	keypair, err := tls.X509KeyPair(r.CertPEM, r.KeyPEM)
	if err != nil {
		t.Fatal(err)
	}
	ln, err := tls.Listen("tcp", "127.0.0.1:0", &tls.Config{Certificates: []tls.Certificate{keypair}})
	if err != nil {
		t.Fatal(err)
	}
	defer ln.Close()
	go func() {
		conn, err := ln.Accept()
		if err != nil {
			return
		}
		// TLS handshake 是懒触发的（首次 Read/Write）；显式跑一次让 client 不拿 EOF
		if tc, ok := conn.(*tls.Conn); ok {
			_ = tc.Handshake()
		}
		conn.Close()
	}()

	// client 把生成的 cert 当 root，连 127.0.0.1（cert 有该 IP SAN）
	roots := x509.NewCertPool()
	if !roots.AppendCertsFromPEM(r.CertPEM) {
		t.Fatal("failed to add cert to roots")
	}
	_, port, _ := net.SplitHostPort(ln.Addr().String())
	conn, err := tls.Dial("tcp", "127.0.0.1:"+port, &tls.Config{
		RootCAs:    roots,
		ServerName: "localhost",
	})
	if err != nil {
		t.Fatalf("TLS handshake with generated cert failed: %v", err)
	}
	conn.Close()
}

// ---- FingerprintSHA256 / SANString ----

func TestFingerprintSHA256(t *testing.T) {
	r, _ := GenerateSelfSigned(Options{CommonName: "x", SANs: BuildSANs("x", nil, nil), KeyType: KeyEC})
	c := parseLeaf(t, r)
	fp := FingerprintSHA256(c)
	if !strings.HasPrefix(fp, "SHA256:") {
		t.Errorf("fingerprint format: %q", fp)
	}
}

func TestSANString(t *testing.T) {
	r, _ := GenerateSelfSigned(Options{
		CommonName: "x",
		SANs:       BuildSANs("localhost", nil, []string{"127.0.0.1"}),
		KeyType:    KeyEC,
	})
	c := parseLeaf(t, r)
	got := SANString(c)
	if !strings.Contains(got, "DNS:localhost") || !strings.Contains(got, "IP:127.0.0.1") {
		t.Errorf("SANString = %q", got)
	}
}
