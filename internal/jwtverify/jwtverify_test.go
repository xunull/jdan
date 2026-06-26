package jwtverify

import (
	"crypto/hmac"
	"crypto/sha256"
	"crypto/sha512"
	"encoding/base64"
	"encoding/json"
	"hash"
	"strings"
	"testing"
)

// signedToken 用给定 alg + secret 造一个真签名 token。
func signedToken(t *testing.T, alg string, secret []byte) string {
	t.Helper()
	hb, _ := json.Marshal(map[string]any{"alg": alg, "typ": "JWT"})
	pb, _ := json.Marshal(map[string]any{"sub": "12", "name": "bob"})
	enc := base64.RawURLEncoding
	signing := enc.EncodeToString(hb) + "." + enc.EncodeToString(pb)

	var h func() hash.Hash
	switch alg {
	case "HS256":
		h = sha256.New
	case "HS384":
		h = sha512.New384
	case "HS512":
		h = sha512.New
	default:
		// 非 HMAC：放个假签名，结构合法即可
		return signing + "." + enc.EncodeToString([]byte("fakesig"))
	}
	mac := hmac.New(h, secret)
	mac.Write([]byte(signing))
	return signing + "." + enc.EncodeToString(mac.Sum(nil))
}

func TestVerify_HS256_Valid(t *testing.T) {
	tok := signedToken(t, "HS256", []byte("mykey"))
	alg, ok, err := Verify(tok, []byte("mykey"))
	if err != nil {
		t.Fatal(err)
	}
	if alg != "HS256" || !ok {
		t.Errorf("alg=%s ok=%v, want HS256 true", alg, ok)
	}
}

func TestVerify_HS256_WrongSecret(t *testing.T) {
	tok := signedToken(t, "HS256", []byte("mykey"))
	_, ok, err := Verify(tok, []byte("wrong"))
	if err != nil {
		t.Fatal(err)
	}
	if ok {
		t.Error("wrong secret should not verify")
	}
}

func TestVerify_HS384(t *testing.T) {
	tok := signedToken(t, "HS384", []byte("k"))
	if _, ok, err := Verify(tok, []byte("k")); err != nil || !ok {
		t.Errorf("HS384 valid: ok=%v err=%v", ok, err)
	}
}

func TestVerify_HS512(t *testing.T) {
	tok := signedToken(t, "HS512", []byte("k"))
	if _, ok, err := Verify(tok, []byte("k")); err != nil || !ok {
		t.Errorf("HS512 valid: ok=%v err=%v", ok, err)
	}
}

func TestVerify_AlgNone_Error(t *testing.T) {
	tok := signedToken(t, "none", nil)
	if _, _, err := Verify(tok, []byte("k")); err == nil {
		t.Error("alg=none should error (nothing to verify)")
	}
}

func TestVerify_RS256_Rejected(t *testing.T) {
	// 关键安全测试：RS256 token + HMAC secret 必须报错，绝不当 HMAC 验（防 alg-confusion）
	tok := signedToken(t, "RS256", []byte("k"))
	_, ok, err := Verify(tok, []byte("k"))
	if err == nil {
		t.Fatal("RS256 + secret should be rejected, not HMAC-verified")
	}
	if ok {
		t.Error("RS256 must never report valid via HMAC path")
	}
	if !strings.Contains(err.Error(), "公钥") {
		t.Errorf("error should explain RS needs a public key: %v", err)
	}
}

func TestVerify_TamperedPayload(t *testing.T) {
	tok := signedToken(t, "HS256", []byte("k"))
	parts := strings.Split(tok, ".")
	// 篡改 payload：换成另一个合法 base64url 段
	parts[1] = base64.RawURLEncoding.EncodeToString([]byte(`{"sub":"evil"}`))
	tampered := strings.Join(parts, ".")
	if _, ok, err := Verify(tampered, []byte("k")); err != nil || ok {
		t.Errorf("tampered payload should fail verification: ok=%v err=%v", ok, err)
	}
}

func TestVerify_BearerPrefix(t *testing.T) {
	tok := signedToken(t, "HS256", []byte("k"))
	if _, ok, err := Verify("Bearer "+tok, []byte("k")); err != nil || !ok {
		t.Errorf("Bearer-prefixed token should verify: ok=%v err=%v", ok, err)
	}
}

func TestVerify_Malformed(t *testing.T) {
	if _, _, err := Verify("not.a.jwt.at.all", []byte("k")); err == nil {
		t.Error("malformed token should error")
	}
}
