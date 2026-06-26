package cli

import (
	"bytes"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func writeJSONFile(t *testing.T, dir, name, content string) string {
	t.Helper()
	p := filepath.Join(dir, name)
	if err := os.WriteFile(p, []byte(content), 0o644); err != nil {
		t.Fatal(err)
	}
	return p
}

// runJSONMerge 跑 json merge，支持注入 stdin。
func runJSONMerge(t *testing.T, stdin string, args ...string) (string, error) {
	t.Helper()
	var out bytes.Buffer
	cmd := newJSONCommand(jsonCmdDeps{out: &out, in: strings.NewReader(stdin)})
	cmd.SetArgs(append([]string{"merge"}, args...))
	err := cmd.Execute()
	return out.String(), err
}

func TestJSONMergeCmd_Basic(t *testing.T) {
	dir := t.TempDir()
	a := writeJSONFile(t, dir, "a.json", `{"a":1,"nest":{"x":1}}`)
	b := writeJSONFile(t, dir, "b.json", `{"b":2,"nest":{"y":2}}`)
	out, err := runJSONMerge(t, "", a, b)
	if err != nil {
		t.Fatal(err)
	}
	if canonJSON(t, out) != canonJSON(t, `{"a":1,"b":2,"nest":{"x":1,"y":2}}`) {
		t.Errorf("merge wrong: %s", strings.TrimSpace(out))
	}
}

func TestJSONMergeCmd_ArraysAppend(t *testing.T) {
	dir := t.TempDir()
	a := writeJSONFile(t, dir, "a.json", `{"l":[1,2]}`)
	b := writeJSONFile(t, dir, "b.json", `{"l":[3,4]}`)
	out, err := runJSONMerge(t, "", a, b, "--arrays", "append")
	if err != nil {
		t.Fatal(err)
	}
	if canonJSON(t, out) != canonJSON(t, `{"l":[1,2,3,4]}`) {
		t.Errorf("append wrong: %s", strings.TrimSpace(out))
	}
}

func TestJSONMergeCmd_Pretty(t *testing.T) {
	dir := t.TempDir()
	a := writeJSONFile(t, dir, "a.json", `{"a":1}`)
	b := writeJSONFile(t, dir, "b.json", `{"b":2}`)
	out, err := runJSONMerge(t, "", a, b, "-p")
	if err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(out, "\n") {
		t.Errorf("--pretty should indent:\n%s", out)
	}
}

func TestJSONMergeCmd_Stdin(t *testing.T) {
	dir := t.TempDir()
	b := writeJSONFile(t, dir, "b.json", `{"b":2}`)
	out, err := runJSONMerge(t, `{"a":1}`, "-", b)
	if err != nil {
		t.Fatal(err)
	}
	if canonJSON(t, out) != canonJSON(t, `{"a":1,"b":2}`) {
		t.Errorf("stdin merge wrong: %s", strings.TrimSpace(out))
	}
}

func TestJSONMergeCmd_DoubleStdin(t *testing.T) {
	if _, err := runJSONMerge(t, `{"a":1}`, "-", "-"); err == nil {
		t.Error("two - (stdin) should error")
	}
}

func TestJSONMergeCmd_TooFewArgs(t *testing.T) {
	dir := t.TempDir()
	a := writeJSONFile(t, dir, "a.json", `{"a":1}`)
	if _, err := runJSONMerge(t, "", a); err == nil {
		t.Error("merge of a single file should error")
	}
}

func TestJSONMergeCmd_InvalidJSON(t *testing.T) {
	dir := t.TempDir()
	a := writeJSONFile(t, dir, "a.json", `{"a":1}`)
	b := writeJSONFile(t, dir, "b.json", `{bad`)
	if _, err := runJSONMerge(t, "", a, b); err == nil {
		t.Error("invalid JSON should error")
	}
}

func TestJSONMergeCmd_MissingFile(t *testing.T) {
	dir := t.TempDir()
	a := writeJSONFile(t, dir, "a.json", `{"a":1}`)
	if _, err := runJSONMerge(t, "", a, filepath.Join(dir, "nope.json")); err == nil {
		t.Error("missing file should error")
	}
}

func TestJSONMergeCmd_BadArraysFlag(t *testing.T) {
	dir := t.TempDir()
	a := writeJSONFile(t, dir, "a.json", `{"a":1}`)
	b := writeJSONFile(t, dir, "b.json", `{"b":2}`)
	if _, err := runJSONMerge(t, "", a, b, "--arrays", "union"); err == nil {
		t.Error("invalid --arrays should error")
	}
}
