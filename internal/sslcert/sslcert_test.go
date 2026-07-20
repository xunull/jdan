package sslcert

import (
	"context"
	"crypto/ecdsa"
	"crypto/elliptic"
	"crypto/rand"
	"crypto/rsa"
	"crypto/tls"
	"crypto/x509"
	"crypto/x509/pkix"
	"encoding/pem"
	"math/big"
	"net"
	"net/http"
	"net/http/httptest"
	"net/url"
	"os"
	"path/filepath"
	"strconv"
	"strings"
	"testing"
	"time"
)

// ─── 测试 helper: 生成 self-signed cert ──────────────────────────────────

type genCertOpts struct {
	CommonName string
	DNSNames   []string
	NotBefore  time.Time
	NotAfter   time.Time
	IsCA       bool
	EC         bool // true → EC; false → RSA 2048
	OCSPServer []string
}

func genCert(t *testing.T, opts genCertOpts) (*x509.Certificate, interface{}) {
	t.Helper()

	if opts.NotBefore.IsZero() {
		opts.NotBefore = time.Now().Add(-1 * time.Hour)
	}
	if opts.NotAfter.IsZero() {
		opts.NotAfter = time.Now().Add(90 * 24 * time.Hour)
	}
	tmpl := &x509.Certificate{
		SerialNumber: big.NewInt(time.Now().UnixNano()),
		Subject:      pkix.Name{CommonName: opts.CommonName, Organization: []string{"test-org"}},
		NotBefore:    opts.NotBefore,
		NotAfter:     opts.NotAfter,
		KeyUsage:     x509.KeyUsageDigitalSignature | x509.KeyUsageKeyEncipherment,
		ExtKeyUsage:  []x509.ExtKeyUsage{x509.ExtKeyUsageServerAuth},
		IsCA:         opts.IsCA,
		DNSNames:     opts.DNSNames,
		OCSPServer:   opts.OCSPServer,
	}
	if opts.IsCA {
		tmpl.KeyUsage |= x509.KeyUsageCertSign
		tmpl.BasicConstraintsValid = true
	}

	var (
		der  []byte
		priv interface{}
		err  error
	)
	if opts.EC {
		ec, e := ecdsa.GenerateKey(elliptic.P256(), rand.Reader)
		if e != nil {
			t.Fatal(e)
		}
		der, err = x509.CreateCertificate(rand.Reader, tmpl, tmpl, &ec.PublicKey, ec)
		priv = ec
	} else {
		rsaK, e := rsa.GenerateKey(rand.Reader, 2048)
		if e != nil {
			t.Fatal(e)
		}
		der, err = x509.CreateCertificate(rand.Reader, tmpl, tmpl, &rsaK.PublicKey, rsaK)
		priv = rsaK
	}
	if err != nil {
		t.Fatal(err)
	}
	cert, err := x509.ParseCertificate(der)
	if err != nil {
		t.Fatal(err)
	}
	return cert, priv
}

func writePEMFile(t *testing.T, certs ...*x509.Certificate) string {
	t.Helper()
	dir := t.TempDir()
	path := filepath.Join(dir, "cert.pem")
	var buf []byte
	for _, c := range certs {
		buf = append(buf, pem.EncodeToMemory(&pem.Block{
			Type:  "CERTIFICATE",
			Bytes: c.Raw,
		})...)
	}
	if err := os.WriteFile(path, buf, 0o644); err != nil {
		t.Fatal(err)
	}
	return path
}

// ─── tests ───────────────────────────────────────────────────────────────

func TestParseTarget_Variants(t *testing.T) {
	for _, tc := range []struct {
		in        string
		wantHost  string
		wantPort  int
		expectErr bool
	}{
		{"example.com", "example.com", 443, false},
		{"example.com:443", "example.com", 443, false},
		{"example.com:8443", "example.com", 8443, false},
		{"https://example.com", "example.com", 443, false},
		{"https://example.com:8443/path", "example.com", 8443, false},
		{"https://example.com/", "example.com", 443, false},
		{"", "", 0, true},
		{"http://example.com", "", 0, true}, // 拒绝 http
		{"example.com:abc", "", 0, true},
	} {
		host, port, err := ParseTarget(tc.in)
		if tc.expectErr {
			if err == nil {
				t.Errorf("ParseTarget(%q) should error", tc.in)
			}
			continue
		}
		if err != nil {
			t.Errorf("ParseTarget(%q) unexpected error: %v", tc.in, err)
			continue
		}
		if host != tc.wantHost || port != tc.wantPort {
			t.Errorf("ParseTarget(%q) = %s:%d, want %s:%d",
				tc.in, host, port, tc.wantHost, tc.wantPort)
		}
	}
}

