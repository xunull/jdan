package cli

import (
	"bytes"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
)

func runHTTPGrade(t *testing.T, handler http.HandlerFunc, extraArgs ...string) (string, error) {
	t.Helper()
	srv := httptest.NewTLSServer(handler)
	defer srv.Close()

	var out bytes.Buffer
	deps := httpGradeDeps{out: &out, client: srv.Client()} // srv.Client() 信任测试证书
	cmd := newHTTPGradeCommand(deps)
	cmd.SetArgs(append([]string{srv.URL}, extraArgs...))
	err := cmd.Execute()
	return out.String(), err
}

func strongHeaders(w http.ResponseWriter, _ *http.Request) {
	h := w.Header()
	h.Set("Strict-Transport-Security", "max-age=31536000; includeSubDomains")
	h.Set("Content-Security-Policy", "default-src 'self'")
	h.Set("X-Content-Type-Options", "nosniff")
	h.Set("X-Frame-Options", "DENY")
	h.Set("Referrer-Policy", "strict-origin-when-cross-origin")
	h.Set("Permissions-Policy", "geolocation=()")
	w.WriteHeader(200)
}

func TestHTTPGrade_Strong(t *testing.T) {
	out, err := runHTTPGrade(t, strongHeaders)
	if err != nil {
		t.Fatalf("不该报错：%v", err)
	}
	if !strings.Contains(out, "A+") {
		t.Errorf("满配应 A+：\n%s", out)
	}
}

func TestHTTPGrade_Empty(t *testing.T) {
	out, err := runHTTPGrade(t, func(w http.ResponseWriter, _ *http.Request) { w.WriteHeader(200) })
	if err != nil {
		t.Fatalf("默认无 --fail-under 不该报错（评估报告恒 0 退出）：%v", err)
	}
	if !strings.Contains(out, "安全响应头评级：F") {
		t.Errorf("裸响应应 F：\n%s", out)
	}
}

func TestHTTPGrade_JSON(t *testing.T) {
	out, err := runHTTPGrade(t, strongHeaders, "--json")
	if err != nil {
		t.Fatal(err)
	}
	var v map[string]any
	if err := json.Unmarshal([]byte(out), &v); err != nil {
		t.Fatalf("--json 应合法：\n%s", out)
	}
	if v["grade"] == nil {
		t.Errorf("json 应含 grade：%v", v)
	}
}

func TestHTTPGrade_FailUnderTriggers(t *testing.T) {
	// 裸响应是 F，--fail-under B 应退出非 0（返回 error）。
	out, err := runHTTPGrade(t, func(w http.ResponseWriter, _ *http.Request) { w.WriteHeader(200) }, "--fail-under", "B")
	if err == nil {
		t.Error("F 低于阈值 B 应报错（非 0 退出）")
	}
	if !strings.Contains(out, "评级：F") {
		t.Errorf("报错前仍应打印报告：\n%s", out)
	}
}

func TestHTTPGrade_FailUnderPasses(t *testing.T) {
	_, err := runHTTPGrade(t, strongHeaders, "--fail-under", "B")
	if err != nil {
		t.Errorf("A+ 高于阈值 B 不该报错：%v", err)
	}
}

func TestHTTPGrade_BadFailUnder(t *testing.T) {
	_, err := runHTTPGrade(t, strongHeaders, "--fail-under", "Z")
	if err == nil || !strings.Contains(err.Error(), "非法等级") {
		t.Errorf("非法等级应报错：%v", err)
	}
}

func TestHTTPGrade_StrictAddsCrossOrigin(t *testing.T) {
	// strong headers 但没配 COOP/COEP/CORP：--strict 下评级应被拉低（不再 A+）。
	out, err := runHTTPGrade(t, strongHeaders, "--strict")
	if err != nil {
		t.Fatal(err)
	}
	if strings.Contains(out, "A+") {
		t.Errorf("--strict 下缺跨源隔离不应还是 A+：\n%s", out)
	}
}
