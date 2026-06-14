package sslscan

import (
	"context"
	"crypto/tls"
	"net"
	"net/http"
	"net/http/httptest"
	"net/url"
	"strconv"
	"strings"
	"testing"
	"time"
)

// startTLSServer 起一个 httptest TLS server 带可配置 TLSConfig 修改器。
func startTLSServer(t *testing.T, configure func(*tls.Config), hstsHeader string) (*httptest.Server, string, int) {
	t.Helper()
	ts := httptest.NewUnstartedServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if hstsHeader != "" {
			w.Header().Set("Strict-Transport-Security", hstsHeader)
		}
		w.WriteHeader(200)
	}))
	ts.EnableHTTP2 = false
	ts.StartTLS()

	if configure != nil {
		// httptest 已经 start，无法重新 set Config；改用 manual server below in tests that need it
	}
	u, _ := url.Parse(ts.URL)
	host, portStr, _ := net.SplitHostPort(u.Host)
	port, _ := strconv.Atoi(portStr)
	return ts, host, port
}

func TestScanVersions_RealHTTPTestServer(t *testing.T) {
	ts, host, port := startTLSServer(t, nil, "")
	defer ts.Close()

	out := scanVersions(context.Background(), Options{Host: host, Port: port, SNI: host})
	if len(out.Results) != 4 {
		t.Fatalf("got %d version results, want 4", len(out.Results))
	}
	// httptest server 应当至少支持 TLS 1.2 或 1.3
	supported := out.SupportedSet()
	if !supported[tls.VersionTLS12] && !supported[tls.VersionTLS13] {
		t.Errorf("httptest server should support TLS 1.2 or 1.3; supported=%v", supported)
	}
}

func TestScanCiphers_DetectsAtLeastOne(t *testing.T) {
	ts, host, port := startTLSServer(t, nil, "")
	defer ts.Close()

	out := scanCiphers(context.Background(), Options{Host: host, Port: port, SNI: host})
	if len(out.TLS12) != len(commonCipherSuites) {
		t.Errorf("expected %d cipher results, got %d", len(commonCipherSuites), len(out.TLS12))
	}
	// 至少一个 strong cipher 应当被支持
	if out.SupportedStrong() == 0 {
		// httptest 自带 cipher 应当包含 strong；如果 0 说明探测路径有问题
		t.Errorf("expected at least one strong cipher supported")
	}
}

func TestScanCiphers_FullList(t *testing.T) {
	ts, host, port := startTLSServer(t, nil, "")
	defer ts.Close()

	out := scanCiphers(context.Background(), Options{
		Host: host, Port: port, SNI: host, FullCipher: true,
	})
	if len(out.TLS12) != len(fullCipherSuites) {
		t.Errorf("--full-cipher should expand to %d, got %d", len(fullCipherSuites), len(out.TLS12))
	}
}

func TestScanALPN_NoH2(t *testing.T) {
	// httptest with EnableHTTP2=false 不应宣称 h2
	ts, host, port := startTLSServer(t, nil, "")
	defer ts.Close()
	out := scanALPN(context.Background(), Options{Host: host, Port: port, SNI: host})
	if out.HasH2() {
		t.Error("EnableHTTP2=false server should not negotiate h2")
	}
}

func TestScanHSTS_NoHeader(t *testing.T) {
	ts, host, port := startTLSServer(t, nil, "")
	defer ts.Close()
	out := scanHSTS(context.Background(), Options{Host: host, Port: port, SNI: host})
	if out == nil {
		t.Fatal("nil section")
	}
	if out.Present {
		t.Errorf("no header should yield Present=false; got %+v", out)
	}
}

func TestScanHSTS_WithPreload(t *testing.T) {
	ts, host, port := startTLSServer(t, nil, "max-age=63072000; includeSubDomains; preload")
	defer ts.Close()
	out := scanHSTS(context.Background(), Options{Host: host, Port: port, SNI: host})
	if !out.Present {
		t.Fatal("expected HSTS present")
	}
	if !out.IncludeSubDomains {
		t.Error("includeSubDomains should be detected")
	}
	if !out.Preload {
		t.Error("preload should be detected")
	}
	if out.MaxAge != 63072000 {
		t.Errorf("max-age = %d, want 63072000", out.MaxAge)
	}
	if got := out.Strength(); got != "preload" {
		t.Errorf("Strength = %q, want preload", got)
	}
}

func TestParseHSTS(t *testing.T) {
	for _, tc := range []struct {
		header string
		want   HSTSSection
	}{
		{
			"max-age=31536000",
			HSTSSection{MaxAge: 31536000},
		},
		{
			"max-age=63072000; includeSubDomains",
			HSTSSection{MaxAge: 63072000, IncludeSubDomains: true},
		},
		{
			"max-age=10; includeSubDomains; preload",
			HSTSSection{MaxAge: 10, IncludeSubDomains: true, Preload: true},
		},
		{
			`max-age="31536000"; preload`,
			HSTSSection{MaxAge: 31536000, Preload: true},
		},
	} {
		got := HSTSSection{}
		parseHSTS(tc.header, &got)
		if got.MaxAge != tc.want.MaxAge ||
			got.IncludeSubDomains != tc.want.IncludeSubDomains ||
			got.Preload != tc.want.Preload {
			t.Errorf("parseHSTS(%q) = %+v, want %+v", tc.header, got, tc.want)
		}
	}
}

