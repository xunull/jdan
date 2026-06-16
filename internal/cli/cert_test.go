package cli

import (
	"bytes"
	"crypto/x509"
	"encoding/pem"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func TestCert_SelfSignedFiles(t *testing.T) {
	dir := t.TempDir()
	var buf bytes.Buffer
	cmd := newCertCommand(certCmdDeps{out: &buf})
	cmd.SetArgs([]string{"localhost", "--out-dir", dir})
	if err := cmd.Execute(); err != nil {
		t.Fatal(err)
	}
	certPath := filepath.Join(dir, "cert.pem")
	keyPath := filepath.Join(dir, "cert-key.pem")
	if _, err := os.Stat(certPath); err != nil {
		t.Errorf("cert.pem not written: %v", err)
	}
	if _, err := os.Stat(keyPath); err != nil {
		t.Errorf("cert-key.pem not written: %v", err)
	}
	// 生成的 cert 应当能解析回来，CN=localhost
	data, _ := os.ReadFile(certPath)
	block, _ := pem.Decode(data)
	c, err := x509.ParseCertificate(block.Bytes)
	if err != nil {
		t.Fatal(err)
	}
	if c.Subject.CommonName != "localhost" {
		t.Errorf("CN = %q", c.Subject.CommonName)
	}
	if len(c.DNSNames) != 1 || c.DNSNames[0] != "localhost" {
		t.Errorf("DNSNames = %v", c.DNSNames)
	}
}

func TestCert_KeyFilePerms0600(t *testing.T) {
	dir := t.TempDir()
	cmd := newCertCommand(certCmdDeps{out: &bytes.Buffer{}})
	cmd.SetArgs([]string{"localhost", "--out-dir", dir})
	if err := cmd.Execute(); err != nil {
		t.Fatal(err)
	}
	info, err := os.Stat(filepath.Join(dir, "cert-key.pem"))
	if err != nil {
		t.Fatal(err)
	}
	if info.Mode().Perm() != 0o600 {
		t.Errorf("key perm = %o, want 600", info.Mode().Perm())
	}
}

func TestCert_IPAndSAN(t *testing.T) {
	dir := t.TempDir()
	var buf bytes.Buffer
	cmd := newCertCommand(certCmdDeps{out: &buf})
	cmd.SetArgs([]string{"myapp", "--ip", "127.0.0.1,::1", "--san", "*.myapp.local", "--out-dir", dir})
	if err := cmd.Execute(); err != nil {
		t.Fatal(err)
	}
	data, _ := os.ReadFile(filepath.Join(dir, "cert.pem"))
	block, _ := pem.Decode(data)
	c, _ := x509.ParseCertificate(block.Bytes)
	if len(c.IPAddresses) != 2 {
		t.Errorf("IPs = %v", c.IPAddresses)
	}
	// myapp + *.myapp.local
	if len(c.DNSNames) != 2 {
		t.Errorf("DNSNames = %v", c.DNSNames)
	}
}

func TestCert_CAMode(t *testing.T) {
	dir := t.TempDir()
	var buf bytes.Buffer
	cmd := newCertCommand(certCmdDeps{out: &buf})
	cmd.SetArgs([]string{"localhost", "--ca", "--out-dir", dir})
	if err := cmd.Execute(); err != nil {
		t.Fatal(err)
	}
	for _, f := range []string{"cert.pem", "cert-key.pem", "ca.pem", "ca-key.pem"} {
		if _, err := os.Stat(filepath.Join(dir, f)); err != nil {
			t.Errorf("%s not written: %v", f, err)
		}
	}
	// ca.pem 应当是 CA
	caData, _ := os.ReadFile(filepath.Join(dir, "ca.pem"))
	block, _ := pem.Decode(caData)
	caCert, _ := x509.ParseCertificate(block.Bytes)
	if !caCert.IsCA {
		t.Error("ca.pem should be a CA")
	}
	// leaf 应当由 CA 签发
	leafData, _ := os.ReadFile(filepath.Join(dir, "cert.pem"))
	lblock, _ := pem.Decode(leafData)
	leaf, _ := x509.ParseCertificate(lblock.Bytes)
	if err := leaf.CheckSignatureFrom(caCert); err != nil {
		t.Errorf("leaf should be signed by CA: %v", err)
	}
}

func TestCert_KeyTypes(t *testing.T) {
	for _, kt := range []string{"ec", "rsa", "ed25519"} {
		dir := t.TempDir()
		cmd := newCertCommand(certCmdDeps{out: &bytes.Buffer{}})
		cmd.SetArgs([]string{"localhost", "--key-type", kt, "--out-dir", dir})
		if err := cmd.Execute(); err != nil {
			t.Errorf("key type %s errored: %v", kt, err)
		}
	}
}

func TestCert_InvalidKeyType(t *testing.T) {
	cmd := newCertCommand(certCmdDeps{out: &bytes.Buffer{}})
	cmd.SetArgs([]string{"localhost", "--key-type", "dsa"})
	if err := cmd.Execute(); err == nil {
		t.Error("invalid key type should error")
	}
}

func TestCert_Stdout(t *testing.T) {
	var buf bytes.Buffer
	cmd := newCertCommand(certCmdDeps{out: &buf})
	cmd.SetArgs([]string{"localhost", "--stdout"})
	if err := cmd.Execute(); err != nil {
		t.Fatal(err)
	}
	out := buf.String()
	if !strings.Contains(out, "BEGIN CERTIFICATE") || !strings.Contains(out, "BEGIN PRIVATE KEY") {
		t.Errorf("stdout should contain cert + key PEM:\n%s", out[:min(200, len(out))])
	}
}

func TestCert_JSON(t *testing.T) {
	dir := t.TempDir()
	var buf bytes.Buffer
	cmd := newCertCommand(certCmdDeps{out: &buf})
	cmd.SetArgs([]string{"localhost", "--json", "--out-dir", dir})
	if err := cmd.Execute(); err != nil {
		t.Fatal(err)
	}
	out := buf.String()
	for _, want := range []string{
		`"subject": "localhost"`,
		`"san": "DNS:localhost"`,
		`"key_type": "EC (P-256)"`,
		`"self_signed": true`,
		`"fingerprint": "SHA256:`,
	} {
		if !strings.Contains(out, want) {
			t.Errorf("missing %q in:\n%s", want, out)
		}
	}
}

func TestCert_DefaultRender(t *testing.T) {
	dir := t.TempDir()
	var buf bytes.Buffer
	cmd := newCertCommand(certCmdDeps{out: &buf})
	cmd.SetArgs([]string{"localhost", "--out-dir", dir})
	if err := cmd.Execute(); err != nil {
		t.Fatal(err)
	}
	out := buf.String()
	for _, want := range []string{
		"Generated self-signed certificate",
		"Subject:     CN=localhost",
		"Self-signed: browsers will warn",
	} {
		if !strings.Contains(out, want) {
			t.Errorf("missing %q in:\n%s", want, out)
		}
	}
}
