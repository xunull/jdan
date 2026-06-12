package jwtdecode

import (
	"encoding/base64"
	"encoding/json"
	"fmt"
	"strings"
	"testing"
	"time"
)

// makeJWT 构造一个测试 JWT（不真的签名，signature 段用占位）。
func makeJWT(t *testing.T, header, payload map[string]any) string {
	t.Helper()
	hb, err := json.Marshal(header)
	if err != nil {
		t.Fatal(err)
	}
	pb, err := json.Marshal(payload)
	if err != nil {
		t.Fatal(err)
	}
	encode := func(b []byte) string {
		return base64.RawURLEncoding.EncodeToString(b)
	}
	return encode(hb) + "." + encode(pb) + ".test-signature"
}

func TestDecode_ValidToken(t *testing.T) {
	exp := time.Now().Add(1 * time.Hour).Unix()
	iat := time.Now().Add(-1 * time.Minute).Unix()
	token := makeJWT(t,
		map[string]any{"alg": "RS256", "typ": "JWT", "kid": "key-1"},
		map[string]any{
			"sub": "user-123",
			"iss": "https://issuer.example.com",
			"aud": "api.example.com",
			"iat": iat,
			"exp": exp,
		},
	)
	r, err := Decode(token)
	if err != nil {
		t.Fatal(err)
	}
	if r.Algorithm != "RS256" {
		t.Errorf("alg: got %q, want RS256", r.Algorithm)
	}
	if r.KeyID != "key-1" {
		t.Errorf("kid: got %q, want key-1", r.KeyID)
	}
	if r.Subject != "user-123" {
		t.Errorf("sub: got %q, want user-123", r.Subject)
	}
	if r.Issuer != "https://issuer.example.com" {
		t.Errorf("iss: got %q", r.Issuer)
	}
	if len(r.Audience) != 1 || r.Audience[0] != "api.example.com" {
		t.Errorf("aud: got %v", r.Audience)
	}
	if r.Expired {
		t.Error("token in future should not be expired")
	}
	if r.TimeRemaining == "" {
		t.Error("expected time remaining for unexpired token")
	}
	if r.Signature != "test-signature" {
		t.Errorf("signature segment lost: %q", r.Signature)
	}
}

func TestDecode_ExpiredToken(t *testing.T) {
	past := time.Now().Add(-1 * time.Hour).Unix()
	token := makeJWT(t,
		map[string]any{"alg": "HS256"},
		map[string]any{"exp": past, "sub": "old-user"},
	)
	r, err := Decode(token)
	if err != nil {
		t.Fatal(err)
	}
	if !r.Expired {
		t.Error("past-exp token should be marked expired")
	}
	if r.TimeRemaining != "" {
		t.Errorf("expired token should not have time remaining, got %q", r.TimeRemaining)
	}
}

func TestDecode_NoExpField(t *testing.T) {
	token := makeJWT(t,
		map[string]any{"alg": "none"},
		map[string]any{"sub": "no-expiry"},
	)
	r, err := Decode(token)
	if err != nil {
		t.Fatal(err)
	}
	if r.ExpiresAt != nil {
		t.Error("missing exp should yield nil ExpiresAt")
	}
	if r.Expired {
		t.Error("missing exp should not be expired")
	}
}

func TestDecode_AudienceArray(t *testing.T) {
	token := makeJWT(t,
		map[string]any{"alg": "RS256"},
		map[string]any{"aud": []any{"a.example.com", "b.example.com"}},
	)
	r, err := Decode(token)
	if err != nil {
		t.Fatal(err)
	}
	if len(r.Audience) != 2 {
		t.Fatalf("got %d audiences, want 2", len(r.Audience))
	}
	if r.Audience[0] != "a.example.com" || r.Audience[1] != "b.example.com" {
		t.Errorf("audience mismatch: %v", r.Audience)
	}
}

func TestDecode_RejectsNon3Segments(t *testing.T) {
	for _, bad := range []string{
		"abc",
		"abc.def",
		"a.b.c.d",
		"",
	} {
		if _, err := Decode(bad); err == nil {
			t.Errorf("Decode(%q) should error", bad)
		}
	}
}

func TestDecode_RejectsEmptySegments(t *testing.T) {
	for _, bad := range []string{
		".payload.sig",
		"header..sig",
	} {
		if _, err := Decode(bad); err == nil {
			t.Errorf("Decode(%q) should error", bad)
		}
	}
}

func TestDecode_RejectsInvalidBase64URL(t *testing.T) {
	// `+` and `/` are valid for standard base64 but NOT for base64url
	token := "abc+def.payload.sig"
	if _, err := Decode(token); err == nil {
		t.Error("token with + in header should error (not valid base64url)")
	}
}

func TestDecode_AcceptsPaddedSegments(t *testing.T) {
	// 严格 JWT spec 禁止 padding，但有些实现错误地带 `=`。我们应当容忍。
	header := map[string]any{"alg": "HS256"}
	payload := map[string]any{"sub": "test"}
	hb, _ := json.Marshal(header)
	pb, _ := json.Marshal(payload)
	// 用 std base64 url-safe + padding
	h := base64.URLEncoding.EncodeToString(hb)
	p := base64.URLEncoding.EncodeToString(pb)
	token := h + "." + p + ".sig"
	r, err := Decode(token)
	if err != nil {
		t.Errorf("padded base64url should be accepted: %v", err)
	}
	if r != nil && r.Algorithm != "HS256" {
		t.Errorf("alg lost: %q", r.Algorithm)
	}
}

