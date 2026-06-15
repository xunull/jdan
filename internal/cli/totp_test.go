package cli

import (
	"bytes"
	"strings"
	"testing"
	"time"
)

// RFC 6238 SHA1 seed "12345678901234567890" 的 base32 编码。
const rfcSeedB32 = "GEZDGNBVGY3TQOJQGEZDGNBVGY3TQOJQ"

// fixedNow 返回固定时间，让 TOTP 码可预测。
func fixedNow(unix int64) func() time.Time {
	return func() time.Time { return time.Unix(unix, 0) }
}

func TestTOTPCode_RFCVector(t *testing.T) {
	var buf bytes.Buffer
	// t=59，6 位码 = RFC 8 位码 94287082 的后 6 位 = 287082
	cmd := newTOTPCommand(totpCmdDeps{out: &buf, now: fixedNow(59)})
	cmd.SetArgs([]string{"code", rfcSeedB32})
	if err := cmd.Execute(); err != nil {
		t.Fatal(err)
	}
	if !strings.HasPrefix(buf.String(), "287082") {
		t.Errorf("got %q, want code 287082", buf.String())
	}
}

func TestTOTPCode_8Digit_RFCVector(t *testing.T) {
	var buf bytes.Buffer
	cmd := newTOTPCommand(totpCmdDeps{out: &buf, now: fixedNow(59)})
	cmd.SetArgs([]string{"code", rfcSeedB32, "--digits", "8"})
	if err := cmd.Execute(); err != nil {
		t.Fatal(err)
	}
	if !strings.HasPrefix(buf.String(), "94287082") {
		t.Errorf("got %q, want 94287082", buf.String())
	}
}

func TestTOTPCode_SHA256_RFCVector(t *testing.T) {
	const seed256 = "GEZDGNBVGY3TQOJQGEZDGNBVGY3TQOJQGEZDGNBVGY3TQOJQGEZA" // 32-byte ASCII seed in base32
	var buf bytes.Buffer
	cmd := newTOTPCommand(totpCmdDeps{out: &buf, now: fixedNow(59)})
	cmd.SetArgs([]string{"code", seed256, "--algo", "sha256", "--digits", "8"})
	if err := cmd.Execute(); err != nil {
		t.Fatal(err)
	}
	if !strings.HasPrefix(buf.String(), "46119246") {
		t.Errorf("got %q, want 46119246", buf.String())
	}
}

func TestTOTPCode_JSON(t *testing.T) {
	var buf bytes.Buffer
	cmd := newTOTPCommand(totpCmdDeps{out: &buf, now: fixedNow(0)})
	cmd.SetArgs([]string{"code", rfcSeedB32, "--json"})
	if err := cmd.Execute(); err != nil {
		t.Fatal(err)
	}
	out := buf.String()
	for _, want := range []string{
		`"code": "755224"`, // counter 0 = HOTP RFC vector
		`"period": 30`,
		`"digits": 6`,
		`"expires_in": 30`,
	} {
		if !strings.Contains(out, want) {
			t.Errorf("missing %q in:\n%s", want, out)
		}
	}
}

func TestTOTPCode_EnvSecret(t *testing.T) {
	var buf bytes.Buffer
	cmd := newTOTPCommand(totpCmdDeps{
		out:    &buf,
		now:    fixedNow(59),
		getenv: func(k string) string { return rfcSeedB32 },
	})
	cmd.SetArgs([]string{"code"})
	if err := cmd.Execute(); err != nil {
		t.Fatal(err)
	}
	if !strings.HasPrefix(buf.String(), "287082") {
		t.Errorf("env secret: got %q", buf.String())
	}
}

func TestTOTPCode_StdinSecret(t *testing.T) {
	var buf bytes.Buffer
	cmd := newTOTPCommand(totpCmdDeps{
		out:    &buf,
		in:     strings.NewReader(rfcSeedB32 + "\n"),
		now:    fixedNow(59),
		getenv: func(string) string { return "" },
	})
	cmd.SetArgs([]string{"code"})
	if err := cmd.Execute(); err != nil {
		t.Fatal(err)
	}
	if !strings.HasPrefix(buf.String(), "287082") {
		t.Errorf("stdin secret: got %q", buf.String())
	}
}

func TestTOTPCode_NoSecret_Errors(t *testing.T) {
	cmd := newTOTPCommand(totpCmdDeps{
		out:    &bytes.Buffer{},
		in:     strings.NewReader(""),
		getenv: func(string) string { return "" },
	})
	cmd.SetArgs([]string{"code"})
	if err := cmd.Execute(); err == nil {
		t.Error("no secret should error")
	}
}

