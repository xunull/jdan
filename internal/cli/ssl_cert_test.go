package cli

import (
	"bytes"
	"crypto/rand"
	"crypto/rsa"
	"crypto/x509"
	"crypto/x509/pkix"
	"encoding/json"
	"encoding/pem"
	"math/big"
	"net/http"
	"net/http/httptest"
	"net/url"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"github.com/xunull/jdan/internal/sslcert"
)

// genSelfSignedPEM 生成一个 self-signed RSA cert PEM 文件，给 -f 测试用。
func genSelfSignedPEM(t *testing.T, opts genCertCLIOpts) string {
	t.Helper()
	if opts.NotBefore.IsZero() {
		opts.NotBefore = time.Now().Add(-1 * time.Hour)
	}
	if opts.NotAfter.IsZero() {
		opts.NotAfter = time.Now().Add(90 * 24 * time.Hour)
	}
	key, err := rsa.GenerateKey(rand.Reader, 2048)
	if err != nil {
		t.Fatal(err)
	}
	tmpl := &x509.Certificate{
		SerialNumber: big.NewInt(time.Now().UnixNano()),
		Subject:      pkix.Name{CommonName: opts.CN},
		NotBefore:    opts.NotBefore,
		NotAfter:     opts.NotAfter,
		KeyUsage:     x509.KeyUsageDigitalSignature,
		ExtKeyUsage:  []x509.ExtKeyUsage{x509.ExtKeyUsageServerAuth},
		DNSNames:     opts.DNSNames,
	}
	der, err := x509.CreateCertificate(rand.Reader, tmpl, tmpl, &key.PublicKey, key)
	if err != nil {
		t.Fatal(err)
	}
	dir := t.TempDir()
	path := filepath.Join(dir, "cert.pem")
	out := pem.EncodeToMemory(&pem.Block{Type: "CERTIFICATE", Bytes: der})
	if err := os.WriteFile(path, out, 0o644); err != nil {
		t.Fatal(err)
	}
	return path
}

type genCertCLIOpts struct {
	CN        string
	DNSNames  []string
	NotBefore time.Time
	NotAfter  time.Time
}

func TestSSLCert_FilePEM_RenderText(t *testing.T) {
	path := genSelfSignedPEM(t, genCertCLIOpts{CN: "filetest.example", DNSNames: []string{"filetest.example"}})

	var buf bytes.Buffer
	cmd := newSSLCertCommand(sslCertCmdDeps{out: &buf})
	cmd.SetArgs([]string{"-f", path, "--no-ocsp"})
	if err := cmd.Execute(); err != nil {
		t.Fatal(err)
	}
	out := buf.String()
	for _, want := range []string{
		"leaf", "Subject:", "filetest.example", "SHA256:", "Verification:",
	} {
		if !strings.Contains(out, want) {
			t.Errorf("output missing %q:\n%s", want, out)
		}
	}
	if strings.Contains(out, "OCSP:") {
		t.Errorf("--no-ocsp + file mode should not emit OCSP section")
	}
}

func TestSSLCert_JSON(t *testing.T) {
	path := genSelfSignedPEM(t, genCertCLIOpts{CN: "json.example", DNSNames: []string{"json.example"}})

	var buf bytes.Buffer
	cmd := newSSLCertCommand(sslCertCmdDeps{out: &buf})
	cmd.SetArgs([]string{"-f", path, "--json", "--no-ocsp"})
	if err := cmd.Execute(); err != nil {
		t.Fatal(err)
	}

	var payload map[string]interface{}
	if err := json.Unmarshal(buf.Bytes(), &payload); err != nil {
		t.Fatalf("not valid JSON: %v\n%s", err, buf.String())
	}
	leaf := payload["leaf"].(map[string]interface{})
	if leaf["subject_cn"] != "json.example" {
		t.Errorf("subject_cn = %v", leaf["subject_cn"])
	}
	if payload["verification"] == nil {
		t.Error("missing verification section")
	}
}

func TestSSLCert_PEMOutput(t *testing.T) {
	path := genSelfSignedPEM(t, genCertCLIOpts{CN: "pem.example"})

	var buf bytes.Buffer
	cmd := newSSLCertCommand(sslCertCmdDeps{out: &buf})
	cmd.SetArgs([]string{"-f", path, "--pem", "--no-ocsp"})
	if err := cmd.Execute(); err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(buf.String(), "-----BEGIN CERTIFICATE-----") {
		t.Errorf("--pem should output PEM headers")
	}
}

func TestSSLCert_ExpiresIn_TooSoonExits1(t *testing.T) {
	// cert 在 5d 后过期，threshold 30d → 应该 fail
	path := genSelfSignedPEM(t, genCertCLIOpts{
		CN: "soon.example", NotAfter: time.Now().Add(5 * 24 * time.Hour),
	})
	cmd := newSSLCertCommand(sslCertCmdDeps{out: &bytes.Buffer{}})
	cmd.SetArgs([]string{"-f", path, "--no-ocsp", "--expires-in", "30d"})
	err := cmd.Execute()
	if err == nil {
		t.Error("expires-in 30d on a 5d cert should error (exit 1)")
	}
	if _, ok := err.(*sslCertExitErr); !ok {
		t.Errorf("expected sslCertExitErr, got %T: %v", err, err)
	}
}

