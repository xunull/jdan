package cli

import (
	"bytes"
	"encoding/json"
	"io"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func runMeta(t *testing.T, in io.Reader, args ...string) (string, error) {
	t.Helper()
	var out bytes.Buffer
	cmd := newMetaCommand(metaCmdDeps{out: &out, errOut: io.Discard, in: in})
	cmd.SetArgs(args)
	cmd.SetOut(io.Discard)
	cmd.SetErr(io.Discard)
	err := cmd.Execute()
	return out.String(), err
}

const sampleHTML = `<html lang="en"><head><title>Hello</title>` +
	`<meta name="description" content="desc">` +
	`<meta property="og:image" content="https://x/i.png"></head></html>`

func TestMeta_Stdin(t *testing.T) {
	out, err := runMeta(t, strings.NewReader(sampleHTML))
	if err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(out, "Hello") || !strings.Contains(out, "og:image") {
		t.Errorf("stdin 解析输出缺字段:\n%s", out)
	}
}

func TestMeta_File(t *testing.T) {
	p := filepath.Join(t.TempDir(), "page.html")
	if err := os.WriteFile(p, []byte(sampleHTML), 0o644); err != nil {
		t.Fatal(err)
	}
	out, err := runMeta(t, nil, p)
	if err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(out, "Hello") {
		t.Errorf("文件解析缺标题:\n%s", out)
	}
}

func TestMeta_URL(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "text/html; charset=utf-8")
		io.WriteString(w, sampleHTML)
	}))
	defer srv.Close()

	out, err := runMeta(t, nil, srv.URL)
	if err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(out, "Hello") || !strings.Contains(out, "og:image") {
		t.Errorf("URL 抓取输出缺字段:\n%s", out)
	}
}

func TestMeta_NonHTML(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "image/png")
		w.Write([]byte{0x89, 0x50, 0x4e, 0x47})
	}))
	defer srv.Close()

	_, err := runMeta(t, nil, srv.URL)
	if err == nil || !strings.Contains(err.Error(), "非 HTML") {
		t.Errorf("非 HTML 内容应报错，got %v", err)
	}
}

func TestMeta_JSON(t *testing.T) {
	out, err := runMeta(t, strings.NewReader(sampleHTML), "--json")
	if err != nil {
		t.Fatal(err)
	}
	var got map[string]any
	if err := json.Unmarshal([]byte(out), &got); err != nil {
		t.Fatalf("--json 不是合法 JSON: %v\n%s", err, out)
	}
	meta, ok := got["meta"].(map[string]any)
	if !ok || meta["title"] != "Hello" {
		t.Errorf("json meta.title 错: %v", got["meta"])
	}
}