func TestHSTSStrength(t *testing.T) {
	for _, tc := range []struct {
		s    HSTSSection
		want string
	}{
		{HSTSSection{Present: false}, "none"},
		{HSTSSection{Present: true, MaxAge: 100}, "weak"},
		{HSTSSection{Present: true, MaxAge: 31536000}, "good"},
		{HSTSSection{Present: true, MaxAge: 31536000, IncludeSubDomains: true, Preload: true}, "preload"},
	} {
		got := tc.s.Strength()
		if got != tc.want {
			t.Errorf("Strength(%+v) = %q, want %q", tc.s, got, tc.want)
		}
	}
}

func TestClassifyCipher(t *testing.T) {
	for _, tc := range []struct {
		id   uint16
		want CipherStrength
	}{
		{tls.TLS_ECDHE_RSA_WITH_AES_256_GCM_SHA384, StrengthStrong},
		{tls.TLS_ECDHE_RSA_WITH_CHACHA20_POLY1305, StrengthStrong},
		{tls.TLS_ECDHE_RSA_WITH_AES_256_CBC_SHA, StrengthAcceptable},
		{tls.TLS_RSA_WITH_AES_256_GCM_SHA384, StrengthAcceptable}, // 无 ECDHE 但算法强
		{tls.TLS_RSA_WITH_3DES_EDE_CBC_SHA, StrengthWeak},
	} {
		got := classifyCipher(tc.id)
		if got != tc.want {
			t.Errorf("classifyCipher(%s) = %s, want %s",
				tls.CipherSuiteName(tc.id), got, tc.want)
		}
	}
}

func TestGradeMapping(t *testing.T) {
	for _, tc := range []struct {
		score      int
		wantLetter string
	}{
		{100, "A+"},
		{90, "A+"},
		{89, "A"},
		{80, "A"},
		{79, "B"},
		{65, "B"},
		{64, "C"},
		{50, "C"},
		{49, "D"},
		{35, "D"},
		{34, "F"},
		{0, "F"},
	} {
		got := scoreToLetter(tc.score)
		if got != tc.wantLetter {
			t.Errorf("scoreToLetter(%d) = %q, want %q", tc.score, got, tc.wantLetter)
		}
	}
}

func TestGrade_PerfectScore(t *testing.T) {
	r := &ScanReport{
		Cert: &CertSection{Trusted: true, HostnameOK: true, DaysLeft: 60, KeySizeBits: 2048},
		Versions: VersionsSection{Results: []VersionResult{
			{Version: "TLS 1.0", Supported: false},
			{Version: "TLS 1.1", Supported: false},
			{Version: "TLS 1.2", Supported: true},
			{Version: "TLS 1.3", Supported: true},
		}},
		Ciphers: CiphersSection{TLS12: []CipherResult{
			{Name: tls.CipherSuiteName(tls.TLS_ECDHE_RSA_WITH_AES_256_GCM_SHA384),
				Supported: true, Strength: StrengthStrong},
		}},
		ALPN:   ALPNSection{Supported: []string{"h2", "http/1.1"}},
		HSTS:   &HSTSSection{Present: true, MaxAge: 63072000, IncludeSubDomains: true, Preload: true},
		Resume: ResumeSection{TLS13PSKSupported: true},
	}
	g := computeGrade(r)
	if g.Letter != "A+" {
		t.Errorf("perfect-config score = %d (%s), want A+", g.Score, g.Letter)
	}
	if len(g.Strengths) == 0 {
		t.Error("perfect config should have Strengths")
	}
}

func TestGrade_FailsForTLS10Only(t *testing.T) {
	r := &ScanReport{
		Cert: &CertSection{Trusted: true, HostnameOK: true, DaysLeft: 60, KeySizeBits: 2048},
		Versions: VersionsSection{Results: []VersionResult{
			{Version: "TLS 1.0", Supported: true},
			{Version: "TLS 1.1", Supported: true},
			{Version: "TLS 1.2", Supported: false},
			{Version: "TLS 1.3", Supported: false},
		}},
		Ciphers: CiphersSection{TLS12: []CipherResult{
			{Name: "TLS_RSA_WITH_3DES_EDE_CBC_SHA", Supported: true, Strength: StrengthWeak},
		}},
	}
	g := computeGrade(r)
	if g.Letter == "A+" || g.Letter == "A" {
		t.Errorf("TLS 1.0/1.1-only with weak cipher should not get A: got %s (%d)", g.Letter, g.Score)
	}
	if !g.IsFailing() {
		t.Error("TLS 1.0/1.1-only should be IsFailing=true")
	}
}

