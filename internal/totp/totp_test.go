package totp

import (
	"encoding/base32"
	"testing"
)

// ---- RFC 4226 Appendix D: HOTP 测试向量 ----
// secret = "12345678901234567890" (ASCII), counter 0..9 的标准 6 位码。

func TestHOTP_RFC4226Vectors(t *testing.T) {
	key := []byte("12345678901234567890")
	want := []string{
		"755224", "287082", "359152", "969429", "338314",
		"254676", "287922", "162583", "399871", "520489",
	}
	for counter, w := range want {
		got := HOTP(key, uint64(counter), 6, AlgoSHA1)
		if got != w {
			t.Errorf("HOTP counter=%d = %s, want %s", counter, got, w)
		}
	}
}

// ---- RFC 6238 Appendix B: TOTP 测试向量 ----
// 不同算法用不同长度的 ASCII seed；8 位码。

const (
	seedSHA1   = "12345678901234567890"
	seedSHA256 = "12345678901234567890123456789012"
	seedSHA512 = "1234567890123456789012345678901234567890123456789012345678901234"
)

func TestTOTP_RFC6238_SHA1(t *testing.T) {
	cfg := Config{Digits: 8, Period: 30, Algorithm: AlgoSHA1}
	cases := []struct {
		time int64
		want string
	}{
		{59, "94287082"},
		{1111111109, "07081804"},
		{1111111111, "14050471"},
		{1234567890, "89005924"},
		{2000000000, "69279037"},
		{20000000000, "65353130"},
	}
	for _, c := range cases {
		got := GenerateAt([]byte(seedSHA1), c.time, cfg)
		if got != c.want {
			t.Errorf("TOTP SHA1 t=%d = %s, want %s", c.time, got, c.want)
		}
	}
}

func TestTOTP_RFC6238_SHA256(t *testing.T) {
	cfg := Config{Digits: 8, Period: 30, Algorithm: AlgoSHA256}
	cases := []struct {
		time int64
		want string
	}{
		{59, "46119246"},
		{1111111109, "68084774"},
		{1234567890, "91819424"},
		{20000000000, "77737706"},
	}
	for _, c := range cases {
		got := GenerateAt([]byte(seedSHA256), c.time, cfg)
		if got != c.want {
			t.Errorf("TOTP SHA256 t=%d = %s, want %s", c.time, got, c.want)
		}
	}
}

func TestTOTP_RFC6238_SHA512(t *testing.T) {
	cfg := Config{Digits: 8, Period: 30, Algorithm: AlgoSHA512}
	cases := []struct {
		time int64
		want string
	}{
		{59, "90693936"},
		{1111111109, "25091201"},
		{1234567890, "93441116"},
		{20000000000, "47863826"},
	}
	for _, c := range cases {
		got := GenerateAt([]byte(seedSHA512), c.time, cfg)
		if got != c.want {
			t.Errorf("TOTP SHA512 t=%d = %s, want %s", c.time, got, c.want)
		}
	}
}

// ---- ExpiresInAt ----

func TestExpiresInAt(t *testing.T) {
	cfg := DefaultConfig() // period 30
	// t=0 → 整窗开始，剩 30
	if got := ExpiresInAt(0, cfg); got != 30 {
		t.Errorf("t=0 expires_in = %d, want 30", got)
	}
	// t=29 → 剩 1
	if got := ExpiresInAt(29, cfg); got != 1 {
		t.Errorf("t=29 expires_in = %d, want 1", got)
	}
	// t=45 → 窗口 [30,60)，剩 15
	if got := ExpiresInAt(45, cfg); got != 15 {
		t.Errorf("t=45 expires_in = %d, want 15", got)
	}
}

// ---- VerifyAt 时间窗 ----

func TestVerifyAt_CurrentWindow(t *testing.T) {
	key := []byte(seedSHA1)
	cfg := DefaultConfig()
	code := GenerateAt(key, 1000, cfg)
	if !VerifyAt(key, code, 1000, 1, cfg) {
		t.Error("current window code should verify")
	}
}

func TestVerifyAt_AdjacentWindow(t *testing.T) {
	key := []byte(seedSHA1)
	cfg := DefaultConfig() // period 30
	// 上一窗生成的码，在 window=1 时仍应有效（时钟漂移容错）
	prevCode := GenerateAt(key, 1000, cfg)
	if !VerifyAt(key, prevCode, 1035, 1, cfg) { // 1035 在下一窗
		t.Error("previous window code should verify with window=1")
	}
	// window=0 时不接受相邻窗
	if VerifyAt(key, prevCode, 1035, 0, cfg) {
		t.Error("previous window code should NOT verify with window=0")
	}
}

func TestVerifyAt_WrongCode(t *testing.T) {
	key := []byte(seedSHA1)
	if VerifyAt(key, "000000", 1000, 1, DefaultConfig()) {
		t.Error("wrong code should not verify (unless astronomically lucky)")
	}
}

func TestVerifyAt_LengthMismatch(t *testing.T) {
	key := []byte(seedSHA1)
	// 8 位码不该匹配 6 位配置
	if VerifyAt(key, "12345678", 1000, 1, DefaultConfig()) {
		t.Error("length mismatch should not verify")
	}
}