func TestParsePEMFile_RoundTrip(t *testing.T) {
	cert, _ := genCert(t, genCertOpts{CommonName: "test.example", DNSNames: []string{"test.example"}})
	path := writePEMFile(t, cert)

	b, err := ParsePEMFile(path)
	if err != nil {
		t.Fatal(err)
	}
	if len(b.Chain) != 1 {
		t.Errorf("chain len %d, want 1", len(b.Chain))
	}
	if b.Leaf().Subject.CommonName != "test.example" {
		t.Errorf("CN: %s", b.Leaf().Subject.CommonName)
	}
	if !strings.HasPrefix(b.Source, "file:") {
		t.Errorf("source should start with file:, got %q", b.Source)
	}
}

func TestParsePEMFile_MultiCert(t *testing.T) {
	leaf, _ := genCert(t, genCertOpts{CommonName: "leaf"})
	inter, _ := genCert(t, genCertOpts{CommonName: "intermediate", IsCA: true})
	path := writePEMFile(t, leaf, inter)

	b, err := ParsePEMFile(path)
	if err != nil {
		t.Fatal(err)
	}
	if len(b.Chain) != 2 {
		t.Errorf("chain len %d, want 2", len(b.Chain))
	}
}

func TestParsePEMBytes_NoCertError(t *testing.T) {
	_, err := ParsePEMBytes([]byte("not a PEM"), "test")
	if err == nil {
		t.Error("no CERTIFICATE block should error")
	}
}

func TestEncodePEM_RoundTrip(t *testing.T) {
	cert, _ := genCert(t, genCertOpts{CommonName: "round"})
	pemOut := EncodePEM([]*x509.Certificate{cert})
	if !strings.Contains(pemOut, "-----BEGIN CERTIFICATE-----") {
		t.Error("missing PEM header")
	}
	// Re-parse and verify CN
	b, err := ParsePEMBytes([]byte(pemOut), "round")
	if err != nil {
		t.Fatal(err)
	}
	if b.Leaf().Subject.CommonName != "round" {
		t.Errorf("CN lost in round-trip: %s", b.Leaf().Subject.CommonName)
	}
}

func TestDescribe_RSACert(t *testing.T) {
	cert, _ := genCert(t, genCertOpts{
		CommonName: "describe.test",
		DNSNames:   []string{"describe.test", "www.describe.test"},
	})
	s := Describe(cert)
	if s.SubjectCN != "describe.test" {
		t.Errorf("SubjectCN: %s", s.SubjectCN)
	}
	if len(s.SAN) != 2 {
		t.Errorf("SAN count: %d", len(s.SAN))
	}
	if !strings.HasPrefix(s.KeyAlgorithm, "RSA") {
		t.Errorf("KeyAlgorithm: %s", s.KeyAlgorithm)
	}
	if s.SHA256 == "" || !strings.Contains(s.SHA256, ":") {
		t.Errorf("SHA256 missing colon-format: %q", s.SHA256)
	}
	if s.DaysLeft <= 0 || s.DaysLeft > 91 {
		t.Errorf("DaysLeft out of expected range: %d", s.DaysLeft)
	}
	if s.Expired {
		t.Error("freshly-generated cert should not be expired")
	}
}

func TestDescribe_ECCert(t *testing.T) {
	cert, _ := genCert(t, genCertOpts{CommonName: "ec.test", EC: true})
	s := Describe(cert)
	if !strings.HasPrefix(s.KeyAlgorithm, "EC") {
		t.Errorf("EC KeyAlgorithm: %s", s.KeyAlgorithm)
	}
}

func TestDescribe_ExpiredCert(t *testing.T) {
	cert, _ := genCert(t, genCertOpts{
		CommonName: "expired.test",
		NotBefore:  time.Now().Add(-48 * time.Hour),
		NotAfter:   time.Now().Add(-1 * time.Hour),
	})
	s := Describe(cert)
	if !s.Expired {
		t.Error("past-NotAfter cert should be Expired")
	}
}

