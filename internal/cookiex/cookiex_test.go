package cookiex

import (
	"net/http"
	"strings"
	"testing"
)

func auditLine(t *testing.T, setCookie string) string {
	t.Helper()
	c, err := ParseSetCookie(setCookie)
	if err != nil {
		t.Fatalf("ParseSetCookie(%q): %v", setCookie, err)
	}
	var b strings.Builder
	for _, is := range Audit(c) {
		b.WriteString(is.Msg + "\n")
	}
	return b.String()
}

func TestAudit_Weak(t *testing.T) {
	msgs := auditLine(t, "sid=abc; Path=/")
	for _, want := range []string{"Secure", "HttpOnly", "SameSite"} {
		if !strings.Contains(msgs, want) {
			t.Errorf("应提到 %q，实际:\n%s", want, msgs)
		}
	}
}

func TestAudit_SameSiteNoneNoSecure(t *testing.T) {
	msgs := auditLine(t, "sid=x; SameSite=None; HttpOnly")
	if !strings.Contains(msgs, "SameSite=None 必须配 Secure") {
		t.Errorf("SameSite=None 无 Secure 应告警:\n%s", msgs)
	}
}

func TestAudit_HostPrefix(t *testing.T) {
	msgs := auditLine(t, "__Host-sid=x; Path=/admin; Secure")
	if !strings.Contains(msgs, "__Host-") {
		t.Errorf("__Host- 前缀违规（Path 非 /）应告警:\n%s", msgs)
	}
}

func TestAudit_Strong(t *testing.T) {
	// __Host- 合规 + Secure + HttpOnly + SameSite=Lax → 无问题
	msgs := auditLine(t, "__Host-sid=x; Path=/; Secure; HttpOnly; SameSite=Lax")
	if msgs != "" {
		t.Errorf("合规 cookie 不应有体检项:\n%s", msgs)
	}
}

func TestSameSiteName(t *testing.T) {
	cases := map[http.SameSite]string{
		http.SameSiteLaxMode:    "Lax",
		http.SameSiteStrictMode: "Strict",
		http.SameSiteNoneMode:   "None",
		0:                       "(未设置)",
	}
	for in, want := range cases {
		if got := SameSiteName(in); got != want {
			t.Errorf("SameSiteName(%d)=%q want %q", in, got, want)
		}
	}
}
