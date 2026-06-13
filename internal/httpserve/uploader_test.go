package httpserve

import (
	"bytes"
	"mime/multipart"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func TestSanitizeFilename(t *testing.T) {
	// 关键安全语义：filepath.Base 把任何带目录段的输入收敛成 basename。
	// 文件落地永远在 uploadDir 下，client 不能通过文件名跳出。
	for _, tc := range []struct {
		in, want string
	}{
		{"report.pdf", "report.pdf"},
		{"  trim.txt  ", "trim.txt"},
		{"sub/dir/file.png", "file.png"},  // filepath.Base 截到 file.png
		{"../etc/passwd", "passwd"},       // 同上：basename 是 passwd
		{"/abs/path/x.bin", "x.bin"},      // 绝对路径也只取 basename
		{"", ""},
		{".", ""},
		{"..", ""},
	} {
		got := sanitizeFilename(tc.in)
		if got != tc.want {
			t.Errorf("sanitizeFilename(%q) = %q, want %q", tc.in, got, tc.want)
		}
	}
}

func TestWriteUpload_BasicWrite(t *testing.T) {
	dir := t.TempDir()
	path, err := writeUpload(dir, "hello.txt", strings.NewReader("world"))
	if err != nil {
		t.Fatal(err)
	}
	if filepath.Base(path) != "hello.txt" {
		t.Errorf("basename %q, want hello.txt", filepath.Base(path))
	}
	b, _ := os.ReadFile(path)
	if string(b) != "world" {
		t.Errorf("content %q, want 'world'", string(b))
	}
}

func TestWriteUpload_CollisionAddsTimestamp(t *testing.T) {
	dir := t.TempDir()
	// 第一次写入
	first, err := writeUpload(dir, "data.bin", strings.NewReader("v1"))
	if err != nil {
		t.Fatal(err)
	}
	// 同名第二次 → 应当加时间戳后缀，不应覆盖
	second, err := writeUpload(dir, "data.bin", strings.NewReader("v2"))
	if err != nil {
		t.Fatal(err)
	}
	if first == second {
		t.Errorf("collision should rename, got identical paths %q", first)
	}
	if !strings.Contains(filepath.Base(second), "data-") {
		t.Errorf("expected timestamped name like data-YYYYMMDD-HHMMSS.bin, got %q", filepath.Base(second))
	}
	// 第一个文件原内容必须没被改
	b, _ := os.ReadFile(first)
	if string(b) != "v1" {
		t.Errorf("first file was overwritten, content = %q", string(b))
	}
}

func TestWriteUpload_RejectsInvalidName(t *testing.T) {
	dir := t.TempDir()
	// 只有空 / "." / ".." 才被 reject；"a/b" 这种会被 filepath.Base 截成 "b" 并写入
	for _, bad := range []string{"", "..", "."} {
		if _, err := writeUpload(dir, bad, strings.NewReader("x")); err == nil {
			t.Errorf("writeUpload(%q) should error", bad)
		}
	}
}

func TestWriteUpload_TraversalAttemptStaysInDir(t *testing.T) {
	dir := t.TempDir()
	// 试图通过 "../escape.txt" 写到 dir 之外，应当被截到 dir/escape.txt
	path, err := writeUpload(dir, "../escape.txt", strings.NewReader("x"))
	if err != nil {
		t.Fatal(err)
	}
	abs, _ := filepath.Abs(path)
	dirAbs, _ := filepath.Abs(dir)
	if !strings.HasPrefix(abs, dirAbs) {
		t.Errorf("traversal escaped: file at %s, expected under %s", abs, dirAbs)
	}
}

func TestUploadHandler_GET_ServesForm(t *testing.T) {
	rec := httptest.NewRecorder()
	req := httptest.NewRequest("GET", "/upload", nil)
	uploadHandler("/tmp/uploads").ServeHTTP(rec, req)
	if rec.Code != http.StatusOK {
		t.Errorf("GET /upload: status %d", rec.Code)
	}
	body := rec.Body.String()
	for _, want := range []string{"<form", "multipart/form-data", "/tmp/uploads"} {
		if !strings.Contains(body, want) {
			t.Errorf("form missing %q", want)
		}
	}
}

func TestUploadHandler_POST_SavesFile(t *testing.T) {
	dir := t.TempDir()

	// 构造 multipart 请求
	body := &bytes.Buffer{}
	mw := multipart.NewWriter(body)
	part, _ := mw.CreateFormFile("file", "test.txt")
	_, _ = part.Write([]byte("hello upload"))
	_ = mw.Close()

	req := httptest.NewRequest("POST", "/upload", body)
	req.Header.Set("Content-Type", mw.FormDataContentType())
	rec := httptest.NewRecorder()

	uploadHandler(dir).ServeHTTP(rec, req)

	if rec.Code != http.StatusOK {
		t.Fatalf("POST /upload: status %d, body %s", rec.Code, rec.Body.String())
	}

	saved := filepath.Join(dir, "test.txt")
	b, err := os.ReadFile(saved)
	if err != nil {
		t.Fatalf("file not saved: %v", err)
	}
	if string(b) != "hello upload" {
		t.Errorf("content %q, want 'hello upload'", string(b))
	}

	if !strings.Contains(rec.Body.String(), "test.txt") {
		t.Error("response should mention saved filename")
	}
}

func TestUploadHandler_POST_TwoFiles(t *testing.T) {
	dir := t.TempDir()

	body := &bytes.Buffer{}
	mw := multipart.NewWriter(body)
	for _, name := range []string{"a.txt", "b.txt"} {
		part, _ := mw.CreateFormFile("file", name)
		_, _ = part.Write([]byte("data-" + name))
	}
	_ = mw.Close()

	req := httptest.NewRequest("POST", "/upload", body)
	req.Header.Set("Content-Type", mw.FormDataContentType())
	rec := httptest.NewRecorder()

	uploadHandler(dir).ServeHTTP(rec, req)

	if rec.Code != http.StatusOK {
		t.Fatalf("status %d", rec.Code)
	}
	for _, name := range []string{"a.txt", "b.txt"} {
		if _, err := os.Stat(filepath.Join(dir, name)); err != nil {
			t.Errorf("file %s not saved: %v", name, err)
		}
	}
}

func TestUploadHandler_POST_NoFile_400(t *testing.T) {
	dir := t.TempDir()
	body := &bytes.Buffer{}
	mw := multipart.NewWriter(body)
	_ = mw.WriteField("text", "no file") // 只有文本字段，没有文件
	_ = mw.Close()

	req := httptest.NewRequest("POST", "/upload", body)
	req.Header.Set("Content-Type", mw.FormDataContentType())
	rec := httptest.NewRecorder()

	uploadHandler(dir).ServeHTTP(rec, req)

	if rec.Code != http.StatusBadRequest {
		t.Errorf("no-file POST should be 400, got %d", rec.Code)
	}
}

func TestUploadHandler_OtherMethods_405(t *testing.T) {
	for _, method := range []string{"PUT", "DELETE", "PATCH"} {
		rec := httptest.NewRecorder()
		req := httptest.NewRequest(method, "/upload", nil)
		uploadHandler("/tmp").ServeHTTP(rec, req)
		if rec.Code != http.StatusMethodNotAllowed {
			t.Errorf("%s should be 405, got %d", method, rec.Code)
		}
	}
}
