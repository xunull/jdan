package cli

import (
	"bytes"
	"crypto/hmac"
	"crypto/sha256"
	"encoding/base64"
	"encoding/json"
	"strings"
	"testing"
)

// mkSignedJWT 造一个 HS256 真签名 token。
func mkSignedJWT(t *testing.T, secret string) string {
	t.Helper()
	hb, _ := json.Marshal(map[string]any{"alg": "HS256", "typ": "JWT"})
	pb, _ := json.Marshal(map[string]any{"sub": "12", "name": "bob"})
	enc := base64.RawURLEncoding
	signing := enc.EncodeToString(hb) + "." + enc.EncodeToString(pb)
	mac := hmac.New(sha256.New, []byte(secret))
	mac.Write([]byte(signing))
	return signing + "." + enc.EncodeToString(mac.Sum(nil))
}

func runJWTVerify(t *testing.T, stdin string, args ...string) (string, error) {
	t.Helper()
	var buf bytes.Buffer
	deps := jwtCmdDeps{out: &buf}
	if stdin != "" {
		deps.in = strings.NewReader(stdin)
	}
	cmd := newJWTCommand(deps)
	cmd.SetArgs(append([]string{"verify"}, args...))
	err := cmd.Execute()
	return buf.String(), err
}

func TestJWTVerifyCmd_Valid(t *testing.T) {
	tok := mkSignedJWT(t, "mykey")
	out, err := runJWTVerify(t, "", tok, "--secret", "mykey")
	if err != nil {
		t.Fatalf("valid token should not error: %v", err)
	}
	if !strings.Contains(out, "✓") || !strings.Contains(out, "HS256") {
		t.Errorf("expected valid mark: %q", out)
	}
}

func TestJWTVerifyCmd_Invalid(t *testing.T) {
	tok := mkSignedJWT(t, "mykey")
	out, err := runJWTVerify(t, "", tok, "--secret", "wrong")
	if err == nil {
		t.Error("invalid signature should error (exit 1)")
	}
	if !strings.Contains(out, "✗") {
		t.Errorf("expected invalid mark in output: %q", out)
	}
}

func TestJWTVerifyCmd_JSON(t *testing.T) {
	tok := mkSignedJWT(t, "mykey")
	out, err := runJWTVerify(t, "", tok, "--secret", "mykey", "--json")
	if err != nil {
		t.Fatal(err)
	}
	var v struct {
		Alg   string `json:"alg"`
		Valid bool   `json:"valid"`
	}
	if err := json.Unmarshal([]byte(out), &v); err != nil {
		t.Fatalf("--json should emit valid JSON: %v\n%s", err, out)
	}
	if v.Alg != "HS256" || !v.Valid {
		t.Errorf("got %+v", v)
	}
}

func TestJWTVerifyCmd_Stdin(t *testing.T) {
	tok := mkSignedJWT(t, "mykey")
	out, err := runJWTVerify(t, "Bearer "+tok, "--secret", "mykey")
	if err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(out, "✓") {
		t.Errorf("stdin + Bearer should verify: %q", out)
	}
}

func TestJWTVerifyCmd_MissingSecret(t *testing.T) {
	tok := mkSignedJWT(t, "mykey")
	if _, err := runJWTVerify(t, "", tok); err == nil {
		t.Error("missing --secret should error")
	}
}
