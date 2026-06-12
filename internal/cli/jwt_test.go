package cli

import (
	"bytes"
	"encoding/base64"
	"encoding/json"
	"strings"
	"testing"
	"time"
)

// 构造一个测试 JWT。
func mkTestJWT(t *testing.T, header, payload map[string]any) string {
	t.Helper()
	hb, _ := json.Marshal(header)
	pb, _ := json.Marshal(payload)
	return base64.RawURLEncoding.EncodeToString(hb) + "." +
		base64.RawURLEncoding.EncodeToString(pb) + ".testsig"
}

func TestJWTDecode_TextOutput(t *testing.T) {
	tok := mkTestJWT(t,
		map[string]any{"alg": "HS256", "typ": "JWT", "kid": "k1"},
		map[string]any{
			"sub": "alice",
			"iss": "issuer.example.com",
			"exp": time.Now().Add(1 * time.Hour).Unix(),
		},
	)
	var buf bytes.Buffer
	cmd := newJWTCommand(jwtCmdDeps{out: &buf})
	cmd.SetArgs([]string{"decode", tok})
	if err := cmd.Execute(); err != nil {
		t.Fatal(err)
	}
	out := buf.String()
	for _, want := range []string{
		"Header:",
		"Payload:",
		"算法: HS256",
		"Key ID: k1",
		"Subject: alice",
		"Issuer: issuer.example.com",
		"过期:",
		"剩余",
		"Signature: (present",
	} {
		if !strings.Contains(out, want) {
			t.Errorf("output missing %q:\n%s", want, out)
		}
	}
}

func TestJWTDecode_HeaderOnly(t *testing.T) {
	tok := mkTestJWT(t,
		map[string]any{"alg": "RS256"},
		map[string]any{"sub": "secret-payload"},
	)
	var buf bytes.Buffer
	cmd := newJWTCommand(jwtCmdDeps{out: &buf})
	cmd.SetArgs([]string{"decode", tok, "--header-only"})
	if err := cmd.Execute(); err != nil {
		t.Fatal(err)
	}
	out := buf.String()
	if !strings.Contains(out, "RS256") {
		t.Error("--header-only lost alg")
	}
	if strings.Contains(out, "secret-payload") {
		t.Error("--header-only leaked payload content")
	}
}

func TestJWTDecode_JSONOutput(t *testing.T) {
	tok := mkTestJWT(t,
		map[string]any{"alg": "HS256"},
		map[string]any{"sub": "json-test", "exp": time.Now().Add(time.Hour).Unix()},
	)
	var buf bytes.Buffer
	cmd := newJWTCommand(jwtCmdDeps{out: &buf})
	cmd.SetArgs([]string{"decode", tok, "--json"})
	if err := cmd.Execute(); err != nil {
		t.Fatal(err)
	}
	var got map[string]any
	if err := json.Unmarshal(buf.Bytes(), &got); err != nil {
		t.Fatalf("invalid JSON: %v\n%s", err, buf.String())
	}
	if got["alg"] != "HS256" {
		t.Errorf("alg: %v", got["alg"])
	}
	if got["sub"] != "json-test" {
		t.Errorf("sub: %v", got["sub"])
	}
	// signature 在 JSON 里必须保留
	if got["signature"] != "testsig" {
		t.Errorf("signature lost in JSON: %v", got["signature"])
	}
}

func TestJWTDecode_Raw(t *testing.T) {
	tok := mkTestJWT(t,
		map[string]any{"alg": "HS256"},
		map[string]any{"sub": "raw"},
	)
	var buf bytes.Buffer
	cmd := newJWTCommand(jwtCmdDeps{out: &buf})
	cmd.SetArgs([]string{"decode", tok, "--raw"})
	if err := cmd.Execute(); err != nil {
		t.Fatal(err)
	}
	out := buf.String()
	// raw 模式不应有 pretty-print 缩进
	if strings.Contains(out, "\n    \"alg\"") {
		t.Error("--raw should not pretty-print")
	}
	if !strings.Contains(out, `{"alg":"HS256"}`) {
		t.Errorf("--raw output missing compact header: %s", out)
	}
}

