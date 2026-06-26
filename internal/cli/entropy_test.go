package cli

import (
	"bytes"
	"encoding/json"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func runEntropy(t *testing.T, stdin string, args ...string) (string, error) {
	t.Helper()
	var buf bytes.Buffer
	deps := entropyCmdDeps{out: &buf}
	if stdin != "" {
		deps.in = strings.NewReader(stdin)
	}
	cmd := newEntropyCommand(deps)
	cmd.SetArgs(args)
	err := cmd.Execute()
	return buf.String(), err
}

func TestEntropyCmd_String(t *testing.T) {
	out, err := runEntropy(t, "", "hello world")
	if err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(out, "bits/byte") || !strings.Contains(out, "bytes:    11") {
		t.Errorf("output wrong:\n%s", out)
	}
}

func TestEntropyCmd_Stdin(t *testing.T) {
	out, err := runEntropy(t, "aaaaaaaa")
	if err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(out, "0.00 bits/byte") {
		t.Errorf("all-same bytes should be 0:\n%s", out)
	}
}

func TestEntropyCmd_File(t *testing.T) {
	dir := t.TempDir()
	p := filepath.Join(dir, "data.bin")
	if err := os.WriteFile(p, []byte("hello world"), 0o644); err != nil {
		t.Fatal(err)
	}
	out, err := runEntropy(t, "", "-f", p)
	if err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(out, "bytes:    11") {
		t.Errorf("file mode wrong:\n%s", out)
	}
}

func TestEntropyCmd_Window(t *testing.T) {
	out, err := runEntropy(t, strings.Repeat("a", 64), "--window", "16")
	if err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(out, "sparkline") {
		t.Errorf("--window should produce a sparkline:\n%s", out)
	}
}

func TestEntropyCmd_Charset(t *testing.T) {
	out, err := runEntropy(t, "", "Tr0ub4dour", "--charset")
	if err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(out, "charset") {
		t.Errorf("--charset should add a charset line:\n%s", out)
	}
}

func TestEntropyCmd_JSON(t *testing.T) {
	out, err := runEntropy(t, "", "hello", "--json")
	if err != nil {
		t.Fatal(err)
	}
	var v map[string]any
	if err := json.Unmarshal([]byte(out), &v); err != nil {
		t.Fatalf("--json should emit valid JSON: %v\n%s", err, out)
	}
}

func TestEntropyCmd_Empty(t *testing.T) {
	if _, err := runEntropy(t, ""); err == nil {
		t.Error("empty input should error")
	}
}