func TestKeyBitsFromAlgo(t *testing.T) {
	for _, tc := range []struct {
		in   string
		want int
	}{
		{"RSA 2048", 2048},
		{"RSA 4096", 4096},
		{"EC P-256", 256},
		{"EC P-384", 384},
		{"Ed25519", 256},
		{"", 0},
		{"weird", 0},
	} {
		got := keyBitsFromAlgo(tc.in)
		if got != tc.want {
			t.Errorf("keyBitsFromAlgo(%q) = %d, want %d", tc.in, got, tc.want)
		}
	}
}

func TestIsWeakKey(t *testing.T) {
	for _, tc := range []struct {
		algo string
		bits int
		want bool
	}{
		{"RSA 2048", 2048, false},
		{"RSA 1024", 1024, true},
		{"EC P-256", 256, false},
		{"EC P-224", 224, true},
		{"Ed25519", 256, false},
	} {
		got := isWeakKey(tc.algo, tc.bits)
		if got != tc.want {
			t.Errorf("isWeakKey(%s, %d) = %v, want %v", tc.algo, tc.bits, got, tc.want)
		}
	}
}

func TestIsWeakSig(t *testing.T) {
	for _, tc := range []struct {
		sig  string
		want bool
	}{
		{"SHA256-RSA", false},
		{"SHA384-RSA", false},
		{"ECDSA-SHA256", false},
		{"SHA1-RSA", true},
		{"MD5-RSA", true},
	} {
		if got := isWeakSig(tc.sig); got != tc.want {
			t.Errorf("isWeakSig(%q) = %v, want %v", tc.sig, got, tc.want)
		}
	}
}

func TestFullScan_RealHTTPTestServer(t *testing.T) {
	ts, host, port := startTLSServer(t, nil, "max-age=31536000")
	defer ts.Close()

	r, err := Scan(context.Background(), Options{
		Host:    host,
		Port:    port,
		SNI:     host,
		Timeout: 10 * time.Second,
	})
	if err != nil {
		t.Fatal(err)
	}
	if r.Target != host {
		t.Errorf("Target = %s, want %s", r.Target, host)
	}
	if r.Cert == nil {
		t.Error("Cert section should be populated")
	}
	if len(r.Versions.Results) != 4 {
		t.Errorf("version results = %d, want 4", len(r.Versions.Results))
	}
	if r.HSTS == nil || !r.HSTS.Present {
		t.Error("HSTS should be present (we set the header)")
	}
	if r.Grade.Letter == "" {
		t.Error("Grade.Letter should be set")
	}
}

func TestScan_NoHost_Errors(t *testing.T) {
	_, err := Scan(context.Background(), Options{})
	if err == nil {
		t.Error("missing Host should error")
	}
}

func TestVersionsHighestSupported(t *testing.T) {
	v := VersionsSection{Results: []VersionResult{
		{Version: "TLS 1.0", Supported: true},
		{Version: "TLS 1.2", Supported: true},
		{Version: "TLS 1.3", Supported: true},
	}}
	if got := v.HighestSupported(); got != tls.VersionTLS13 {
		t.Errorf("HighestSupported = %x, want TLS 1.3", got)
	}
}

func TestCiphersSection_Counts(t *testing.T) {
	c := CiphersSection{TLS12: []CipherResult{
		{Supported: true, Strength: StrengthStrong, Name: "ECDHE-RSA-AES256-GCM-SHA384"},
		{Supported: true, Strength: StrengthAcceptable, Name: "RSA_WITH_AES_256_GCM_SHA384"},
		{Supported: true, Strength: StrengthWeak, Name: "TLS_RSA_WITH_3DES_EDE_CBC_SHA"},
		{Supported: false, Strength: StrengthStrong, Name: "X"},
	}}
	if got := c.SupportedStrong(); got != 1 {
		t.Errorf("SupportedStrong = %d, want 1", got)
	}
	if got := c.SupportedWeak(); got != 1 {
		t.Errorf("SupportedWeak = %d, want 1", got)
	}
	if got := c.SupportedNonFS(); got != 2 {
		// RSA_WITH and TLS_RSA_WITH 都没 ECDHE
		t.Errorf("SupportedNonFS = %d, want 2", got)
	}
}

func TestALPN_HasH2(t *testing.T) {
	if (ALPNSection{Supported: []string{"h2", "http/1.1"}}).HasH2() == false {
		t.Error("expected HasH2 true")
	}
	if (ALPNSection{Supported: []string{"http/1.1"}}).HasH2() == true {
		t.Error("expected HasH2 false")
	}
}

func TestContainsHelper(t *testing.T) {
	if !contains("ECDHE-RSA-AES256-GCM-SHA384", "GCM") {
		t.Error("GCM should match")
	}
	if contains("RSA_WITH_RC4", "ECDHE") {
		t.Error("ECDHE should not match in RSA_WITH_RC4")
	}
	if !strings.Contains("hello", "ell") {
		t.Error("sanity check") // ensure strings imported
	}
}
