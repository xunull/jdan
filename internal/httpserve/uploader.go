package httpserve

import (
	"fmt"
	"html/template"
	"io"
	"net/http"
	"os"
	"path/filepath"
	"strings"
	"time"
)

// 上传 multipart form 解析的内存阈值，超过会落盘到 Go 的临时文件。
// 这不限制总上传大小，只控制 buffer 行为。
const maxMultipartMemory = 32 << 20 // 32 MB

// uploadHandler 处理 GET 和 POST 上传：
//   - GET /upload  → 简单 HTML 表单（手机浏览器友好）
//   - POST /upload → 接收 multipart/form-data，写入 uploadDir
//
// 防覆盖：目标已存在时，文件名加 -YYYYMMDD-HHMMSS 时间戳。
// 防 path traversal：只用 filepath.Base，client 不能影响目标目录。
func uploadHandler(uploadDir string) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		switch r.Method {
		case http.MethodGet:
			renderUploadForm(w, uploadDir)
		case http.MethodPost:
			handleUpload(w, r, uploadDir)
		default:
			http.Error(w, "method not allowed", http.StatusMethodNotAllowed)
		}
	}
}

const uploadFormHTML = `<!doctype html>
<html lang="en">
<head>
  <meta charset="utf-8">
  <meta name="viewport" content="width=device-width, initial-scale=1">
  <title>jdan upload</title>
  <style>
    body { font-family: -apple-system, BlinkMacSystemFont, sans-serif; max-width: 480px; margin: 2em auto; padding: 0 1em; color: #222; }
    h1 { font-size: 1.1em; font-weight: 600; }
    .field { margin: 1em 0; }
    input[type=file] { width: 100%; padding: .5em; border: 1px solid #ddd; border-radius: 4px; }
    button { width: 100%; padding: .8em; font-size: 1em; background: #111; color: #fff; border: none; border-radius: 4px; }
    .note { color: #888; font-size: .85em; margin-top: 2em; }
  </style>
</head>
<body>
  <h1>upload to {{.Dir}}</h1>
  <form method="POST" action="/upload" enctype="multipart/form-data">
    <div class="field">
      <input type="file" name="file" multiple required>
    </div>
    <button type="submit">upload</button>
  </form>
  <p class="note">files are saved on the host machine running jdan http serve.</p>
</body>
</html>
`

var uploadFormTmpl = template.Must(template.New("upload").Parse(uploadFormHTML))

func renderUploadForm(w http.ResponseWriter, uploadDir string) {
	w.Header().Set("Content-Type", "text/html; charset=utf-8")
	_ = uploadFormTmpl.Execute(w, map[string]string{"Dir": uploadDir})
}

func handleUpload(w http.ResponseWriter, r *http.Request, uploadDir string) {
	if err := r.ParseMultipartForm(maxMultipartMemory); err != nil {
		http.Error(w, "invalid multipart form: "+err.Error(), http.StatusBadRequest)
		return
	}
	if r.MultipartForm == nil || len(r.MultipartForm.File) == 0 {
		http.Error(w, "no file in form", http.StatusBadRequest)
		return
	}
	if err := os.MkdirAll(uploadDir, 0o755); err != nil {
		http.Error(w, "cannot create upload dir: "+err.Error(), http.StatusInternalServerError)
		return
	}

	var savedFiles []string
	for _, headers := range r.MultipartForm.File {
		for _, fh := range headers {
			src, err := fh.Open()
			if err != nil {
				http.Error(w, "open upload: "+err.Error(), http.StatusBadRequest)
				return
			}
			savedPath, err := writeUpload(uploadDir, fh.Filename, src)
			_ = src.Close()
			if err != nil {
				http.Error(w, "save upload: "+err.Error(), http.StatusInternalServerError)
				return
			}
			savedFiles = append(savedFiles, filepath.Base(savedPath))
		}
	}

	w.Header().Set("Content-Type", "text/plain; charset=utf-8")
	fmt.Fprintf(w, "uploaded %d file(s):\n", len(savedFiles))
	for _, f := range savedFiles {
		fmt.Fprintf(w, "  %s\n", f)
	}
}

// writeUpload 把 reader 内容存到 uploadDir/<sanitized basename>。
// 同名文件存在 → 加时间戳后缀。返回最终落盘的绝对路径。
// 单独导出（小写）便于单测直接调用，不必构造 multipart 请求。
func writeUpload(uploadDir, clientName string, src io.Reader) (string, error) {
	base := sanitizeFilename(clientName)
	if base == "" {
		return "", fmt.Errorf("invalid filename %q", clientName)
	}

	target := filepath.Join(uploadDir, base)
	if _, err := os.Stat(target); err == nil {
		ext := filepath.Ext(base)
		stem := strings.TrimSuffix(base, ext)
		ts := time.Now().Format("20060102-150405")
		target = filepath.Join(uploadDir, fmt.Sprintf("%s-%s%s", stem, ts, ext))
	}

	dst, err := os.OpenFile(target, os.O_WRONLY|os.O_CREATE|os.O_EXCL, 0o644)
	if err != nil {
		return "", err
	}
	defer dst.Close()
	if _, err := io.Copy(dst, src); err != nil {
		return "", err
	}
	return target, nil
}

// sanitizeFilename 把 client 提供的文件名收紧成只允许的 basename：
//   - 用 filepath.Base 去掉目录段（防 path traversal）
//   - trim 空格
//   - 拒绝 "" / "." / ".." / 任何带 / 或 \\ 的形式
func sanitizeFilename(name string) string {
	b := strings.TrimSpace(filepath.Base(name))
	if b == "" || b == "." || b == ".." {
		return ""
	}
	if strings.ContainsAny(b, `/\`) {
		return ""
	}
	return b
}
