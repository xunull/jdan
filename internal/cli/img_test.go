package cli

import (
	"bytes"
	"encoding/json"
	"image"
	"image/color"
	"image/gif"
	"image/jpeg"
	"image/png"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/xunull/jdan/internal/imageinfo"
)

func writePNG(t *testing.T, dir, name string, w, h int) string {
	t.Helper()
	img := image.NewNRGBA(image.Rect(0, 0, w, h))
	img.Set(0, 0, color.NRGBA{255, 0, 0, 255})
	path := filepath.Join(dir, name)
	f, err := os.Create(path)
	if err != nil {
		t.Fatal(err)
	}
	defer f.Close()
	if err := png.Encode(f, img); err != nil {
		t.Fatal(err)
	}
	return path
}

func writeJPEG(t *testing.T, dir, name string, w, h int) string {
	t.Helper()
	img := image.NewRGBA(image.Rect(0, 0, w, h))
	path := filepath.Join(dir, name)
	f, err := os.Create(path)
	if err != nil {
		t.Fatal(err)
	}
	defer f.Close()
	if err := jpeg.Encode(f, img, nil); err != nil {
		t.Fatal(err)
	}
	return path
}

func writeGIF(t *testing.T, dir, name string, w, h int) string {
	t.Helper()
	pal := image.NewPaletted(image.Rect(0, 0, w, h), []color.Color{color.Black, color.White})
	path := filepath.Join(dir, name)
	f, err := os.Create(path)
	if err != nil {
		t.Fatal(err)
	}
	defer f.Close()
	if err := gif.Encode(f, pal, nil); err != nil {
		t.Fatal(err)
	}
	return path
}

func runImgCmd(t *testing.T, in []byte, args ...string) (string, error) {
	t.Helper()
	var buf bytes.Buffer
	deps := imgCmdDeps{out: &buf}
	if in != nil {
		deps.in = bytes.NewReader(in)
	}
	cmd := newImgCommand(deps)
	cmd.SetArgs(args)
	err := cmd.Execute()
	return buf.String(), err
}

func TestImgCmd_SingleDetail(t *testing.T) {
	dir := t.TempDir()
	p := writePNG(t, dir, "logo.png", 120, 80)
	out, err := runImgCmd(t, nil, p)
	if err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(out, "120 x 80") {
		t.Errorf("detail should show dimensions:\n%s", out)
	}
	if !strings.Contains(out, "PNG") {
		t.Errorf("detail should show format:\n%s", out)
	}
}

func TestImgCmd_BatchTable(t *testing.T) {
	dir := t.TempDir()
	p1 := writePNG(t, dir, "a.png", 10, 10)
	p2 := writeJPEG(t, dir, "b.jpg", 20, 30)
	p3 := writeGIF(t, dir, "c.gif", 5, 5)
	out, err := runImgCmd(t, nil, p1, p2, p3)
	if err != nil {
		t.Fatal(err)
	}
	lines := strings.Split(strings.TrimRight(out, "\n"), "\n")
	if len(lines) != 3 {
		t.Fatalf("expected 3 table rows, got %d:\n%s", len(lines), out)
	}
	if !strings.Contains(out, "10x10") || !strings.Contains(out, "20x30") || !strings.Contains(out, "5x5") {
		t.Errorf("table missing dimensions:\n%s", out)
	}
}

func TestImgCmd_JSON(t *testing.T) {
	dir := t.TempDir()
	p := writePNG(t, dir, "x.png", 64, 48)
	out, err := runImgCmd(t, nil, p, "--json")
	if err != nil {
		t.Fatal(err)
	}
	var got []imageinfo.Info
	if err := json.Unmarshal([]byte(out), &got); err != nil {
		t.Fatalf("invalid JSON: %v\n%s", err, out)
	}
	if len(got) != 1 || got[0].Width != 64 || got[0].Height != 48 {
		t.Errorf("bad JSON content: %+v", got)
	}
}

func TestImgCmd_Stdin(t *testing.T) {
	var buf bytes.Buffer
	img := image.NewNRGBA(image.Rect(0, 0, 16, 16))
	if err := png.Encode(&buf, img); err != nil {
		t.Fatal(err)
	}
	out, err := runImgCmd(t, buf.Bytes())
	if err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(out, "<stdin>") || !strings.Contains(out, "16 x 16") {
		t.Errorf("stdin output wrong:\n%s", out)
	}
}

func TestImgCmd_FileNotFound(t *testing.T) {
	out, err := runImgCmd(t, nil, "/no/such/file.png")
	if err == nil {
		t.Error("missing file should error")
	}
	if !strings.Contains(out, "file.png") {
		t.Errorf("should report the bad path:\n%s", out)
	}
}

func TestImgCmd_BatchPartialFailure(t *testing.T) {
	dir := t.TempDir()
	good := writePNG(t, dir, "good.png", 10, 10)
	bad := filepath.Join(dir, "bad.txt")
	if err := os.WriteFile(bad, []byte("not an image"), 0o644); err != nil {
		t.Fatal(err)
	}
	out, err := runImgCmd(t, nil, good, bad)
	// 整体应当报错（有坏文件），但好文件仍被处理
	if err == nil {
		t.Error("batch with a bad file should error overall")
	}
	if !strings.Contains(out, "10 x 10") {
		t.Errorf("good file should still be processed:\n%s", out)
	}
	if !strings.Contains(out, "bad.txt") {
		t.Errorf("bad file should be reported:\n%s", out)
	}
}

func TestImgCmd_JSONEmptyOnAllFail(t *testing.T) {
	out, _ := runImgCmd(t, nil, "/no/such.png", "--json")
	// 全失败时 JSON 仍应是合法的空数组
	trimmed := strings.TrimSpace(out)
	idx := strings.Index(trimmed, "[")
	if idx < 0 {
		t.Fatalf("no JSON array in output:\n%s", out)
	}
	var got []imageinfo.Info
	if err := json.Unmarshal([]byte(trimmed[idx:]), &got); err != nil {
		t.Fatalf("invalid JSON: %v\n%s", err, out)
	}
	if len(got) != 0 {
		t.Errorf("expected empty array, got %+v", got)
	}
}
