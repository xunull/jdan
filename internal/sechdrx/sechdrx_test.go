package sechdrx

import (
	"encoding/json"
	"net/http"
	"strings"
	"testing"
)

func hdr(kv map[string]string) http.Header {
	h := http.Header{}
	for k, v := range kv {
		h.Set(k, v)
	}
	return h
}

// check 找某个 header 的检查项。
func find(r Report, header string) (Check, bool) {
	for _, c := range r.Checks {
		if c.Header == header {
			return c, true
		}
	}
	return Check{}, false
}

func TestGrade_AllStrong(t *testing.T) {
	r := Grade(hdr(map[string]string{
		"Strict-Transport-Security": "max-age=31536000; includeSubDomains; preload",
		"Content-Security-Policy":   "default-src 'self'",
		"X-Content-Type-Options":    "nosniff",
		"X-Frame-Options":           "DENY",
		"Referrer-Policy":           "strict-origin-when-cross-origin",
		"Permissions-Policy":        "geolocation=()",
	}), true, Options{})
	if r.Score != 100 {
		t.Errorf("满配应满分 100，得 %d", r.Score)
	}
	if r.Grade != "A+" {
		t.Errorf("满配应 A+，得 %s", r.Grade)
	}
}

func TestGrade_Empty(t *testing.T) {
	r := Grade(http.Header{}, true, Options{})
	// 全缺核心项 → 0 分（Server 未暴露算 pass 但不加分）。
	if r.Score != 0 {
		t.Errorf("全缺失应 0 分，得 %d", r.Score)
	}
	if r.Grade != "F" {
		t.Errorf("全缺失应 F，得 %s", r.Grade)
	}
}

func TestGrade_HSTS_NotHTTPS(t *testing.T) {
	r := Grade(hdr(map[string]string{"Strict-Transport-Security": "max-age=31536000"}), false, Options{})
	c, _ := find(r, "Strict-Transport-Security")
	if c.Status != Fail {
		t.Errorf("非 HTTPS 时 HSTS 应 fail：%+v", c)
	}
}

func TestGrade_HSTS_ShortMaxAge(t *testing.T) {
	r := Grade(hdr(map[string]string{"Strict-Transport-Security": "max-age=100"}), true, Options{})
	c, _ := find(r, "Strict-Transport-Security")
	if c.Status != Warn {
		t.Errorf("max-age 太短应 warn：%+v", c)
	}
}

func TestGrade_CSP_UnsafeInlineDowngrades(t *testing.T) {
	strong := Grade(hdr(map[string]string{"Content-Security-Policy": "default-src 'self'"}), true, Options{})
	weak := Grade(hdr(map[string]string{"Content-Security-Policy": "default-src 'self' 'unsafe-inline'"}), true, Options{})
	cs, _ := find(strong, "Content-Security-Policy")
	cw, _ := find(weak, "Content-Security-Policy")
	if cs.Status != Pass {
		t.Errorf("干净 CSP 应 pass：%+v", cs)
	}
	if cw.Status != Warn {
		t.Errorf("含 unsafe-inline 应 warn：%+v", cw)
	}
	if weak.Score >= strong.Score {
		t.Errorf("unsafe-inline 应扣分：weak=%d strong=%d", weak.Score, strong.Score)
	}
}

func TestGrade_FrameCoveredByCSP(t *testing.T) {
	// 没有 X-Frame-Options，但 CSP 有 frame-ancestors → frame 检查应 pass。
	r := Grade(hdr(map[string]string{"Content-Security-Policy": "frame-ancestors 'self'"}), true, Options{})
	c, _ := find(r, "X-Frame-Options")
	if c.Status != Pass {
		t.Errorf("CSP frame-ancestors 应覆盖 X-Frame-Options：%+v", c)
	}
}