func TestTOTPCode_InvalidSecret_Errors(t *testing.T) {
	cmd := newTOTPCommand(totpCmdDeps{out: &bytes.Buffer{}, now: fixedNow(0)})
	cmd.SetArgs([]string{"code", "0189!!!"})
	if err := cmd.Execute(); err == nil {
		t.Error("invalid base32 should error")
	}
}

func TestTOTPURI_Full(t *testing.T) {
	var buf bytes.Buffer
	cmd := newTOTPCommand(totpCmdDeps{out: &buf, now: fixedNow(59)})
	uri := "otpauth://totp/GitHub:quincy?secret=" + rfcSeedB32 + "&issuer=GitHub&digits=6&period=30"
	cmd.SetArgs([]string{"uri", uri})
	if err := cmd.Execute(); err != nil {
		t.Fatal(err)
	}
	out := buf.String()
	for _, want := range []string{
		"Issuer:    GitHub",
		"Account:   quincy",
		"Algorithm: SHA1",
		"Code:      287082",
	} {
		if !strings.Contains(out, want) {
			t.Errorf("missing %q in:\n%s", want, out)
		}
	}
}

func TestTOTPURI_JSON(t *testing.T) {
	var buf bytes.Buffer
	cmd := newTOTPCommand(totpCmdDeps{out: &buf, now: fixedNow(59)})
	uri := "otpauth://totp/Acme:bob?secret=" + rfcSeedB32
	cmd.SetArgs([]string{"uri", uri, "--json"})
	if err := cmd.Execute(); err != nil {
		t.Fatal(err)
	}
	out := buf.String()
	for _, want := range []string{
		`"issuer": "Acme"`,
		`"account": "bob"`,
		`"code": "287082"`,
	} {
		if !strings.Contains(out, want) {
			t.Errorf("missing %q in:\n%s", want, out)
		}
	}
}

func TestTOTPURI_Invalid(t *testing.T) {
	cmd := newTOTPCommand(totpCmdDeps{out: &bytes.Buffer{}, now: fixedNow(0)})
	cmd.SetArgs([]string{"uri", "https://example.com"})
	if err := cmd.Execute(); err == nil {
		t.Error("non-otpauth URI should error")
	}
}

func TestTOTPVerify_Valid(t *testing.T) {
	var buf bytes.Buffer
	cmd := newTOTPCommand(totpCmdDeps{out: &buf, now: fixedNow(59)})
	// t=59 → 287082
	cmd.SetArgs([]string{"verify", rfcSeedB32, "287082"})
	if err := cmd.Execute(); err != nil {
		t.Fatalf("valid code should not error: %v", err)
	}
	if !strings.Contains(buf.String(), "valid") {
		t.Errorf("got %q", buf.String())
	}
}

func TestTOTPVerify_Invalid(t *testing.T) {
	var buf bytes.Buffer
	cmd := newTOTPCommand(totpCmdDeps{out: &buf, now: fixedNow(59)})
	cmd.SetArgs([]string{"verify", rfcSeedB32, "000000"})
	err := cmd.Execute()
	if err == nil {
		t.Error("invalid code should return non-nil error (exit 1)")
	}
	if _, ok := err.(*totpVerifyExitErr); !ok {
		t.Errorf("expected *totpVerifyExitErr, got %T", err)
	}
	if !strings.Contains(buf.String(), "invalid") {
		t.Errorf("got %q", buf.String())
	}
}

func TestTOTPVerify_WindowTolerance(t *testing.T) {
	var buf bytes.Buffer
	// 上一窗 t=59 的码 287082，在 t=89（下一窗）用 window=1 仍有效
	cmd := newTOTPCommand(totpCmdDeps{out: &buf, now: fixedNow(89)})
	cmd.SetArgs([]string{"verify", rfcSeedB32, "287082", "--window", "1"})
	if err := cmd.Execute(); err != nil {
		t.Errorf("adjacent window should verify with --window 1: %v", err)
	}
}

func TestTOTPVerify_WindowZeroRejectsAdjacent(t *testing.T) {
	cmd := newTOTPCommand(totpCmdDeps{out: &bytes.Buffer{}, now: fixedNow(89)})
	cmd.SetArgs([]string{"verify", rfcSeedB32, "287082", "--window", "0"})
	if err := cmd.Execute(); err == nil {
		t.Error("adjacent window should be rejected with --window 0")
	}
}
