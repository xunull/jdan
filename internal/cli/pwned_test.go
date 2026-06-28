package cli

import (
	"bytes"
	"encoding/json"
	"io"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
)

// "password" 的 SHA1 = 5BAA61E4C9B93F3F0682250B6CF8331B7EE68FD8
// prefix=5BAA6 suffix=1E4C9B93F3F0682250B6CF8331B7EE68FD8
const pwBody = "0000000000000000000000000000000000A:0\r\n" +
	"1E4C9B93F3F0682250B6CF8331B7EE68FD8:99\r\n"

func runPwned(t *testing.T, in io.Reader, baseURL string, args ...string) (string, int) {
	t.Helper()
	var out bytes.Buffer
	code := -1 // -1 = 没调 exit（干净）
	deps := pwnedCmdDeps{
		out: &out, errOut: io.Discard, in: in,
		exit:    func(c int) { code = c },
		baseURL: baseURL + "/",
	}
	cmd := newPwnedCommand(deps)
	cmd.SetArgs(args)
	cmd.SetOut(io.Discard)
	cmd.SetErr(io.Discard)
	_ = cmd.Execute()
	return out.String(), code
}

func pwnedServer(body string, gotPath *string) *httptest.Server {
	return httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if gotPath != nil {
			*gotPath = r.URL.Path
		}
		io.WriteString(w, body)
	}))
}

func TestPwned_Found(t *testing.T) {
	srv := pwnedServer(pwBody, nil)
	defer srv.Close()
	out, code := runPwned(t, strings.NewReader("password"), srv.URL)
	if !strings.Contains(out, "泄露") || !strings.Contains(out, "99") {
		t.Errorf("应报泄露 99 次:\n%s", out)
	}
	if code != 1 {
		t.Errorf("泄露应 exit 1，got %d", code)
	}
}

func TestPwned_Clean(t *testing.T) {
	srv := pwnedServer(pwBody, nil) // body 里没有 "hello" 的 suffix
	defer srv.Close()
	out, code := runPwned(t, strings.NewReader("hello"), srv.URL)
	if !strings.Contains(out, "没在 HIBP") {
		t.Errorf("应报未泄露:\n%s", out)
	}
	if code != -1 {
		t.Errorf("干净不应调 exit，got %d", code)
	}
}

// 核心隐私保证：只有 5 位前缀出本机，完整哈希绝不出现在请求里。
func TestPwned_OnlyPrefixSent(t *testing.T) {
	var path string
	srv := pwnedServer(pwBody, &path)
	defer srv.Close()
	runPwned(t, strings.NewReader("password"), srv.URL)
	if path != "/5BAA6" {
		t.Errorf("应只发 5 位前缀，实际 path=%q", path)
	}
	if strings.Contains(path, "1E4C9B93") {
		t.Error("完整哈希/后缀绝不能出现在请求 URL 里")
	}
}

func TestPwned_AddPaddingDefault(t *testing.T) {
	var padding string
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		padding = r.Header.Get("Add-Padding")
		io.WriteString(w, pwBody)
	}))
	defer srv.Close()
	runPwned(t, strings.NewReader("password"), srv.URL)
	if padding != "true" {
		t.Errorf("默认应发 Add-Padding: true，got %q", padding)
	}
}

func TestPwned_JSON(t *testing.T) {
	srv := pwnedServer(pwBody, nil)
	defer srv.Close()
	out, _ := runPwned(t, strings.NewReader("password"), srv.URL, "--json")
	var got map[string]any
	if err := json.Unmarshal([]byte(out), &got); err != nil {
		t.Fatalf("非法 JSON: %v\n%s", err, out)
	}
	if got["pwned"] != true || got["count"].(float64) != 99 {
		t.Errorf("json=%v", got)
	}
}

func TestPwned_Batch(t *testing.T) {
	srv := pwnedServer(pwBody, nil)
	defer srv.Close()
	out, code := runPwned(t, strings.NewReader("password\nhello\n"), srv.URL, "--batch")
	if !strings.Contains(out, "2 个里有 1 个已泄露") {
		t.Errorf("批量汇总错:\n%s", out)
	}
	if code != 1 {
		t.Errorf("批量含泄露应 exit 1，got %d", code)
	}
}

func TestPwned_EmptyInput(t *testing.T) {
	srv := pwnedServer(pwBody, nil)
	defer srv.Close()
	_, code := runPwned(t, strings.NewReader(""), srv.URL)
	if code != 2 {
		t.Errorf("空密码应 exit 2，got %d", code)
	}
}

func TestCommafy(t *testing.T) {
	cases := map[int]string{0: "0", 999: "999", 1000: "1,000", 52372427: "52,372,427"}
	for in, want := range cases {
		if got := commafy(in); got != want {
			t.Errorf("commafy(%d)=%q want %q", in, got, want)
		}
	}
}
