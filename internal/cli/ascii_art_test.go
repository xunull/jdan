package cli

import (
	"bytes"
	"image"
	"image/color"
	"image/png"
	"io"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func pngBytes(t *testing.T, w, h int, c color.Color) []byte {
	t.Helper()
	m := image.NewRGBA(image.Rect(0, 0, w, h))
	for y := range h {
		for x := range w {
			m.Set(x, y, c)
		}
	}
	var buf bytes.Buffer
	if err := png.Encode(&buf, m); err != nil {
		t.Fatal(err)
	}
	return buf.Bytes()
}

func runAsciiArt(t *testing.T, stdin []byte, args ...string) (string, error) {
	t.Helper()
	var buf bytes.Buffer
	deps := asciiArtCmdDeps{out: &buf, errOut: io.Discard, in: bytes.NewReader(stdin)}
	cmd := newAsciiArtCommand(deps)
	cmd.SetArgs(args)
	err := cmd.Execute()
	return buf.String(), err
}

func TestAsciiArtCmd_Stdin(t *testing.T) {
	out, err := runAsciiArt(t, pngBytes(t, 40, 40, color.White), "-w", "20")
	if err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(out, "@") {
		t.Errorf("全白图应渲染出 '@':\n%s", out)
	}
}

func TestAsciiArtCmd_File(t *testing.T) {
	dir := t.TempDir()
	f := filepath.Join(dir, "img.png")
	if err := os.WriteFile(f, pngBytes(t, 30, 30, color.Black), 0o600); err != nil {
		t.Fatal(err)
	}
	out, err := runAsciiArt(t, nil, f, "-w", "10")
	if err != nil {
		t.Fatal(err)
	}
	if strings.TrimSpace(out) != "" {
		t.Errorf("全黑图应全空格:\n%q", out)
	}
}

func TestAsciiArtCmd_BadInput(t *testing.T) {
	if _, err := runAsciiArt(t, []byte("not an image"), "-w", "10"); err == nil {
		t.Error("非图片输入应报错")
	}
}

func TestAsciiArtCmd_FileNotFound(t *testing.T) {
	if _, err := runAsciiArt(t, nil, "/no/such/file.png"); err == nil {
		t.Error("文件不存在应报错")
	}
}

func TestAsciiArtCmd_NoANSIWhenPiped(t *testing.T) {
	// buffer 非 TTY，即便 --color 也不应插 ANSI
	out, err := runAsciiArt(t, pngBytes(t, 20, 20, color.RGBA{255, 0, 0, 255}), "-w", "10", "--color")
	if err != nil {
		t.Fatal(err)
	}
	if strings.Contains(out, "\x1b") {
		t.Error("管道（非 TTY）即便 --color 也不应有 ANSI")
	}
}
