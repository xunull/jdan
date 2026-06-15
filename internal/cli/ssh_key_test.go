package cli

import (
	"bytes"
	"strings"
	"testing"
)

const tdata = "../sshkey/testdata/"

func TestSSHKeyInfo_PublicEd25519(t *testing.T) {
	var buf bytes.Buffer
	cmd := newSSHKeyCommand(sshKeyCmdDeps{out: &buf})
	cmd.SetArgs([]string{"info", tdata + "ed25519.pub"})
	if err := cmd.Execute(); err != nil {
		t.Fatal(err)
	}
	out := buf.String()
	for _, want := range []string{
		"Type:         ssh-ed25519",
		"Algorithm:    Ed25519",
		"Bits:         256",
		"Comment:      test@jdan",
		"Fingerprint:  SHA256:SzSqqFFld3c5fMa83VFfeqj8pTuFwlEOl7E2HuVfloU",
	} {
		if !strings.Contains(out, want) {
			t.Errorf("missing %q in:\n%s", want, out)
		}
	}
}

func TestSSHKeyInfo_PrivateEd25519(t *testing.T) {
	var buf bytes.Buffer
	cmd := newSSHKeyCommand(sshKeyCmdDeps{out: &buf})
	cmd.SetArgs([]string{"info", tdata + "ed25519"})
	if err := cmd.Execute(); err != nil {
		t.Fatal(err)
	}
	out := buf.String()
	if !strings.Contains(out, "OpenSSH private key") {
		t.Errorf("private key should show OpenSSH private key:\n%s", out)
	}
	// comment 从同名 .pub 文件回退读取
	if !strings.Contains(out, "Comment:      test@jdan") {
		t.Errorf("comment should be read from sibling .pub:\n%s", out)
	}
	if !strings.Contains(out, "Public key:   ssh-ed25519") {
		t.Errorf("should show derived public key:\n%s", out)
	}
}

func TestSSHKeyInfo_EncryptedPrivate(t *testing.T) {
	var buf bytes.Buffer
	cmd := newSSHKeyCommand(sshKeyCmdDeps{out: &buf})
	cmd.SetArgs([]string{"info", tdata + "ed25519_enc"})
	if err := cmd.Execute(); err != nil {
		t.Fatal(err)
	}
	out := buf.String()
	if !strings.Contains(out, "Encrypted:") {
		t.Errorf("encrypted key should show Encrypted line:\n%s", out)
	}
	// 不解密时不该泄露 fingerprint
	if strings.Contains(out, "Fingerprint:") {
		t.Errorf("should not derive fingerprint without passphrase:\n%s", out)
	}
}

func TestSSHKeyInfo_EncryptedWithPassphrase(t *testing.T) {
	var buf bytes.Buffer
	cmd := newSSHKeyCommand(sshKeyCmdDeps{out: &buf})
	cmd.SetArgs([]string{"info", tdata + "ed25519_enc", "--passphrase", "secretpass"})
	if err := cmd.Execute(); err != nil {
		t.Fatal(err)
	}
	out := buf.String()
	if !strings.Contains(out, "Fingerprint:  SHA256:") {
		t.Errorf("with passphrase should derive fingerprint:\n%s", out)
	}
	if !strings.Contains(out, "Comment:      encrypted@jdan") {
		t.Errorf("comment should be present:\n%s", out)
	}
}

func TestSSHKeyInfo_JSON(t *testing.T) {
	var buf bytes.Buffer
	cmd := newSSHKeyCommand(sshKeyCmdDeps{out: &buf})
	cmd.SetArgs([]string{"info", tdata + "rsa2048.pub", "--json"})
	if err := cmd.Execute(); err != nil {
		t.Fatal(err)
	}
	out := buf.String()
	for _, want := range []string{
		`"kind": "public"`,
		`"type": "ssh-rsa"`,
		`"algorithm": "RSA"`,
		`"bits": 2048`,
		`"fingerprint_sha256": "SHA256:910z2tdmhhJrvnWK/PfsQPA/ge16C88PnTsIBec3JEI"`,
	} {
		if !strings.Contains(out, want) {
			t.Errorf("missing %q in:\n%s", want, out)
		}
	}
}