func TestDecode_RejectsInvalidJSON(t *testing.T) {
	// header = base64url("not json")
	garbage := base64.RawURLEncoding.EncodeToString([]byte("not json"))
	token := garbage + "." + garbage + ".sig"
	if _, err := Decode(token); err == nil {
		t.Error("non-JSON header/payload should error")
	}
}

func TestDecode_HeaderPayloadPrettyPrint(t *testing.T) {
	token := makeJWT(t,
		map[string]any{"alg": "RS256", "typ": "JWT"},
		map[string]any{"sub": "abc"},
	)
	r, err := Decode(token)
	if err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(r.Header, "\n  \"alg\"") {
		t.Errorf("header not pretty-printed: %q", r.Header)
	}
	if !strings.Contains(r.Payload, "\n  \"sub\"") {
		t.Errorf("payload not pretty-printed: %q", r.Payload)
	}
}

func TestDecode_TrimsLeadingTrailingSpace(t *testing.T) {
	token := makeJWT(t,
		map[string]any{"alg": "HS256"},
		map[string]any{"sub": "x"},
	)
	r, err := Decode("  " + token + "\n\t")
	if err != nil {
		t.Errorf("whitespace should be trimmed: %v", err)
	}
	if r.Algorithm != "HS256" {
		t.Error("alg lost after trim")
	}
}

func TestParseEpoch_HandlesNonNumericGracefully(t *testing.T) {
	// String exp 应当被忽略而不是炸 panic
	token := makeJWT(t,
		map[string]any{"alg": "HS256"},
		map[string]any{"exp": "not-a-number"},
	)
	r, err := Decode(token)
	if err != nil {
		t.Fatal(err)
	}
	if r.ExpiresAt != nil {
		t.Error("non-numeric exp should yield nil ExpiresAt")
	}
}

func TestHumanDuration(t *testing.T) {
	for _, tc := range []struct {
		d    time.Duration
		want string
	}{
		{30 * time.Second, "30s"},
		{90 * time.Second, "1m"},
		{2 * time.Hour, "2h"},
		{2*time.Hour + 30*time.Minute, "2h 30m"},
		{25 * time.Hour, "1d 1h"},
		{72 * time.Hour, "3d"},
	} {
		got := humanDuration(tc.d)
		if got != tc.want {
			t.Errorf("humanDuration(%v) = %q, want %q", tc.d, got, tc.want)
		}
	}
}

func TestHumanDuration_NegativeIsExpired(t *testing.T) {
	if got := humanDuration(-1 * time.Hour); got != "expired" {
		t.Errorf("negative duration should be 'expired', got %q", got)
	}
}

// 真实世界的 RS256 token（jwt.io 示例 token，已 mask）
// 用以验证我们能解析"实际格式"的 token
func TestDecode_RealisticRS256Token(t *testing.T) {
	// header = {"alg":"RS256","typ":"JWT","kid":"abc"}
	// payload = {"sub":"1234567890","name":"John Doe","iat":1516239022}
	token := "eyJhbGciOiJSUzI1NiIsInR5cCI6IkpXVCIsImtpZCI6ImFiYyJ9." +
		"eyJzdWIiOiIxMjM0NTY3ODkwIiwibmFtZSI6IkpvaG4gRG9lIiwiaWF0IjoxNTE2MjM5MDIyfQ." +
		"signature-placeholder"
	r, err := Decode(token)
	if err != nil {
		t.Fatal(err)
	}
	if r.Algorithm != "RS256" {
		t.Errorf("alg: %q", r.Algorithm)
	}
	if r.KeyID != "abc" {
		t.Errorf("kid: %q", r.KeyID)
	}
	if r.Subject != "1234567890" {
		t.Errorf("sub: %q", r.Subject)
	}
	if r.IssuedAt == nil || r.IssuedAt.Year() != 2018 {
		// iat = 1516239022 = 2018-01-18
		t.Errorf("iat parse failed: %v", r.IssuedAt)
	}
	// 没 exp 字段就不应该被标记过期
	if r.Expired {
		t.Error("no exp claim ⇒ should not be marked expired")
	}
	if r.ExpiresAt != nil {
		t.Errorf("no exp claim ⇒ ExpiresAt should be nil, got %v", r.ExpiresAt)
	}
}

// 单独验证 enrichFromPayload 对 nbf 也生效
func TestDecode_NotBefore(t *testing.T) {
	future := time.Now().Add(1 * time.Hour).Unix()
	token := makeJWT(t,
		map[string]any{"alg": "HS256"},
		map[string]any{"nbf": future},
	)
	r, err := Decode(token)
	if err != nil {
		t.Fatal(err)
	}
	if r.NotBefore == nil {
		t.Error("nbf field not parsed")
	}
}

// JSON output 应当稳定可被脚本消费
func TestResult_JSONSerializable(t *testing.T) {
	token := makeJWT(t,
		map[string]any{"alg": "HS256"},
		map[string]any{"sub": "x"},
	)
	r, err := Decode(token)
	if err != nil {
		t.Fatal(err)
	}
	b, err := json.Marshal(r)
	if err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(string(b), `"alg":"HS256"`) {
		t.Errorf("JSON missing alg: %s", string(b))
	}
}

// signature 段不能被截断/丢失
func TestDecode_PreservesSignatureSegment(t *testing.T) {
	header := base64.RawURLEncoding.EncodeToString([]byte(`{"alg":"HS256"}`))
	payload := base64.RawURLEncoding.EncodeToString([]byte(`{"sub":"x"}`))
	sig := "very-secret-signature-bytes"
	token := fmt.Sprintf("%s.%s.%s", header, payload, sig)
	r, err := Decode(token)
	if err != nil {
		t.Fatal(err)
	}
	if r.Signature != sig {
		t.Errorf("signature segment changed: got %q, want %q", r.Signature, sig)
	}
}
