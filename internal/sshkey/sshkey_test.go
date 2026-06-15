package sshkey

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func readKey(t *testing.T, name string) []byte {
	t.Helper()
	b, err := os.ReadFile(filepath.Join("testdata", name))
	if err != nil {
		t.Fatal(err)
	}
	return b
}

// 参考值来自 ssh-keygen -lf / -lf -E md5 / -y（生成 fixture 时抓取）。
const (
	ed25519SHA256 = "SHA256:SzSqqFFld3c5fMa83VFfeqj8pTuFwlEOl7E2HuVfloU"
	ed25519MD5    = "MD5:e0:18:f6:ec:b7:d1:52:d6:dc:b2:a8:53:b0:41:14:dc"
	ed25519Pub    = "ssh-ed25519 AAAAC3NzaC1lZDI1NTE5AAAAIBacgxyVVKWrOOhXHfv9F3/L9pATfWSk4Uj17X6zoo4P test@jdan"
	rsa2048SHA256 = "SHA256:910z2tdmhhJrvnWK/PfsQPA/ge16C88PnTsIBec3JEI"
	ecdsaSHA256   = "SHA256:cXxRjj62xxXqfFgz+TOvhg87awFQ6n/qAHRlZ8Mcr4w"
)

// ---- IsPrivateKey ----

func TestIsPrivateKey(t *testing.T) {
	if !IsPrivateKey(readKey(t, "ed25519")) {
		t.Error("ed25519 (private) should be detected as private")
	}
	if IsPrivateKey(readKey(t, "ed25519.pub")) {
		t.Error("ed25519.pub should NOT be detected as private")
	}
}

// ---- ParsePublicKey ----

func TestParsePublicKey_Ed25519(t *testing.T) {
	pub, comment, err := ParsePublicKey(readKey(t, "ed25519.pub"))
	if err != nil {
		t.Fatal(err)
	}
	if pub.Type() != "ssh-ed25519" {
		t.Errorf("type = %s", pub.Type())
	}
	if comment != "test@jdan" {
		t.Errorf("comment = %q", comment)
	}
}

func TestParsePublicKey_Invalid(t *testing.T) {
	if _, _, err := ParsePublicKey([]byte("not a key")); err == nil {
		t.Error("invalid key should error")
	}
}

// ---- Fingerprint（byte-equal 对齐 ssh-keygen）----

func TestFingerprintSHA256_Ed25519(t *testing.T) {
	pub, _, _ := ParsePublicKey(readKey(t, "ed25519.pub"))
	if got := FingerprintSHA256(pub); got != ed25519SHA256 {
		t.Errorf("got %q, want %q", got, ed25519SHA256)
	}
}

func TestFingerprintMD5_Ed25519(t *testing.T) {
	pub, _, _ := ParsePublicKey(readKey(t, "ed25519.pub"))
	if got := FingerprintMD5(pub); got != ed25519MD5 {
		t.Errorf("got %q, want %q", got, ed25519MD5)
	}
}

func TestFingerprintSHA256_RSA(t *testing.T) {
	pub, _, _ := ParsePublicKey(readKey(t, "rsa2048.pub"))
	if got := FingerprintSHA256(pub); got != rsa2048SHA256 {
		t.Errorf("got %q, want %q", got, rsa2048SHA256)
	}
}

func TestFingerprintSHA256_ECDSA(t *testing.T) {
	pub, _, _ := ParsePublicKey(readKey(t, "ecdsa256.pub"))
	if got := FingerprintSHA256(pub); got != ecdsaSHA256 {
		t.Errorf("got %q, want %q", got, ecdsaSHA256)
	}
}

// ---- KeyInfo bits + algorithm ----

func TestInfoFromPublicKey_Ed25519(t *testing.T) {
	pub, comment, _ := ParsePublicKey(readKey(t, "ed25519.pub"))
	info := InfoFromPublicKey(pub, comment)
	if info.Algorithm != "Ed25519" {
		t.Errorf("algorithm = %s", info.Algorithm)
	}
	if info.Bits != 256 {
		t.Errorf("bits = %d, want 256", info.Bits)
	}
	if info.Kind != "public" {
		t.Errorf("kind = %s", info.Kind)
	}
}

