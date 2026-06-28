package cli

import (
	"bytes"
	"encoding/json"
	"image/png"
	"io"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func runBarcode(t *testing.T, in io.Reader, args ...string) (string, error) {
	t.Helper()
	var out bytes.Buffer
	cmd := newBarcodeCommand(barcodeCmdDeps{out: &out, in: in})
	cmd.SetArgs(args)
	cmd.SetOut(io.Discard)
	cmd.SetErr(io.Discard)
	err := cmd.Execute()
	return out.String(), err
}

func TestBarcode_Terminal(t *testing.T) {
	out, err := runBarcode(t, nil, "ABC-123")
	if err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(out, "█") {
		t.Errorf("终端应渲染竖条:\n%s", out)
	}
	if !strings.Contains(out, "ABC-123") {
		t.Errorf("应在下方显示人眼可读文本:\n%s", out)
	}
}

func TestBarcode_NoText(t *testing.T) {
	out, _ := runBarcode(t, nil, "ABC-123", "--no-text")
	if strings.Contains(out, "ABC-123") {
		t.Errorf("--no-text 不应显示文本:\n%s", out)
	}
}

func TestBarcode_JSON(t *testing.T) {
	out, err := runBarcode(t, nil, "ABC-123", "--json")
	if err != nil {
		t.Fatal(err)
	}
	var got map[string]any
	if err := json.Unmarshal([]byte(out), &got); err != nil {
		t.Fatalf("非法 JSON: %v\n%s", err, out)
	}
	if got["code_set"] != "B" || got["data"] != "ABC-123" {
		t.Errorf("json=%v", got)
	}
}

func TestBarcode_PNG(t *testing.T) {
	p := filepath.Join(t.TempDir(), "bc.png")
	if _, err := runBarcode(t, nil, "SKU42", "-o", p); err != nil {
		t.Fatal(err)
	}
	f, err := os.Open(p)
	if err != nil {
		t.Fatal(err)
	}
	defer f.Close()
	img, err := png.Decode(f) // 必须是合法 PNG
	if err != nil {
		t.Fatalf("输出不是合法 PNG: %v", err)
	}
	if img.Bounds().Dx() <= 0 || img.Bounds().Dy() <= 0 {
		t.Error("PNG 尺寸非法")
	}
}

func TestBarcode_SVG(t *testing.T) {
	p := filepath.Join(t.TempDir(), "bc.svg")
	if _, err := runBarcode(t, nil, "SKU42", "-o", p); err != nil {
		t.Fatal(err)
	}
	b, _ := os.ReadFile(p)
	s := string(b)
	if !strings.HasPrefix(s, "<svg") || !strings.Contains(s, "<rect") {
		t.Errorf("SVG 结构不对:\n%s", s[:min(80, len(s))])
	}
	if !strings.Contains(s, ">SKU42</text>") {
		t.Error("SVG 应含人眼可读 <text>")
	}
}

func TestBarcode_Stdin(t *testing.T) {
	out, err := runBarcode(t, strings.NewReader("PIPED\n"))
	if err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(out, "PIPED") {
		t.Errorf("stdin 输入未渲染:\n%s", out)
	}
}

func TestBarcode_NonASCIIError(t *testing.T) {
	_, err := runBarcode(t, nil, "中文")
	if err == nil || !strings.Contains(err.Error(), "范围") {
		t.Errorf("非 ASCII 应报错，got %v", err)
	}
}