func TestGrade_ServerVersionLeakDeducts(t *testing.T) {
	// 带个核心头做基线（否则都是 0 分、扣分被 clamp 抹掉，看不出差异）。
	withVer := Grade(hdr(map[string]string{"X-Content-Type-Options": "nosniff", "Server": "nginx/1.18.0"}), true, Options{})
	noVer := Grade(hdr(map[string]string{"X-Content-Type-Options": "nosniff", "Server": "cloudflare"}), true, Options{})
	cv, _ := find(withVer, "Server")
	cn, _ := find(noVer, "Server")
	if cv.Status != Warn {
		t.Errorf("Server 带版本应 warn：%+v", cv)
	}
	if cn.Status != Info {
		t.Errorf("Server 无版本应 info（不扣分）：%+v", cn)
	}
	if withVer.Score >= noVer.Score {
		t.Errorf("带版本号应扣分：withVer=%d noVer=%d", withVer.Score, noVer.Score)
	}
}

func TestGrade_XPoweredByDeducts(t *testing.T) {
	r := Grade(hdr(map[string]string{"X-Powered-By": "PHP/8.1"}), true, Options{})
	c, ok := find(r, "X-Powered-By")
	if !ok || c.Status != Warn {
		t.Errorf("X-Powered-By 应报 warn：%+v", c)
	}
}

func TestGrade_StrictAddsCrossOrigin(t *testing.T) {
	base := hdr(map[string]string{"Content-Security-Policy": "default-src 'self'"})
	def := Grade(base, true, Options{Strict: false})
	strict := Grade(base, true, Options{Strict: true})
	// 默认：COOP 缺失是 Info；strict：是 Fail 且扣分
	cDef, _ := find(def, "Cross-Origin-Opener-Policy")
	cStrict, _ := find(strict, "Cross-Origin-Opener-Policy")
	if cDef.Status != Info {
		t.Errorf("默认 COOP 缺失应 info：%+v", cDef)
	}
	if cStrict.Status != Fail {
		t.Errorf("strict 下 COOP 缺失应 fail：%+v", cStrict)
	}
	if strict.Score >= def.Score {
		t.Errorf("strict 缺跨源隔离应扣分：strict=%d def=%d", strict.Score, def.Score)
	}
}

func TestGrade_ScoreClampedAtZero(t *testing.T) {
	// 全缺核心 + 一堆泄露头，分不能变负。
	r := Grade(hdr(map[string]string{
		"Server":              "Apache/2.4.1",
		"X-Powered-By":        "PHP/8",
		"X-AspNet-Version":    "4.0",
		"X-AspNetMvc-Version": "5.2",
	}), true, Options{})
	if r.Score < 0 {
		t.Errorf("分数不应为负：%d", r.Score)
	}
}

func TestRank_Ordering(t *testing.T) {
	if !(Rank("A+") > Rank("A") && Rank("A") > Rank("B") && Rank("B") > Rank("F")) {
		t.Error("等级序应 A+ > A > B > F")
	}
	if Rank("garbage") != 0 {
		t.Error("未知等级应为 0")
	}
}

func TestRender_ContainsGradeAndAdvice(t *testing.T) {
	r := Grade(http.Header{}, true, Options{})
	s := r.Render()
	if !strings.Contains(s, "安全响应头评级：F") {
		t.Errorf("应含等级行：\n%s", s)
	}
	if !strings.Contains(s, "建议：") {
		t.Errorf("fail 项应给建议：\n%s", s)
	}
}

func TestFormatJSON(t *testing.T) {
	r := Grade(hdr(map[string]string{"X-Content-Type-Options": "nosniff"}), true, Options{})
	r.URL = "https://x.test"
	s, err := r.FormatJSON()
	if err != nil {
		t.Fatal(err)
	}
	var v map[string]any
	if err := json.Unmarshal([]byte(s), &v); err != nil {
		t.Fatalf("非法 json：\n%s", s)
	}
	if v["grade"] == nil || v["url"] != "https://x.test" {
		t.Errorf("json 缺字段：%v", v)
	}
}
