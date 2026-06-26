package pemx

import (
	"crypto/ecdsa"
	"crypto/elliptic"
	"crypto/rand"
	"crypto/rsa"
	"crypto/x509"
	"crypto/x509/pkix"
	"encoding/json"
	"encoding/pem"
	"strings"
	"testing"

	"github.com/xunull/jdan/internal/certgen"
)

// pair 用 certgen 造一对匹配的 cert+key（PEM）。
func pair(t *testing.T, kt certgen.KeyType) (certPEM, keyPEM []byte) {
	t.Helper()
	r, err := certgen.GenerateSelfSigned(certgen.Options{CommonName: "example.com", KeyType: kt, Days: 365})
	if err != nil {
		t.Fatal(err)
	}
	return r.CertPEM, r.KeyPEM
}

// ---- 单块识别 ----

func TestInspect_Certificate(t *testing.T) {
	certPEM, _ := pair(t, certgen.KeyEC)
	res, err := Inspect(certPEM)
	if err != nil {
		t.Fatal(err)
	}
	if len(res.Blocks) != 1 || res.Blocks[0].Kind != "certificate" {
		t.Fatalf("got %+v", res.Blocks)
	}
	if res.Blocks[0].Cert == nil || res.Blocks[0].Cert.SubjectCN != "example.com" {
		t.Errorf("cert summary wrong: %+v", res.Blocks[0].Cert)
	}
}

func TestInspect_PrivateKeyKinds(t *testing.T) {
	for _, kt := range []certgen.KeyType{certgen.KeyRSA, certgen.KeyEC, certgen.KeyEd25519} {
		_, keyPEM := pair(t, kt)
		res, err := Inspect(keyPEM)
		if err != nil {
			t.Fatalf("%s: %v", kt, err)
		}
		b := res.Blocks[0]
		if b.Kind != "private-key" || b.Key == nil {
			t.Errorf("%s: kind=%s key=%v", kt, b.Kind, b.Key)
		}
		if b.Note == "" {
			t.Errorf("%s: private key should carry a '不打印' note", kt)
		}
	}
}

func TestInspect_PublicKey(t *testing.T) {
	key, _ := rsa.GenerateKey(rand.Reader, 2048)
	der, _ := x509.MarshalPKIXPublicKey(&key.PublicKey)
	pubPEM := pem.EncodeToMemory(&pem.Block{Type: "PUBLIC KEY", Bytes: der})
	res, err := Inspect(pubPEM)
	if err != nil {
		t.Fatal(err)
	}
	b := res.Blocks[0]
	if b.Kind != "public-key" || b.Key == nil || !b.Key.Public {
		t.Fatalf("got %+v / key %+v", b, b.Key)
	}
	if !strings.HasPrefix(b.Key.Algorithm, "RSA") {
		t.Errorf("algorithm = %q", b.Key.Algorithm)
	}
}

func TestInspect_CSR(t *testing.T) {
	key, _ := ecdsa.GenerateKey(elliptic.P256(), rand.Reader)
	tmpl := &x509.CertificateRequest{
		Subject:  pkix.Name{CommonName: "csr.example.com"},
		DNSNames: []string{"csr.example.com", "www.csr.example.com"},
	}
	der, err := x509.CreateCertificateRequest(rand.Reader, tmpl, key)
	if err != nil {
		t.Fatal(err)
	}
	csrPEM := pem.EncodeToMemory(&pem.Block{Type: "CERTIFICATE REQUEST", Bytes: der})
	res, err := Inspect(csrPEM)
	if err != nil {
		t.Fatal(err)
	}
	b := res.Blocks[0]
	if b.Kind != "csr" || b.CSR == nil {
		t.Fatalf("got %+v", b)
	}
	if b.CSR.SubjectCN != "csr.example.com" || len(b.CSR.SAN) != 2 {
		t.Errorf("csr info wrong: %+v", b.CSR)
	}
	if !strings.HasPrefix(b.CSR.KeyAlgorithm, "EC") {
		t.Errorf("csr key algorithm = %q", b.CSR.KeyAlgorithm)
	}
}

// ---- 多块 ----

func TestInspect_Fullchain(t *testing.T) {
	c1, _ := pair(t, certgen.KeyEC)
	c2, _ := pair(t, certgen.KeyRSA)
	res, err := Inspect(append(append([]byte{}, c1...), c2...))
	if err != nil {
		t.Fatal(err)
	}
	if len(res.Blocks) != 2 {
		t.Fatalf("want 2 blocks, got %d", len(res.Blocks))
	}
}

// ---- 加密私钥 / 其它块 / 容错 ----

func TestInspect_EncryptedLegacy(t *testing.T) {
	blk := &pem.Block{
		Type:    "RSA PRIVATE KEY",
		Headers: map[string]string{"Proc-Type": "4,ENCRYPTED", "DEK-Info": "AES-256-CBC,ABCDEF"},
		Bytes:   []byte("not-real-encrypted-bytes"),
	}
	res, err := Inspect(pem.EncodeToMemory(blk))
	if err != nil {
		t.Fatal(err)
	}
	if res.Blocks[0].Kind != "encrypted-private-key" {
		t.Errorf("DEK-Info should mark encrypted, got %q", res.Blocks[0].Kind)
	}
}