func TestSSHKeyInfo_PastedPublicKey(t *testing.T) {
	var buf bytes.Buffer
	cmd := newSSHKeyCommand(sshKeyCmdDeps{out: &buf})
	cmd.SetArgs([]string{"info", "ssh-ed25519 AAAAC3NzaC1lZDI1NTE5AAAAIBacgxyVVKWrOOhXHfv9F3/L9pATfWSk4Uj17X6zoo4P pasted@inline"})
	if err := cmd.Execute(); err != nil {
		t.Fatal(err)
	}
	out := buf.String()
	if !strings.Contains(out, "Comment:      pasted@inline") {
		t.Errorf("inline pasted key should parse comment:\n%s", out)
	}
	if !strings.Contains(out, "SHA256:SzSqqFFld3c5fMa83VFfeqj8pTuFwlEOl7E2HuVfloU") {
		t.Errorf("inline pasted key fingerprint wrong:\n%s", out)
	}
}

func TestSSHKeyFingerprint_SHA256(t *testing.T) {
	var buf bytes.Buffer
	cmd := newSSHKeyCommand(sshKeyCmdDeps{out: &buf})
	cmd.SetArgs([]string{"fingerprint", tdata + "ed25519.pub"})
	if err := cmd.Execute(); err != nil {
		t.Fatal(err)
	}
	got := strings.TrimSpace(buf.String())
	// 跟 ssh-keygen -lf byte-equal
	want := "256 SHA256:SzSqqFFld3c5fMa83VFfeqj8pTuFwlEOl7E2HuVfloU test@jdan (ED25519)"
	if got != want {
		t.Errorf("got  %q\nwant %q", got, want)
	}
}

func TestSSHKeyFingerprint_MD5(t *testing.T) {
	var buf bytes.Buffer
	cmd := newSSHKeyCommand(sshKeyCmdDeps{out: &buf})
	cmd.SetArgs([]string{"fingerprint", tdata + "ed25519.pub", "--md5"})
	if err := cmd.Execute(); err != nil {
		t.Fatal(err)
	}
	got := strings.TrimSpace(buf.String())
	want := "256 MD5:e0:18:f6:ec:b7:d1:52:d6:dc:b2:a8:53:b0:41:14:dc test@jdan (ED25519)"
	if got != want {
		t.Errorf("got  %q\nwant %q", got, want)
	}
}

func TestSSHKeyFingerprint_FromPrivateKey(t *testing.T) {
	var buf bytes.Buffer
	cmd := newSSHKeyCommand(sshKeyCmdDeps{out: &buf})
	cmd.SetArgs([]string{"fingerprint", tdata + "ed25519"})
	if err := cmd.Execute(); err != nil {
		t.Fatal(err)
	}
	// 私钥的 fingerprint 应当跟公钥一致
	if !strings.Contains(buf.String(), "SHA256:SzSqqFFld3c5fMa83VFfeqj8pTuFwlEOl7E2HuVfloU") {
		t.Errorf("private key fingerprint should match public:\n%s", buf.String())
	}
}

func TestSSHKeyFingerprint_EncryptedErrors(t *testing.T) {
	cmd := newSSHKeyCommand(sshKeyCmdDeps{out: &bytes.Buffer{}})
	cmd.SetArgs([]string{"fingerprint", tdata + "ed25519_enc"})
	err := cmd.Execute()
	if err == nil || !strings.Contains(err.Error(), "passphrase") {
		t.Errorf("encrypted key without passphrase should error, got %v", err)
	}
}

