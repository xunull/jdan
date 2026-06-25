package cli

import (
	"bytes"
	"encoding/json"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func writeFile(t *testing.T, dir, name string, content []byte) string {
	t.Helper()
	p := filepath.Join(dir, name)
	if err := os.WriteFile(p, content, 0o644); err != nil {
		t.Fatal(err)
	}
	return p
}

func runMimeCmd(t *testing.T, in []byte, args ...string) (string, error) {
	t.Helper()
	var buf bytes.Buffer
	deps := mimeCmdDeps{out: &buf}
	if in != nil {
		deps.in = bytes.NewReader(in)
	}
	cmd := newMimeCommand(deps)
	cmd.SetArgs(args)
	err := cmd.Execute()
	return buf.String(), err
}

var pngMagic = []byte("\x89PNG\r\n\x1a\n\x00\x00")

func TestMimeCmd_SingleClean(t *testing.T) {
	dir := t.TempDir()
	p := writeFile(t, dir, "logo.png", pngMagic)
	out, err := runMimeCmd(t, nil, p)
	if err != nil {
		t.Fatal(err)
	}
	if strings.TrimSpace(out) != "image/png" {
		t.Errorf("single-file output should be just the mime, got %q", out)
	}
}

func TestMimeCmd_BatchTable(t *testing.T) {
	dir := t.TempDir()
	p1 := writeFile(t, dir, "a.png", pngMagic)
	p2 := writeFile(t, dir, "b.pdf", []byte("%PDF-1.4\n"))
	out, err := runMimeCmd(t, nil, p1, p2)
	if err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(out, "image/png") || !strings.Contains(out, "application/pdf") {
		t.Errorf("batch missing types:\n%s", out)
	}
	lines := strings.Split(strings.TrimRight(out, "\n"), "\n")
	if len(lines) != 2 {
		t.Errorf("expected 2 rows, got %d", len(lines))
	}
}

func TestMimeCmd_ExtMismatchNote(t *testing.T) {
	dir := t.TempDir()
	p := writeFile(t, dir, "weird.txt", pngMagic) // PNG 内容、.txt 扩展名
	out, err := runMimeCmd(t, nil, p)
	if err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(out, "image/png") || !strings.Contains(out, "不符") {
		t.Errorf("should flag extension mismatch:\n%s", out)
	}
}

func TestMimeCmd_JSON(t *testing.T) {
	dir := t.TempDir()
	p := writeFile(t, dir, "weird.txt", pngMagic)
	out, err := runMimeCmd(t, nil, p, "--json")
	if err != nil {
		t.Fatal(err)
	}
	var got []mimeInfo
	if err := json.Unmarshal([]byte(out), &got); err != nil {
		t.Fatalf("invalid JSON: %v\n%s", err, out)
	}
	if len(got) != 1 || got[0].Mime != "image/png" || !got[0].ExtMismatch || got[0].Ext != ".txt" {
		t.Errorf("bad JSON content: %+v", got)
	}
}

func TestMimeCmd_Stdin(t *testing.T) {
	out, err := runMimeCmd(t, pngMagic)
	if err != nil {
		t.Fatal(err)
	}
	if strings.TrimSpace(out) != "image/png" {
		t.Errorf("stdin should detect png, got %q", out)
	}
}

func TestMimeCmd_EmptyFile(t *testing.T) {
	dir := t.TempDir()
	p := writeFile(t, dir, "empty.dat", []byte{})
	out, err := runMimeCmd(t, nil, p)
	if err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(out, "inode/x-empty") {
		t.Errorf("empty file should be inode/x-empty, got %q", out)
	}
}

func TestMimeCmd_FileNotFound(t *testing.T) {
	out, err := runMimeCmd(t, nil, "/no/such/file.bin")
	if err == nil {
		t.Error("missing file should error")
	}
	if !strings.Contains(out, "file.bin") {
		t.Errorf("should report the bad path:\n%s", out)
	}
}

func TestMimeCmd_BatchPartialFailure(t *testing.T) {
	dir := t.TempDir()
	good := writeFile(t, dir, "good.png", pngMagic)
	out, err := runMimeCmd(t, nil, good, "/no/such.bin")
	if err == nil {
		t.Error("batch with a bad file should error overall")
	}
	if !strings.Contains(out, "image/png") {
		t.Errorf("good file should still be processed:\n%s", out)
	}
	if !strings.Contains(out, "such.bin") {
		t.Errorf("bad file should be reported:\n%s", out)
	}
}

func TestMimeCmd_JSONEmptyOnAllFail(t *testing.T) {
	out, _ := runMimeCmd(t, nil, "/no/such.bin", "--json")
	trimmed := strings.TrimSpace(out)
	idx := strings.Index(trimmed, "[")
	if idx < 0 {
		t.Fatalf("no JSON array in output:\n%s", out)
	}
	var got []mimeInfo
	if err := json.Unmarshal([]byte(trimmed[idx:]), &got); err != nil {
		t.Fatalf("invalid JSON: %v\n%s", err, out)
	}
	if len(got) != 0 {
		t.Errorf("expected empty array, got %+v", got)
	}
}