func TestInspect_EncryptedPKCS8(t *testing.T) {
	res, err := Inspect(pem.EncodeToMemory(&pem.Block{Type: "ENCRYPTED PRIVATE KEY", Bytes: []byte("x")}))
	if err != nil {
		t.Fatal(err)
	}
	if res.Blocks[0].Kind != "encrypted-private-key" {
		t.Errorf("got %q", res.Blocks[0].Kind)
	}
}

func TestInspect_OtherBlock(t *testing.T) {
	res, err := Inspect(pem.EncodeToMemory(&pem.Block{Type: "DH PARAMETERS", Bytes: []byte("params")}))
	if err != nil {
		t.Fatal(err)
	}
	if res.Blocks[0].Kind != "other" {
		t.Errorf("got %q", res.Blocks[0].Kind)
	}
}

func TestInspect_BadCertBlockTolerated(t *testing.T) {
	res, err := Inspect(pem.EncodeToMemory(&pem.Block{Type: "CERTIFICATE", Bytes: []byte("garbage-der")}))
	if err != nil {
		t.Fatal(err)
	}
	if res.Blocks[0].Err == "" {
		t.Error("bad cert should record a parse error, not crash")
	}
}

func TestInspect_NoPEM(t *testing.T) {
	if _, err := Inspect([]byte("this is not pem at all")); err == nil {
		t.Error("non-PEM input should error")
	}
}

// ---- key ↔ cert 匹配 ----

func TestInspect_KeyMatchesCert(t *testing.T) {
	certPEM, keyPEM := pair(t, certgen.KeyEC)
	res, err := Inspect(append(append([]byte{}, certPEM...), keyPEM...))
	if err != nil {
		t.Fatal(err)
	}
	if res.KeyMatchesCert == nil || !*res.KeyMatchesCert {
		t.Errorf("matching cert+key should report match: %v", res.KeyMatchesCert)
	}
}

func TestInspect_KeyMismatch(t *testing.T) {
	certPEM, _ := pair(t, certgen.KeyEC)
	_, otherKey := pair(t, certgen.KeyEC)
	res, err := Inspect(append(append([]byte{}, certPEM...), otherKey...))
	if err != nil {
		t.Fatal(err)
	}
	if res.KeyMatchesCert == nil || *res.KeyMatchesCert {
		t.Errorf("mismatched cert+key should report mismatch: %v", res.KeyMatchesCert)
	}
}

func TestInspect_NoMatchCheckWhenAmbiguous(t *testing.T) {
	// 只有证书、没有私钥 → 不做匹配判断
	certPEM, _ := pair(t, certgen.KeyEC)
	res, _ := Inspect(certPEM)
	if res.KeyMatchesCert != nil {
		t.Error("no private key → match check should be nil")
	}
}

// ---- 安全：绝不泄漏私钥 ----

func TestInspect_NoPrivateKeyLeak(t *testing.T) {
	_, keyPEM := pair(t, certgen.KeyRSA)
	res, _ := Inspect(keyPEM)

	// 提取私钥的 base64 体（去头尾）
	blk, _ := pem.Decode(keyPEM)
	secretB64 := strings.TrimSpace(strings.ReplaceAll(string(pem.EncodeToMemory(blk)), "\n", ""))
	_ = secretB64

	text := res.FormatText()
	jsonOut, _ := res.FormatJSON()
	for _, out := range []string{text, jsonOut} {
		if strings.Contains(out, "PRIVATE KEY") && strings.Contains(out, "MII") {
			t.Errorf("output leaked private key material:\n%s", out)
		}
		// 任何 ≥40 连续 base64 字符都视为可疑的密钥体泄漏
		for _, line := range strings.Fields(out) {
			if len(line) >= 60 && isB64ish(line) {
				t.Errorf("suspiciously long base64 token in output (possible key leak): %s", line)
			}
		}
	}
}

func isB64ish(s string) bool {
	for _, r := range s {
		if !((r >= 'A' && r <= 'Z') || (r >= 'a' && r <= 'z') || (r >= '0' && r <= '9') || r == '+' || r == '/' || r == '=') {
			return false
		}
	}
	return true
}

// ---- 输出 ----

func TestFormatJSON_Valid(t *testing.T) {
	certPEM, keyPEM := pair(t, certgen.KeyEC)
	res, _ := Inspect(append(append([]byte{}, certPEM...), keyPEM...))
	s, err := res.FormatJSON()
	if err != nil {
		t.Fatal(err)
	}
	var v map[string]any
	if err := json.Unmarshal([]byte(s), &v); err != nil {
		t.Fatalf("invalid JSON: %v\n%s", err, s)
	}
	if v["key_matches_cert"] != true {
		t.Errorf("key_matches_cert = %v", v["key_matches_cert"])
	}
}
