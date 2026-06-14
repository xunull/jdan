package cli

import (
	"bytes"
	"crypto/ecdsa"
	"crypto/elliptic"
	"crypto/rand"
	"crypto/x509"
	"crypto/x509/pkix"
	"encoding/json"
	"encoding/pem"
	"math/big"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"
)

// genPinCertPEM 生成 N 个 self-signed cert（chain 的近似），写一个 PEM 文件给 -f 测试用。
func genPinCertPEM(t *testing.T, count int) string {
	t.Helper()
	dir := t.TempDir()
	path := filepath.Join(dir, "chain.pem")
	var buf []byte
	for i := 0; i < count; i++ {
		key, err := ecdsa.GenerateKey(elliptic.P256(), rand.Reader)
		if err != nil {
			t.Fatal(err)
		}
		tmpl := &x509.Certificate{
			SerialNumber: big.NewInt(int64(i + 1)),
			Subject:      pkix.Name{CommonName: "pin-test-" + string(rune('a'+i))},
			NotBefore:    time.Now(),
			NotAfter:     time.Now().Add(time.Hour),
		}
		der, err := x509.CreateCertificate(rand.Reader, tmpl, tmpl, &key.PublicKey, key)
		if err != nil {
			t.Fatal(err)
		}
		buf = append(buf, pem.EncodeToMemory(&pem.Block{Type: "CERTIFICATE", Bytes: der})...)
	}
	if err := os.WriteFile(path, buf, 0o644); err != nil {
		t.Fatal(err)
	}
	return path
}

func TestSSLPin_File_DefaultLeafAndIntermediate(t *testing.T) {
	path := genPinCertPEM(t, 2)
	var buf bytes.Buffer
	cmd := newSSLPinCommand(sslPinCmdDeps{out: &buf})
	cmd.SetArgs([]string{"-f", path})
	if err := cmd.Execute(); err != nil {
		t.Fatal(err)
	}
	out := buf.String()
	// 默认 2 个 cert：Leaf + Intermediate
	if !strings.Contains(out, "Leaf") {
		t.Errorf("missing Leaf section: %s", out)
	}
	if !strings.Contains(out, "Intermediate") {
		t.Errorf("missing Intermediate section: %s", out)
	}
	// 应当有所有 6 个格式
	for _, name := range []string{"okhttp", "ios", "hpkp", "nss", "curl", "raw"} {
		if !strings.Contains(out, "▸ "+name+":") {
			t.Errorf("missing %q format header", name)
		}
	}
}

func TestSSLPin_File_LeafOnly(t *testing.T) {
	path := genPinCertPEM(t, 3)
	var buf bytes.Buffer
	cmd := newSSLPinCommand(sslPinCmdDeps{out: &buf})
	cmd.SetArgs([]string{"-f", path, "--leaf-only"})
	if err := cmd.Execute(); err != nil {
		t.Fatal(err)
	}
	out := buf.String()
	if !strings.Contains(out, "Leaf") {
		t.Error("--leaf-only missing Leaf section")
	}
	if strings.Contains(out, "Intermediate") {
		t.Error("--leaf-only should not show Intermediate")
	}
}

func TestSSLPin_File_Full(t *testing.T) {
	path := genPinCertPEM(t, 3)
	var buf bytes.Buffer
	cmd := newSSLPinCommand(sslPinCmdDeps{out: &buf})
	cmd.SetArgs([]string{"-f", path, "--full"})
	if err := cmd.Execute(); err != nil {
		t.Fatal(err)
	}
	out := buf.String()
	// 应当看到全部 3 个 cert（leaf + 2 intermediate；最后一个 self-signed 算 root 还是 intermediate
	// 取决于 chain 是否能识别——这里都 self-signed 所以最后一个标 "Root"）
	if !strings.Contains(out, "Leaf") {
		t.Error("--full missing Leaf")
	}
	// 至少 2 个 SPKI hash
	hashCount := strings.Count(out, "SPKI hash:")
	if hashCount < 3 {
		t.Errorf("--full with 3 cert chain should show 3 SPKI hashes, got %d", hashCount)
	}
}

func TestSSLPin_LeafOnlyAndFull_MutuallyExclusive(t *testing.T) {
	path := genPinCertPEM(t, 2)
	cmd := newSSLPinCommand(sslPinCmdDeps{out: &bytes.Buffer{}})
	cmd.SetArgs([]string{"-f", path, "--leaf-only", "--full"})
	if err := cmd.Execute(); err == nil {
		t.Error("--leaf-only + --full should error (mutually exclusive)")
	}
}

func TestSSLPin_Format_Single(t *testing.T) {
	path := genPinCertPEM(t, 2)
	var buf bytes.Buffer
	cmd := newSSLPinCommand(sslPinCmdDeps{out: &buf})
	cmd.SetArgs([]string{"-f", path, "--format", "okhttp"})
	if err := cmd.Execute(); err != nil {
		t.Fatal(err)
	}
	out := buf.String()
	// 应当只有 OkHttp 输出，不带其他格式 header
	if !strings.Contains(out, "CertificatePinner.Builder()") {
		t.Error("missing OkHttp builder")
	}
	if strings.Contains(out, "<key>NSAppTransportSecurity</key>") {
		t.Error("--format okhttp should not include iOS plist")
	}
	if strings.Contains(out, "Public-Key-Pins:") {
		t.Error("--format okhttp should not include HPKP header")
	}
}

func TestSSLPin_Format_Invalid(t *testing.T) {
	path := genPinCertPEM(t, 2)
	cmd := newSSLPinCommand(sslPinCmdDeps{out: &bytes.Buffer{}})
	cmd.SetArgs([]string{"-f", path, "--format", "nonexistent"})
	if err := cmd.Execute(); err == nil {
		t.Error("unknown format should error")
	}
}

func TestSSLPin_JSON(t *testing.T) {
	path := genPinCertPEM(t, 2)
	var buf bytes.Buffer
	cmd := newSSLPinCommand(sslPinCmdDeps{out: &buf})
	cmd.SetArgs([]string{"-f", path, "--json"})
	if err := cmd.Execute(); err != nil {
		t.Fatal(err)
	}
	var payload map[string]interface{}
	if err := json.Unmarshal(buf.Bytes(), &payload); err != nil {
		t.Fatalf("invalid JSON: %v\n%s", err, buf.String())
	}
	if payload["entries"] == nil {
		t.Error("JSON missing entries")
	}
	if payload["formats"] == nil {
		t.Error("JSON missing formats map")
	}
	// formats 应当含所有 6 个 key
	formats := payload["formats"].(map[string]interface{})
	for _, name := range []string{"okhttp", "ios", "hpkp", "nss", "curl", "raw"} {
		if formats[name] == nil {
			t.Errorf("JSON formats missing %q", name)
		}
	}
}

func TestSSLPin_NoArgsNoFile_Errors(t *testing.T) {
	cmd := newSSLPinCommand(sslPinCmdDeps{out: &bytes.Buffer{}})
	cmd.SetArgs([]string{})
	if err := cmd.Execute(); err == nil {
		t.Error("no host + no -f should error")
	}
}

func TestSSLPin_FileMissing_Errors(t *testing.T) {
	cmd := newSSLPinCommand(sslPinCmdDeps{out: &bytes.Buffer{}})
	cmd.SetArgs([]string{"-f", "/definitely/not/exist.pem"})
	if err := cmd.Execute(); err == nil {
		t.Error("missing file should error")
	}
}
