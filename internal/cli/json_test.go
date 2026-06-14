package cli

import (
	"bytes"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func TestJSONPretty_Stdin(t *testing.T) {
	var buf bytes.Buffer
	cmd := newJSONCommand(jsonCmdDeps{
		out: &buf,
		in:  strings.NewReader(`{"b":2,"a":1}`),
	})
	cmd.SetArgs([]string{"pretty"})
	if err := cmd.Execute(); err != nil {
		t.Fatal(err)
	}
	out := buf.String()
	if !strings.Contains(out, "  \"a\": 1") || !strings.Contains(out, "  \"b\": 2") {
		t.Errorf("pretty output:\n%s", out)
	}
}

func TestJSONPretty_FileArg(t *testing.T) {
	dir := t.TempDir()
	p := filepath.Join(dir, "x.json")
	_ = os.WriteFile(p, []byte(`{"a":1}`), 0o644)
	var buf bytes.Buffer
	cmd := newJSONCommand(jsonCmdDeps{out: &buf})
	cmd.SetArgs([]string{"pretty", p})
	if err := cmd.Execute(); err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(buf.String(), "\"a\": 1") {
		t.Errorf("file arg: %s", buf.String())
	}
}

func TestJSONPretty_InPlace(t *testing.T) {
	dir := t.TempDir()
	p := filepath.Join(dir, "x.json")
	_ = os.WriteFile(p, []byte(`{"b":2,"a":1}`), 0o644)
	cmd := newJSONCommand(jsonCmdDeps{out: &bytes.Buffer{}})
	cmd.SetArgs([]string{"pretty", p, "--in-place"})
	if err := cmd.Execute(); err != nil {
		t.Fatal(err)
	}
	body, _ := os.ReadFile(p)
	if !strings.Contains(string(body), "  \"a\": 1") {
		t.Errorf("in-place file:\n%s", body)
	}
}

func TestJSONPretty_InPlaceRequiresFile(t *testing.T) {
	cmd := newJSONCommand(jsonCmdDeps{
		out: &bytes.Buffer{},
		in:  strings.NewReader(`{"a":1}`),
	})
	cmd.SetArgs([]string{"pretty", "--in-place"})
	if err := cmd.Execute(); err == nil {
		t.Error("--in-place + stdin should error")
	}
}

func TestJSONMinify_Stdin(t *testing.T) {
	var buf bytes.Buffer
	cmd := newJSONCommand(jsonCmdDeps{
		out: &buf,
		in:  strings.NewReader("{\n  \"a\": 1,\n  \"b\": 2\n}"),
	})
	cmd.SetArgs([]string{"minify"})
	if err := cmd.Execute(); err != nil {
		t.Fatal(err)
	}
	got := strings.TrimSpace(buf.String())
	if got != `{"a":1,"b":2}` {
		t.Errorf("got %q", got)
	}
}

func TestJSONPath_StringResult(t *testing.T) {
	var buf bytes.Buffer
	cmd := newJSONCommand(jsonCmdDeps{
		out: &buf,
		in:  strings.NewReader(`{"name":"alice"}`),
	})
	cmd.SetArgs([]string{"path", "name"})
	if err := cmd.Execute(); err != nil {
		t.Fatal(err)
	}
	got := strings.TrimSpace(buf.String())
	if got != `"alice"` {
		t.Errorf("got %q, want \"alice\"", got)
	}
}

func TestJSONPath_RawString(t *testing.T) {
	var buf bytes.Buffer
	cmd := newJSONCommand(jsonCmdDeps{
		out: &buf,
		in:  strings.NewReader(`{"name":"alice"}`),
	})
	cmd.SetArgs([]string{"path", "name", "-r"})
	if err := cmd.Execute(); err != nil {
		t.Fatal(err)
	}
	got := strings.TrimSpace(buf.String())
	if got != "alice" {
		t.Errorf("got %q, want alice", got)
	}
}

func TestJSONPath_BracketIndex(t *testing.T) {
	var buf bytes.Buffer
	cmd := newJSONCommand(jsonCmdDeps{
		out: &buf,
		in:  strings.NewReader(`{"xs":[10,20,30]}`),
	})
	cmd.SetArgs([]string{"path", "xs[1]"})
	if err := cmd.Execute(); err != nil {
		t.Fatal(err)
	}
	got := strings.TrimSpace(buf.String())
	if got != "20" {
		t.Errorf("got %q, want 20", got)
	}
}

func TestJSONPath_DotPathOnArray(t *testing.T) {
	var buf bytes.Buffer
	cmd := newJSONCommand(jsonCmdDeps{
		out: &buf,
		in:  strings.NewReader(`{"xs":[10,20,30]}`),
	})
	cmd.SetArgs([]string{"path", "xs.2"})
	if err := cmd.Execute(); err != nil {
		t.Fatal(err)
	}
	got := strings.TrimSpace(buf.String())
	if got != "30" {
		t.Errorf("got %q, want 30", got)
	}
}

func TestJSONPath_Pointer(t *testing.T) {
	var buf bytes.Buffer
	cmd := newJSONCommand(jsonCmdDeps{
		out: &buf,
		in:  strings.NewReader(`{"a":{"b":42}}`),
	})
	cmd.SetArgs([]string{"path", "/a/b", "--pointer"})
	if err := cmd.Execute(); err != nil {
		t.Fatal(err)
	}
	got := strings.TrimSpace(buf.String())
	if got != "42" {
		t.Errorf("got %q, want 42", got)
	}
}

func TestJSONKeys_TopLevel(t *testing.T) {
	var buf bytes.Buffer
	cmd := newJSONCommand(jsonCmdDeps{
		out: &buf,
		in:  strings.NewReader(`{"c":1,"a":2,"b":3}`),
	})
	cmd.SetArgs([]string{"keys"})
	if err := cmd.Execute(); err != nil {
		t.Fatal(err)
	}
	got := strings.Split(strings.TrimSpace(buf.String()), "\n")
	if len(got) != 3 || got[0] != "a" || got[1] != "b" || got[2] != "c" {
		t.Errorf("got %v", got)
	}
}

func TestJSONKeys_AllPaths(t *testing.T) {
	var buf bytes.Buffer
	cmd := newJSONCommand(jsonCmdDeps{
		out: &buf,
		in:  strings.NewReader(`{"a":1,"b":{"c":[10,20]}}`),
	})
	cmd.SetArgs([]string{"keys", "--all"})
	if err := cmd.Execute(); err != nil {
		t.Fatal(err)
	}
	out := buf.String()
	for _, want := range []string{"a", "b.c[0]", "b.c[1]"} {
		if !strings.Contains(out, want+"\n") {
			t.Errorf("missing %q in:\n%s", want, out)
		}
	}
}

func TestJSONDiff_Text(t *testing.T) {
	dir := t.TempDir()
	a := filepath.Join(dir, "a.json")
	b := filepath.Join(dir, "b.json")
	_ = os.WriteFile(a, []byte(`{"x":1}`), 0o644)
	_ = os.WriteFile(b, []byte(`{"x":2}`), 0o644)

	var buf bytes.Buffer
	cmd := newJSONCommand(jsonCmdDeps{out: &buf})
	cmd.SetArgs([]string{"diff", a, b})
	if err := cmd.Execute(); err != nil {
		t.Fatal(err)
	}
	got := strings.TrimSpace(buf.String())
	if !strings.Contains(got, "~ /x: 1 -> 2") {
		t.Errorf("got %q", got)
	}
}

func TestJSONDiff_Identical(t *testing.T) {
	dir := t.TempDir()
	a := filepath.Join(dir, "a.json")
	b := filepath.Join(dir, "b.json")
	_ = os.WriteFile(a, []byte(`{"x":1}`), 0o644)
	_ = os.WriteFile(b, []byte(`{"x":1}`), 0o644)

	var buf bytes.Buffer
	cmd := newJSONCommand(jsonCmdDeps{out: &buf})
	cmd.SetArgs([]string{"diff", a, b})
	if err := cmd.Execute(); err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(buf.String(), "(identical)") {
		t.Errorf("got %q", buf.String())
	}
}

func TestJSONDiff_JSONOutputAndExitCode(t *testing.T) {
	dir := t.TempDir()
	a := filepath.Join(dir, "a.json")
	b := filepath.Join(dir, "b.json")
	_ = os.WriteFile(a, []byte(`{"x":1}`), 0o644)
	_ = os.WriteFile(b, []byte(`{"x":2}`), 0o644)

	var buf bytes.Buffer
	cmd := newJSONCommand(jsonCmdDeps{out: &buf})
	cmd.SetArgs([]string{"diff", a, b, "--json", "--exit-code"})
	err := cmd.Execute()
	if err == nil {
		t.Error("diff with --exit-code should return non-nil error when differing")
	}
	if _, ok := err.(*jsonCmdExitErr); !ok {
		t.Errorf("expected *jsonCmdExitErr, got %T", err)
	}
	if !strings.Contains(buf.String(), `"op": "replace"`) {
		t.Errorf("expected JSON patch in output:\n%s", buf.String())
	}
}

func TestJSONLines_Count(t *testing.T) {
	var buf bytes.Buffer
	in := `{"a":1}
{"b":2}

{"c":3}
`
	cmd := newJSONCommand(jsonCmdDeps{
		out: &buf,
		in:  strings.NewReader(in),
	})
	cmd.SetArgs([]string{"lines", "--count"})
	if err := cmd.Execute(); err != nil {
		t.Fatal(err)
	}
	if strings.TrimSpace(buf.String()) != "3" {
		t.Errorf("got %q, want 3", buf.String())
	}
}

func TestJSONLines_Get(t *testing.T) {
	var buf bytes.Buffer
	cmd := newJSONCommand(jsonCmdDeps{
		out: &buf,
		in:  strings.NewReader("{\"a\":1}\n{\"b\":2}\n"),
	})
	cmd.SetArgs([]string{"lines", "--get", "1"})
	if err := cmd.Execute(); err != nil {
		t.Fatal(err)
	}
	if strings.TrimSpace(buf.String()) != `{"b":2}` {
		t.Errorf("got %q", buf.String())
	}
}

func TestJSONLines_Head(t *testing.T) {
	var buf bytes.Buffer
	cmd := newJSONCommand(jsonCmdDeps{
		out: &buf,
		in:  strings.NewReader("{\"a\":1}\n{\"b\":2}\n{\"c\":3}\n"),
	})
	cmd.SetArgs([]string{"lines", "--head", "2"})
	if err := cmd.Execute(); err != nil {
		t.Fatal(err)
	}
	lines := strings.Split(strings.TrimSpace(buf.String()), "\n")
	if len(lines) != 2 || lines[0] != `{"a":1}` || lines[1] != `{"b":2}` {
		t.Errorf("got %v", lines)
	}
}

func TestJSONLines_ModesAreMutuallyExclusive(t *testing.T) {
	cmd := newJSONCommand(jsonCmdDeps{
		out: &bytes.Buffer{},
		in:  strings.NewReader("{}\n"),
	})
	cmd.SetArgs([]string{"lines", "--count", "--head", "2"})
	if err := cmd.Execute(); err == nil {
		t.Error("expected mutual-exclusivity error")
	}
}
