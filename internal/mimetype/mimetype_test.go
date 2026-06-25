package mimetype

import (
	"strings"
	"testing"
)

func TestDetect_StdlibFormats(t *testing.T) {
	cases := []struct {
		name string
		data []byte
		want string // 主类型前缀
	}{
		{"png", []byte("\x89PNG\r\n\x1a\n"), "image/png"},
		{"pdf", []byte("%PDF-1.7\n"), "application/pdf"},
		{"gif", []byte("GIF89a"), "image/gif"},
		{"zip", []byte("PK\x03\x04"), "application/zip"},
		{"gzip", []byte("\x1f\x8b\x08"), "application/x-gzip"},
		{"text", []byte("hello, world\n"), "text/plain"},
	}
	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			got := Detect(c.data)
			if !strings.HasPrefix(got, c.want) {
				t.Errorf("Detect(%s) = %q, want prefix %q", c.name, got, c.want)
			}
		})
	}
}

func TestDetect_ExtraSignatures(t *testing.T) {
	cases := []struct {
		name string
		data []byte
		want string
	}{
		{"elf", []byte("\x7fELF\x02\x01\x01"), "application/x-elf"},
		{"7z", []byte("\x37\x7a\xbc\xaf\x27\x1c\x00\x04"), "application/x-7z-compressed"},
		{"xz", []byte("\xfd7zXZ\x00\x00"), "application/x-xz"},
		{"zstd", []byte("\x28\xb5\x2f\xfd\x00\x00"), "application/zstd"},
		{"bzip2", []byte("BZh91AY"), "application/x-bzip2"},
		{"sqlite", []byte("SQLite format 3\x00rest"), "application/vnd.sqlite3"},
	}
	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			if got := Detect(c.data); got != c.want {
				t.Errorf("Detect(%s) = %q, want %q", c.name, got, c.want)
			}
		})
	}
}

func TestDetect_TarAtOffset257(t *testing.T) {
	data := make([]byte, 300)
	copy(data[257:], []byte("ustar"))
	if got := Detect(data); got != "application/x-tar" {
		t.Errorf("tar magic at 257 = %q, want application/x-tar", got)
	}
}

func TestDetect_Empty(t *testing.T) {
	if got := Detect(nil); got != "inode/x-empty" {
		t.Errorf("empty = %q, want inode/x-empty", got)
	}
	if got := Detect([]byte{}); got != "inode/x-empty" {
		t.Errorf("empty slice = %q, want inode/x-empty", got)
	}
}

func TestDetect_ExtraTakesPrecedence(t *testing.T) {
	// ELF magic 不该被 stdlib 误判成别的
	if got := Detect([]byte("\x7fELF")); got != "application/x-elf" {
		t.Errorf("ELF should win, got %q", got)
	}
}

func TestMatchSig_OffsetBeyondData(t *testing.T) {
	// 数据比 tar 偏移短，不该 panic、不该误命中
	short := []byte("ustar")
	if got := Detect(short); got == "application/x-tar" {
		t.Error("short data should not match tar (offset 257)")
	}
}

func TestExtMismatch_Mismatch(t *testing.T) {
	// .txt 实为 PNG → 不符
	ext, mismatch := ExtMismatch("weird.txt", "image/png")
	if ext != ".txt" || !mismatch {
		t.Errorf("got ext=%q mismatch=%v, want .txt/true", ext, mismatch)
	}
}

func TestExtMismatch_Match(t *testing.T) {
	// .png 实为 PNG → 相符
	if _, mismatch := ExtMismatch("logo.png", "image/png"); mismatch {
		t.Error(".png with image/png should not mismatch")
	}
	// .txt 实为 text → 相符（带 charset 也要 base 比较）
	if _, mismatch := ExtMismatch("a.txt", "text/plain; charset=utf-8"); mismatch {
		t.Error(".txt with text/plain should not mismatch")
	}
}

func TestExtMismatch_NoExtension(t *testing.T) {
	if ext, mismatch := ExtMismatch("Makefile", "text/plain; charset=utf-8"); ext != "" || mismatch {
		t.Errorf("no-ext file should give empty/false, got %q/%v", ext, mismatch)
	}
}

func TestExtMismatch_UnknownExtension(t *testing.T) {
	// .xyz 未知扩展名 → 不报不符
	if _, mismatch := ExtMismatch("data.xyz", "image/png"); mismatch {
		t.Error("unknown extension should not flag mismatch")
	}
}

func TestExtMismatch_OctetStreamNotFlagged(t *testing.T) {
	// 实测纯未知（octet-stream）不该对任何扩展名报不符
	if _, mismatch := ExtMismatch("a.png", "application/octet-stream"); mismatch {
		t.Error("octet-stream should not flag mismatch")
	}
}

func TestExtMismatch_CaseInsensitive(t *testing.T) {
	// 大写扩展名也要识别
	if _, mismatch := ExtMismatch("PHOTO.PNG", "image/png"); mismatch {
		t.Error(".PNG should be recognized (case-insensitive)")
	}
}