func TestSSHKeyFingerprint_JSON(t *testing.T) {
	var buf bytes.Buffer
	cmd := newSSHKeyCommand(sshKeyCmdDeps{out: &buf})
	cmd.SetArgs([]string{"fingerprint", tdata + "ed25519.pub", "--json"})
	if err := cmd.Execute(); err != nil {
		t.Fatal(err)
	}
	out := buf.String()
	for _, want := range []string{
		`"bits": 256`,
		`"fingerprint": "SHA256:SzSqqFFld3c5fMa83VFfeqj8pTuFwlEOl7E2HuVfloU"`,
		`"algorithm": "Ed25519"`,
	} {
		if !strings.Contains(out, want) {
			t.Errorf("missing %q in:\n%s", want, out)
		}
	}
}

func TestSSHKeyPubkey_Ed25519(t *testing.T) {
	var buf bytes.Buffer
	cmd := newSSHKeyCommand(sshKeyCmdDeps{out: &buf})
	cmd.SetArgs([]string{"pubkey", tdata + "ed25519"})
	if err := cmd.Execute(); err != nil {
		t.Fatal(err)
	}
	got := strings.TrimSpace(buf.String())
	// 跟 ssh-keygen -y byte-equal
	want := "ssh-ed25519 AAAAC3NzaC1lZDI1NTE5AAAAIBacgxyVVKWrOOhXHfv9F3/L9pATfWSk4Uj17X6zoo4P test@jdan"
	if got != want {
		t.Errorf("got  %q\nwant %q", got, want)
	}
}

func TestSSHKeyPubkey_RejectsPublicKey(t *testing.T) {
	cmd := newSSHKeyCommand(sshKeyCmdDeps{out: &bytes.Buffer{}})
	cmd.SetArgs([]string{"pubkey", tdata + "ed25519.pub"})
	err := cmd.Execute()
	if err == nil || !strings.Contains(err.Error(), "private key") {
		t.Errorf("pubkey on public key should error, got %v", err)
	}
}

func TestSSHKeyPubkey_EncryptedNeedsPassphrase(t *testing.T) {
	cmd := newSSHKeyCommand(sshKeyCmdDeps{out: &bytes.Buffer{}})
	cmd.SetArgs([]string{"pubkey", tdata + "ed25519_enc"})
	err := cmd.Execute()
	if err == nil || !strings.Contains(err.Error(), "passphrase") {
		t.Errorf("encrypted key should require passphrase, got %v", err)
	}
}

func TestSSHKeyPubkey_EncryptedWithPassphrase(t *testing.T) {
	var buf bytes.Buffer
	cmd := newSSHKeyCommand(sshKeyCmdDeps{out: &buf})
	cmd.SetArgs([]string{"pubkey", tdata + "ed25519_enc", "--passphrase", "secretpass"})
	if err := cmd.Execute(); err != nil {
		t.Fatal(err)
	}
	if !strings.HasPrefix(strings.TrimSpace(buf.String()), "ssh-ed25519 ") {
		t.Errorf("should output public key line:\n%s", buf.String())
	}
}

func TestSSHKeyInfo_Stdin(t *testing.T) {
	var buf bytes.Buffer
	cmd := newSSHKeyCommand(sshKeyCmdDeps{
		out: &buf,
		in:  strings.NewReader("ssh-ed25519 AAAAC3NzaC1lZDI1NTE5AAAAIBacgxyVVKWrOOhXHfv9F3/L9pATfWSk4Uj17X6zoo4P stdin@jdan\n"),
	})
	cmd.SetArgs([]string{"info", "-"})
	if err := cmd.Execute(); err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(buf.String(), "Comment:      stdin@jdan") {
		t.Errorf("stdin input should parse:\n%s", buf.String())
	}
}

func TestSSHKeyInfo_InvalidKey(t *testing.T) {
	cmd := newSSHKeyCommand(sshKeyCmdDeps{
		out: &bytes.Buffer{},
		in:  strings.NewReader("this is not a key\n"),
	})
	cmd.SetArgs([]string{"info", "-"})
	if err := cmd.Execute(); err == nil {
		t.Error("invalid key should error")
	}
}