func TestInfoFromPublicKey_RSA_Bits(t *testing.T) {
	pub, comment, _ := ParsePublicKey(readKey(t, "rsa2048.pub"))
	info := InfoFromPublicKey(pub, comment)
	if info.Algorithm != "RSA" {
		t.Errorf("algorithm = %s", info.Algorithm)
	}
	if info.Bits != 2048 {
		t.Errorf("bits = %d, want 2048", info.Bits)
	}
}

func TestInfoFromPublicKey_ECDSA_Bits(t *testing.T) {
	pub, comment, _ := ParsePublicKey(readKey(t, "ecdsa256.pub"))
	info := InfoFromPublicKey(pub, comment)
	if info.Algorithm != "ECDSA" {
		t.Errorf("algorithm = %s", info.Algorithm)
	}
	if info.Bits != 256 {
		t.Errorf("bits = %d, want 256", info.Bits)
	}
}

// ---- ParsePrivateKey + 公钥导出 ----

func TestParsePrivateKey_Unencrypted(t *testing.T) {
	signer, _, err := ParsePrivateKey(readKey(t, "ed25519"), "")
	if err != nil {
		t.Fatal(err)
	}
	// 导出公钥应当跟 ssh-keygen -y 一致（不含 comment 部分）
	got := PublicKeyLine(signer, "test@jdan")
	if got != ed25519Pub {
		t.Errorf("got %q, want %q", got, ed25519Pub)
	}
}

func TestParsePrivateKey_Encrypted_NoPass_Errors(t *testing.T) {
	_, _, err := ParsePrivateKey(readKey(t, "ed25519_enc"), "")
	if err != ErrEncrypted {
		t.Errorf("expected ErrEncrypted, got %v", err)
	}
}

func TestParsePrivateKey_Encrypted_WithPass(t *testing.T) {
	signer, _, err := ParsePrivateKey(readKey(t, "ed25519_enc"), "secretpass")
	if err != nil {
		t.Fatal(err)
	}
	if signer.PublicKey().Type() != "ssh-ed25519" {
		t.Errorf("type = %s", signer.PublicKey().Type())
	}
}

func TestParsePrivateKey_WrongPass_Errors(t *testing.T) {
	_, _, err := ParsePrivateKey(readKey(t, "ed25519_enc"), "wrongpass")
	if err == nil {
		t.Error("wrong passphrase should error")
	}
}

// ---- IsEncryptedPrivateKey ----

func TestIsEncryptedPrivateKey(t *testing.T) {
	if !IsEncryptedPrivateKey(readKey(t, "ed25519_enc")) {
		t.Error("ed25519_enc should be detected as encrypted")
	}
	if IsEncryptedPrivateKey(readKey(t, "ed25519")) {
		t.Error("ed25519 (unencrypted) should NOT be encrypted")
	}
}

// ---- InfoFromSigner ----

func TestInfoFromSigner(t *testing.T) {
	signer, _, _ := ParsePrivateKey(readKey(t, "ed25519"), "")
	info := InfoFromSigner(signer, "test@jdan")
	if info.Kind != "private" {
		t.Errorf("kind = %s", info.Kind)
	}
	if info.PublicKeyLine != ed25519Pub {
		t.Errorf("PublicKeyLine = %q", info.PublicKeyLine)
	}
	// fingerprint 应当跟公钥一致
	if info.FingerprintSHA != ed25519SHA256 {
		t.Errorf("private key fingerprint should match public: %q", info.FingerprintSHA)
	}
}

// ---- String 渲染 ----

func TestKeyInfo_String_Encrypted(t *testing.T) {
	info := KeyInfo{Kind: "private", Encrypted: true}
	s := info.String()
	if !strings.Contains(s, "Encrypted:") || !strings.Contains(s, "passphrase-protected") {
		t.Errorf("encrypted info should render Encrypted line:\n%s", s)
	}
	// 加密时不应泄露 algorithm/fingerprint（都是空）
	if strings.Contains(s, "Fingerprint:") {
		t.Errorf("encrypted info should not show fingerprint:\n%s", s)
	}
}