func TestShortName_CN(t *testing.T) {
	cert, _ := genCert(t, genCertOpts{CommonName: "abc"})
	if got := ShortName(cert); got != "CN=abc" {
		t.Errorf("got %s", got)
	}
}

func TestVerify_SelfSignedNotTrusted(t *testing.T) {
	cert, _ := genCert(t, genCertOpts{CommonName: "untrusted", DNSNames: []string{"untrusted"}})
	b := &Bundle{Chain: []*x509.Certificate{cert}, Source: "test"}

	r := Verify(b, "untrusted")
	if r.Trusted {
		t.Error("self-signed should NOT be trusted by system store")
	}
	if r.TrustErr == "" {
		t.Error("trust err should be populated")
	}
}

func TestVerify_HostnameMismatch(t *testing.T) {
	cert, _ := genCert(t, genCertOpts{CommonName: "host-a", DNSNames: []string{"host-a"}})
	b := &Bundle{Chain: []*x509.Certificate{cert}, Source: "test"}

	r := Verify(b, "different-host")
	if r.HostnameOK {
		t.Error("different hostname should fail VerifyHostname")
	}
	if r.HostnameErr == "" {
		t.Error("hostname err should be populated")
	}
}

func TestVerify_EmptyHostnameSkipsHostnameCheck(t *testing.T) {
	cert, _ := genCert(t, genCertOpts{CommonName: "x", DNSNames: []string{"x"}})
	b := &Bundle{Chain: []*x509.Certificate{cert}, Source: "test"}

	r := Verify(b, "")
	if !r.HostnameSkipped {
		t.Error("empty hostname should set HostnameSkipped=true")
	}
}

func TestVerify_ExpiredCert(t *testing.T) {
	cert, _ := genCert(t, genCertOpts{
		CommonName: "expired",
		NotBefore:  time.Now().Add(-48 * time.Hour),
		NotAfter:   time.Now().Add(-1 * time.Hour),
	})
	b := &Bundle{Chain: []*x509.Certificate{cert}}

	r := Verify(b, "")
	if !r.Expired {
		t.Error("past-NotAfter cert should be Expired")
	}
}

func TestFetchFromHost_LocalTLSServer(t *testing.T) {
	ts := httptest.NewTLSServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(200)
	}))
	defer ts.Close()

	u, _ := url.Parse(ts.URL)
	host, portStr, _ := net.SplitHostPort(u.Host)
	port, _ := strconv.Atoi(portStr)

	b, err := FetchFromHost(context.Background(), FetchOptions{
		Host: host, Port: port, Timeout: 3 * time.Second,
	})
	if err != nil {
		t.Fatal(err)
	}
	if b.Leaf() == nil {
		t.Fatal("no leaf")
	}
}

func TestFetchFromHost_BadHost(t *testing.T) {
	_, err := FetchFromHost(context.Background(), FetchOptions{
		Host: "127.0.0.1", Port: 1, Timeout: 200 * time.Millisecond,
	})
	if err == nil {
		t.Error("port 1 should error (no listener)")
	}
}

func TestFetchFromHost_NoHost(t *testing.T) {
	_, err := FetchFromHost(context.Background(), FetchOptions{})
	if err == nil {
		t.Error("missing host should error")
	}
}

func TestCheckOCSP_NoResponderURL(t *testing.T) {
	cert, _ := genCert(t, genCertOpts{CommonName: "no-ocsp"})
	issuer, _ := genCert(t, genCertOpts{CommonName: "issuer", IsCA: true})

	status := CheckOCSP(context.Background(), cert, issuer)
	if status.Available {
		t.Error("cert without OCSPServer should not be Available")
	}
	if status.Checked {
		t.Error("not Available should mean not Checked")
	}
}

