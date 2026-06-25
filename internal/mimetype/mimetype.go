// Package mimetype 实现 jdan mime 命令的核心：读文件 magic bytes 报真实
// content-type（不看扩展名）。0 新依赖。
//
// 以 stdlib net/http 的 DetectContentType（WHATWG MIME Sniffing，~60 种）作底座，
// 在它之上加一层精选 magic 表覆盖 stdlib 漏掉的常见开发格式（ELF/7z/xz/zstd/
// bzip2/tar/SQLite）。
package mimetype

import (
	"net/http"
	"path/filepath"
	"strings"
)

// sniffLen 是 DetectContentType 需要的最大字节数；也足够覆盖最大偏移的 magic
// （tar 在 257）。调用方读前 512 字节即可。
const sniffLen = 512

type signature struct {
	offset int
	magic  []byte
	mime   string
}

// extraSignatures 是 stdlib 漏掉的格式（先于 DetectContentType 检查）。
var extraSignatures = []signature{
	{0, []byte{0x7f, 'E', 'L', 'F'}, "application/x-elf"},
	{0, []byte{0x37, 0x7a, 0xbc, 0xaf, 0x27, 0x1c}, "application/x-7z-compressed"},
	{0, []byte{0xfd, '7', 'z', 'X', 'Z', 0x00}, "application/x-xz"},
	{0, []byte{0x28, 0xb5, 0x2f, 0xfd}, "application/zstd"},
	{0, []byte{'B', 'Z', 'h'}, "application/x-bzip2"},
	{0, []byte("SQLite format 3\x00"), "application/vnd.sqlite3"},
	{257, []byte("ustar"), "application/x-tar"},
}

// Detect 按内容判断 content-type。空数据返回 inode/x-empty（对齐 file 语义）。
func Detect(data []byte) string {
	if len(data) == 0 {
		return "inode/x-empty"
	}
	for _, s := range extraSignatures {
		if matchSig(data, s) {
			return s.mime
		}
	}
	return http.DetectContentType(data)
}

func matchSig(data []byte, s signature) bool {
	if s.offset+len(s.magic) > len(data) {
		return false
	}
	for i, b := range s.magic {
		if data[s.offset+i] != b {
			return false
		}
	}
	return true
}

// baseType 去掉 "; charset=..." 等参数，返回主类型。
func baseType(m string) string {
	return strings.TrimSpace(strings.SplitN(m, ";", 2)[0])
}

// extExpected 把常见扩展名映射到「应有」主类型。仅收录有 magic 的二进制格式
// （加 .txt），避免对纯文本结构格式（json/csv 等会被探成 text/plain）误报。
// 故意不回退到 stdlib mime.TypeByExtension——那依赖 OS 的 mime.types，
// 会引入非确定性；本表 OS 无关、跨平台 + 测试可复现。
var extExpected = map[string]string{
	".txt":  "text/plain",
	".png":  "image/png",
	".jpg":  "image/jpeg",
	".jpeg": "image/jpeg",
	".gif":  "image/gif",
	".webp": "image/webp",
	".bmp":  "image/bmp",
	".pdf":  "application/pdf",
	".zip":  "application/zip",
	".gz":   "application/x-gzip",
	".tar":  "application/x-tar",
	".7z":   "application/x-7z-compressed",
	".xz":   "application/x-xz",
	".zst":  "application/zstd",
	".bz2":  "application/x-bzip2",
	".mp3":  "audio/mpeg",
	".mp4":  "video/mp4",
	".wav":  "audio/wave",
}

// ExtMismatch 报告文件扩展名「应有」的类型是否与实测 mime 不一致。
// 返回扩展名（小写，含点）和是否不符。无扩展名 / 扩展名未知 / 实测为
// application/octet-stream（纯未知）时不报不符。
func ExtMismatch(path, detected string) (ext string, mismatch bool) {
	ext = strings.ToLower(filepath.Ext(path))
	if ext == "" {
		return "", false
	}
	expected := extExpected[ext]
	if expected == "" {
		return ext, false
	}
	got := baseType(detected)
	if got == "application/octet-stream" {
		return ext, false
	}
	return ext, got != expected
}

// MaxSniff 暴露需要读取的最大字节数。
func MaxSniff() int { return sniffLen }