func TestJWTDecode_StdinInput(t *testing.T) {
	tok := mkTestJWT(t,
		map[string]any{"alg": "HS256"},
		map[string]any{"sub": "from-stdin"},
	)
	var buf bytes.Buffer
	cmd := newJWTCommand(jwtCmdDeps{
		out: &buf,
		in:  strings.NewReader(tok + "\n"),
	})
	cmd.SetArgs([]string{"decode"})
	if err := cmd.Execute(); err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(buf.String(), "from-stdin") {
		t.Error("stdin input not processed")
	}
}

func TestJWTDecode_NoInput_Errors(t *testing.T) {
	cmd := newJWTCommand(jwtCmdDeps{
		out: &bytes.Buffer{},
		in:  strings.NewReader(""),
	})
	cmd.SilenceUsage = true
	cmd.SilenceErrors = true
	cmd.SetArgs([]string{"decode"})
	if err := cmd.Execute(); err == nil {
		t.Error("empty input should error")
	}
}

func TestJWTDecode_InvalidToken_Errors(t *testing.T) {
	cmd := newJWTCommand(jwtCmdDeps{out: &bytes.Buffer{}})
	cmd.SilenceUsage = true
	cmd.SilenceErrors = true
	cmd.SetArgs([]string{"decode", "not.a.valid.token.really"})
	if err := cmd.Execute(); err == nil {
		t.Error("malformed token should error")
	}
}

func TestJWTDecode_ExpiredTokenShown(t *testing.T) {
	tok := mkTestJWT(t,
		map[string]any{"alg": "HS256"},
		map[string]any{"sub": "x", "exp": time.Now().Add(-1 * time.Hour).Unix()},
	)
	var buf bytes.Buffer
	cmd := newJWTCommand(jwtCmdDeps{out: &buf})
	cmd.SetArgs([]string{"decode", tok})
	if err := cmd.Execute(); err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(buf.String(), "已过期") {
		t.Errorf("expired token should be marked: %s", buf.String())
	}
}

func TestJWTDecode_NoSignatureLeakInText(t *testing.T) {
	// signature 内容不能出现在默认文本输出里（防误粘）
	tok := mkTestJWT(t,
		map[string]any{"alg": "HS256"},
		map[string]any{"sub": "x"},
	)
	var buf bytes.Buffer
	cmd := newJWTCommand(jwtCmdDeps{out: &buf})
	cmd.SetArgs([]string{"decode", tok})
	if err := cmd.Execute(); err != nil {
		t.Fatal(err)
	}
	out := buf.String()
	// 文本输出可以提到 "Signature:" 但不能包含原始 "testsig" 字符串
	if strings.Contains(out, "testsig") {
		t.Errorf("default text output leaked signature: %s", out)
	}
}

func TestJWTDecode_TrimsWhitespace(t *testing.T) {
	tok := mkTestJWT(t,
		map[string]any{"alg": "HS256"},
		map[string]any{"sub": "trim"},
	)
	var buf bytes.Buffer
	cmd := newJWTCommand(jwtCmdDeps{out: &buf})
	cmd.SetArgs([]string{"decode", "  " + tok + "\n\t"})
	if err := cmd.Execute(); err != nil {
		t.Errorf("token with whitespace should be accepted: %v", err)
	}
}

// jwt root command 自身（无 subcommand）应当显示帮助而不是炸
func TestJWT_RootShowsHelp(t *testing.T) {
	var buf bytes.Buffer
	cmd := newJWTCommand(jwtCmdDeps{out: &buf})
	cmd.SetOut(&buf)
	cmd.SetArgs([]string{})
	if err := cmd.Execute(); err != nil {
		t.Errorf("jwt root should show help, got error: %v", err)
	}
	if !strings.Contains(buf.String(), "decode") {
		t.Error("help should mention 'decode' subcommand")
	}
}