func TestCheckOCSP_HTTPMockGood(t *testing.T) {
	// 这个测试目的是验证 OCSP 路径"能跑通"：用一个真假 mock OCSP responder。
	// 真实 OCSP 响应需要正确 sign，写起来很复杂；这里仅验证 cert 没 OCSPServer
	// 时的 fast-path + 有 OCSPServer 时会真去发请求（HTTP 500 → error path）。

	mockSrv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(500)
		_, _ = w.Write([]byte("intentional fail"))
	}))
	defer mockSrv.Close()

	cert, _ := genCert(t, genCertOpts{CommonName: "x", OCSPServer: []string{mockSrv.URL}})
	issuer, _ := genCert(t, genCertOpts{CommonName: "issuer", IsCA: true})

	status := CheckOCSP(context.Background(), cert, issuer)
	if !status.Available {
		t.Error("OCSPServer set should give Available=true")
	}
	if status.Checked {
		t.Error("500 response should not yield Checked=true")
	}
	if status.Err == "" {
		t.Error("500 should populate Err")
	}
}

func TestBundle_FullChain_DedupesRoot(t *testing.T) {
	leaf, _ := genCert(t, genCertOpts{CommonName: "leaf"})
	root, _ := genCert(t, genCertOpts{CommonName: "root", IsCA: true})
	// server already sent root
	b := &Bundle{
		Chain:          []*x509.Certificate{leaf, root},
		VerifiedChains: [][]*x509.Certificate{{leaf, root}},
	}
	full := b.FullChain()
	if len(full) != 2 {
		t.Errorf("FullChain should dedupe to 2, got %d", len(full))
	}
}

func TestNow_Override(t *testing.T) {
	orig := Now
	defer func() { Now = orig }()
	fixed := time.Date(2026, 6, 15, 12, 0, 0, 0, time.UTC)
	Now = func() time.Time { return fixed }

	cert, _ := genCert(t, genCertOpts{
		CommonName: "fixed-time",
		NotBefore:  fixed.Add(-24 * time.Hour),
		NotAfter:   fixed.Add(10 * 24 * time.Hour),
	})
	s := Describe(cert)
	if s.DaysLeft != 10 {
		t.Errorf("DaysLeft = %d, want 10 (with frozen Now)", s.DaysLeft)
	}
}

// 验证 TLS 自签 cert 通过 httptest server 取回后，verify 能正确报告"不可信"
func TestVerify_TLSServerSelfSigned(t *testing.T) {
	ts := httptest.NewTLSServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {}))
	defer ts.Close()
	u, _ := url.Parse(ts.URL)
	host, portStr, _ := net.SplitHostPort(u.Host)
	port, _ := strconv.Atoi(portStr)

	b, err := FetchFromHost(context.Background(), FetchOptions{
		Host: host, Port: port, Timeout: 2 * time.Second,
	})
	if err != nil {
		t.Fatal(err)
	}
	r := Verify(b, host)
	if r.Trusted {
		t.Error("httptest server self-signed cert should NOT be trusted")
	}
}

// 验证使用 InsecureSkipVerify 的 fetch 真把所有 cert 拿回来了
func TestFetchFromHost_ReturnsServerCert(t *testing.T) {
	// 设置一个 TLS server with config 强制 server name
	cert, key := genCert(t, genCertOpts{CommonName: "fetch-test.local", DNSNames: []string{"fetch-test.local"}})
	rsaKey, ok := key.(*rsa.PrivateKey)
	if !ok {
		t.Fatal("key not RSA")
	}
	srvCert := tls.Certificate{
		Certificate: [][]byte{cert.Raw},
		PrivateKey:  rsaKey,
		Leaf:        cert,
	}
	cfg := &tls.Config{Certificates: []tls.Certificate{srvCert}}

	listener, err := tls.Listen("tcp", "127.0.0.1:0", cfg)
	if err != nil {
		t.Fatal(err)
	}
	defer listener.Close()
	go func() {
		for {
			c, err := listener.Accept()
			if err != nil {
				return
			}
			// 等 client 完成 TLS 握手再 Close（accept 完不能立刻关）
			if tlsC, ok := c.(*tls.Conn); ok {
				_ = tlsC.Handshake()
			}
			_ = c.Close()
		}
	}()

	port := listener.Addr().(*net.TCPAddr).Port
	b, err := FetchFromHost(context.Background(), FetchOptions{
		Host: "127.0.0.1", Port: port, SNI: "fetch-test.local", Timeout: 2 * time.Second,
	})
	if err != nil {
		t.Fatal(err)
	}
	if b.Leaf().Subject.CommonName != "fetch-test.local" {
		t.Errorf("CN: %s", b.Leaf().Subject.CommonName)
	}
}