// ---- DecodeSecret 容错 ----

func TestDecodeSecret_Basic(t *testing.T) {
	// "Hello!" 的 base32 是 "JBSWY3DPEHPK3PXP" 不对，用已知 vector
	// RFC 4648: base32("foobar") = "MZXW6YTBOI======"
	got, err := DecodeSecret("MZXW6YTBOI======")
	if err != nil {
		t.Fatal(err)
	}
	if string(got) != "foobar" {
		t.Errorf("got %q, want foobar", got)
	}
}

func TestDecodeSecret_Lowercase(t *testing.T) {
	got, err := DecodeSecret("mzxw6ytboi")
	if err != nil {
		t.Fatal(err)
	}
	if string(got) != "foobar" {
		t.Errorf("lowercase: got %q", got)
	}
}

func TestDecodeSecret_SpacesAndNoPadding(t *testing.T) {
	// Google 显示格式：空格分组 + 无 padding
	got, err := DecodeSecret("MZXW 6YTB OI")
	if err != nil {
		t.Fatal(err)
	}
	if string(got) != "foobar" {
		t.Errorf("spaced: got %q", got)
	}
}

func TestDecodeSecret_Empty(t *testing.T) {
	if _, err := DecodeSecret("   "); err == nil {
		t.Error("empty secret should error")
	}
}

func TestDecodeSecret_Invalid(t *testing.T) {
	if _, err := DecodeSecret("0189!@#"); err == nil {
		t.Error("invalid base32 (0,1,8,9 not in alphabet) should error")
	}
}

func TestDecodeSecret_RoundTrip(t *testing.T) {
	orig := []byte("super secret key")
	enc := base32.StdEncoding.EncodeToString(orig)
	got, err := DecodeSecret(enc)
	if err != nil {
		t.Fatal(err)
	}
	if string(got) != string(orig) {
		t.Errorf("round-trip failed: %q", got)
	}
}

// ---- ParseAlgorithm ----

func TestParseAlgorithm(t *testing.T) {
	cases := map[string]Algorithm{
		"":       AlgoSHA1,
		"sha1":   AlgoSHA1,
		"SHA1":   AlgoSHA1,
		"sha256": AlgoSHA256,
		"SHA512": AlgoSHA512,
	}
	for in, want := range cases {
		got, err := ParseAlgorithm(in)
		if err != nil {
			t.Errorf("ParseAlgorithm(%q) errored: %v", in, err)
			continue
		}
		if got != want {
			t.Errorf("ParseAlgorithm(%q) = %s, want %s", in, got, want)
		}
	}
	if _, err := ParseAlgorithm("md5"); err == nil {
		t.Error("md5 should be unsupported")
	}
}

// ---- ParseOtpauthURI ----

func TestParseOtpauthURI_Full(t *testing.T) {
	uri := "otpauth://totp/GitHub:quincy?secret=MZXW6YTBOI&issuer=GitHub&algorithm=SHA256&digits=8&period=60"
	p, err := ParseOtpauthURI(uri)
	if err != nil {
		t.Fatal(err)
	}
	if p.Issuer != "GitHub" {
		t.Errorf("issuer = %q", p.Issuer)
	}
	if p.Account != "quincy" {
		t.Errorf("account = %q", p.Account)
	}
	if p.Secret != "MZXW6YTBOI" {
		t.Errorf("secret = %q", p.Secret)
	}
	if p.Algorithm != AlgoSHA256 {
		t.Errorf("algorithm = %s", p.Algorithm)
	}
	if p.Digits != 8 {
		t.Errorf("digits = %d", p.Digits)
	}
	if p.Period != 60 {
		t.Errorf("period = %d", p.Period)
	}
}

func TestParseOtpauthURI_Defaults(t *testing.T) {
	uri := "otpauth://totp/Acme:bob?secret=MZXW6YTBOI"
	p, err := ParseOtpauthURI(uri)
	if err != nil {
		t.Fatal(err)
	}
	cfg := p.Config()
	if cfg.Algorithm != AlgoSHA1 || cfg.Digits != 6 || cfg.Period != 30 {
		t.Errorf("defaults wrong: %+v", cfg)
	}
}

func TestParseOtpauthURI_IssuerFromLabelOnly(t *testing.T) {
	uri := "otpauth://totp/MyService:alice?secret=MZXW6YTBOI"
	p, _ := ParseOtpauthURI(uri)
	if p.Issuer != "MyService" {
		t.Errorf("issuer from label = %q", p.Issuer)
	}
}

func TestParseOtpauthURI_MissingSecret(t *testing.T) {
	if _, err := ParseOtpauthURI("otpauth://totp/Acme:bob?issuer=Acme"); err == nil {
		t.Error("missing secret should error")
	}
}

func TestParseOtpauthURI_WrongScheme(t *testing.T) {
	if _, err := ParseOtpauthURI("https://example.com"); err == nil {
		t.Error("non-otpauth scheme should error")
	}
}

func TestParseOtpauthURI_HOTPRejected(t *testing.T) {
	if _, err := ParseOtpauthURI("otpauth://hotp/Acme:bob?secret=MZXW6YTBOI&counter=0"); err == nil {
		t.Error("hotp type should be rejected")
	}
}
