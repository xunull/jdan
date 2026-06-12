package cli

import (
	"bytes"
	"image/png"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func TestQR_DefaultRendersTerminal(t *testing.T) {
	var buf bytes.Buffer
	cmd := newQRCommand(qrCmdDeps{out: &buf})
	cmd.SetArgs([]string{"hello"})
	if err := cmd.Execute(); err != nil {
		t.Fatal(err)
	}
	if !strings.ContainsAny(buf.String(), "▀▄█") {
		t.Errorf("missing half-block chars in output")
	}
}

func TestQR_StdinInput(t *testing.T) {
	var buf bytes.Buffer
	cmd := newQRCommand(qrCmdDeps{
		out: &buf,
		in:  strings.NewReader("from-stdin\n"),
	})
	cmd.SetArgs(nil)
	if err := cmd.Execute(); err != nil {
		t.Fatal(err)
	}
	if buf.Len() == 0 {
		t.Error("expected output from stdin input")
	}
}

func TestQR_NoInput_Errors(t *testing.T) {
	cmd := newQRCommand(qrCmdDeps{
		out: &bytes.Buffer{},
		in:  strings.NewReader(""),
	})
	cmd.SilenceUsage = true
	cmd.SilenceErrors = true
	cmd.SetArgs(nil)
	if err := cmd.Execute(); err == nil {
		t.Error("empty input should error")
	}
}

func TestQR_JSONOutput(t *testing.T) {
	var buf bytes.Buffer
	cmd := newQRCommand(qrCmdDeps{out: &buf})
	cmd.SetArgs([]string{"hello", "--json"})
	if err := cmd.Execute(); err != nil {
		t.Fatal(err)
	}
	out := buf.String()
	if !strings.Contains(out, `"data": "hello"`) {
		t.Errorf("JSON missing data field, got: %s", out)
	}
	if !strings.Contains(out, `"ecc"`) {
		t.Errorf("JSON missing ecc field, got: %s", out)
	}
	if !strings.Contains(out, `"modules"`) {
		t.Errorf("JSON missing modules field, got: %s", out)
	}
}

func TestQR_WritePNGFile(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "test.png")
	cmd := newQRCommand(qrCmdDeps{out: &bytes.Buffer{}})
	cmd.SetArgs([]string{"hello", "--output", path})
	if err := cmd.Execute(); err != nil {
		t.Fatal(err)
	}
	b, err := os.ReadFile(path)
	if err != nil {
		t.Fatal(err)
	}
	img, err := png.Decode(bytes.NewReader(b))
	if err != nil {
		t.Errorf("output not valid PNG: %v", err)
	}
	if img.Bounds().Dx() != 256 {
		t.Errorf("default PNG size should be 256, got %d", img.Bounds().Dx())
	}
}

func TestQR_WritePNGCustomSize(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "test.png")
	cmd := newQRCommand(qrCmdDeps{out: &bytes.Buffer{}})
	cmd.SetArgs([]string{"hello", "--output", path, "--png-size", "100"})
	if err := cmd.Execute(); err != nil {
		t.Fatal(err)
	}
	b, _ := os.ReadFile(path)
	img, _ := png.Decode(bytes.NewReader(b))
	if img.Bounds().Dx() != 100 {
		t.Errorf("custom PNG size: got %d, want 100", img.Bounds().Dx())
	}
}

func TestQR_WriteSVGFile(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "test.svg")
	cmd := newQRCommand(qrCmdDeps{out: &bytes.Buffer{}})
	cmd.SetArgs([]string{"hello", "--output", path})
	if err := cmd.Execute(); err != nil {
		t.Fatal(err)
	}
	b, err := os.ReadFile(path)
	if err != nil {
		t.Fatal(err)
	}
	s := string(b)
	if !strings.HasPrefix(s, "<?xml") {
		t.Errorf("not valid SVG: %s", s[:50])
	}
	if !strings.HasSuffix(s, "</svg>") {
		t.Errorf("SVG not properly closed")
	}
}

func TestQR_UnsupportedOutput_Errors(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "test.jpg")
	cmd := newQRCommand(qrCmdDeps{out: &bytes.Buffer{}})
	cmd.SilenceUsage = true
	cmd.SilenceErrors = true
	cmd.SetArgs([]string{"hello", "--output", path})
	if err := cmd.Execute(); err == nil {
		t.Error("unsupported .jpg extension should error")
	}
}

func TestQR_InvalidECC_Errors(t *testing.T) {
	cmd := newQRCommand(qrCmdDeps{out: &bytes.Buffer{}})
	cmd.SilenceUsage = true
	cmd.SilenceErrors = true
	cmd.SetArgs([]string{"hello", "--ecc", "X"})
	if err := cmd.Execute(); err == nil {
		t.Error("invalid ECC level should error")
	}
}

func TestQR_FullBlock(t *testing.T) {
	var buf bytes.Buffer
	cmd := newQRCommand(qrCmdDeps{out: &buf})
	cmd.SetArgs([]string{"hello", "--full-block"})
	if err := cmd.Execute(); err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(buf.String(), "██") {
		t.Error("full-block should emit ██")
	}
}

func TestQR_Invert(t *testing.T) {
	var normal, inverted bytes.Buffer
	cmdA := newQRCommand(qrCmdDeps{out: &normal})
	cmdA.SetArgs([]string{"hello"})
	cmdA.Execute()
	cmdB := newQRCommand(qrCmdDeps{out: &inverted})
	cmdB.SetArgs([]string{"hello", "--invert"})
	cmdB.Execute()
	if normal.String() == inverted.String() {
		t.Error("--invert should produce different output")
	}
}

func TestQR_ECCLevels_AcceptedCases(t *testing.T) {
	for _, level := range []string{"L", "M", "Q", "H", "l", "m"} {
		var buf bytes.Buffer
		cmd := newQRCommand(qrCmdDeps{out: &buf})
		cmd.SetArgs([]string{"x", "--ecc", level})
		if err := cmd.Execute(); err != nil {
			t.Errorf("ECC %q rejected: %v", level, err)
		}
	}
}