func TestSSLCert_ExpiresIn_PlentyOfTime(t *testing.T) {
	// cert 在 60d 后过期，threshold 30d → 应该不 error
	path := genSelfSignedPEM(t, genCertCLIOpts{
		CN: "fine.example", NotAfter: time.Now().Add(60 * 24 * time.Hour),
	})
	cmd := newSSLCertCommand(sslCertCmdDeps{out: &bytes.Buffer{}})
	cmd.SetArgs([]string{"-f", path, "--no-ocsp", "--expires-in", "30d"})
	if err := cmd.Execute(); err != nil {
		t.Errorf("60d cert should not trigger expires-in 30d: %v", err)
	}
}

func TestSSLCert_ExpiresIn_InvalidDuration(t *testing.T) {
	path := genSelfSignedPEM(t, genCertCLIOpts{CN: "x"})
	cmd := newSSLCertCommand(sslCertCmdDeps{out: &bytes.Buffer{}})
	cmd.SetArgs([]string{"-f", path, "--no-ocsp", "--expires-in", "not-a-duration"})
	if err := cmd.Execute(); err == nil {
		t.Error("invalid --expires-in should error")
	}
}

func TestSSLCert_HostFromTLSServer(t *testing.T) {
	ts := httptest.NewTLSServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(200)
	}))
	defer ts.Close()
	u, _ := url.Parse(ts.URL)

	var buf bytes.Buffer
	cmd := newSSLCertCommand(sslCertCmdDeps{out: &buf})
	cmd.SetArgs([]string{u.Host, "--no-ocsp"})
	if err := cmd.Execute(); err != nil {
		t.Fatal(err)
	}
	out := buf.String()
	if !strings.Contains(out, "Verification:") {
		t.Errorf("missing verification section: %s", out)
	}
	// httptest 自签 → 必然 not trusted
	if !strings.Contains(out, "NOT trusted") && !strings.Contains(out, "not trusted") {
		t.Errorf("self-signed httptest server should be marked NOT trusted:\n%s", out)
	}
}

func TestSSLCert_NoArgsNoFile_Errors(t *testing.T) {
	cmd := newSSLCertCommand(sslCertCmdDeps{out: &bytes.Buffer{}})
	cmd.SetArgs([]string{})
	if err := cmd.Execute(); err == nil {
		t.Error("no host + no -f should error")
	}
}

func TestParseDurationWithDays(t *testing.T) {
	for _, tc := range []struct {
		in   string
		want time.Duration
	}{
		{"30d", 30 * 24 * time.Hour},
		{"1d", 24 * time.Hour},
		{"24h", 24 * time.Hour},
		{"720h", 720 * time.Hour},
		{"15m", 15 * time.Minute},
	} {
		got, err := parseDurationWithDays(tc.in)
		if err != nil {
			t.Errorf("parseDurationWithDays(%q) error: %v", tc.in, err)
			continue
		}
		if got != tc.want {
			t.Errorf("parseDurationWithDays(%q) = %v, want %v", tc.in, got, tc.want)
		}
	}
	for _, bad := range []string{"abc", "", "5x"} {
		if _, err := parseDurationWithDays(bad); err == nil {
			t.Errorf("parseDurationWithDays(%q) should error", bad)
		}
	}
}

func TestDaysProgressBar(t *testing.T) {
	for _, tc := range []struct {
		days, valid int
		wantBar     string
	}{
		{67, 90, "███████"},
		{0, 90, "░░░░"},
		{90, 90, "██████████"},
	} {
		got := daysProgressBar(tc.days, tc.valid)
		if !strings.Contains(got, tc.wantBar) {
			t.Errorf("daysProgressBar(%d/%d) = %q, missing pattern %q",
				tc.days, tc.valid, got, tc.wantBar)
		}
	}
}

func TestDaysProgressBar_Expired(t *testing.T) {
	got := daysProgressBar(-5, 90)
	if !strings.Contains(got, "EXPIRED") {
		t.Errorf("negative days should show EXPIRED: %s", got)
	}
}

func TestChainDescribed(t *testing.T) {
	// 通过 ParsePEMFile 拿一个真实 Bundle 然后 chainDescribed
	path := genSelfSignedPEM(t, genCertCLIOpts{CN: "chain.example"})
	b, err := sslcert.ParsePEMFile(path)
	if err != nil {
		t.Fatal(err)
	}
	summaries := chainDescribed(b.Chain)
	if len(summaries) != 1 {
		t.Errorf("got %d summaries, want 1", len(summaries))
	}
	if summaries[0].SubjectCN != "chain.example" {
		t.Errorf("CN: %s", summaries[0].SubjectCN)
	}
}
