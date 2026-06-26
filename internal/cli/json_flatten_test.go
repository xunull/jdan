package cli

import (
	"bytes"
	"encoding/json"
	"strings"
	"testing"
)

// canonJSON 把 JSON 文本规范化（解码再编码，key 排序），便于忽略顺序比较。
func canonJSON(t *testing.T, s string) string {
	t.Helper()
	var v any
	if err := json.Unmarshal([]byte(s), &v); err != nil {
		t.Fatalf("bad JSON %q: %v", s, err)
	}
	b, _ := json.Marshal(v)
	return string(b)
}

func runJSONSub(t *testing.T, in string, args ...string) (string, error) {
	t.Helper()
	var out bytes.Buffer
	cmd := newJSONCommand(jsonCmdDeps{out: &out, in: strings.NewReader(in)})
	cmd.SetArgs(args)
	err := cmd.Execute()
	return out.String(), err
}

func TestJSONFlattenCmd_Stdin(t *testing.T) {
	out, err := runJSONSub(t, `{"a":{"b":1,"c":[10,20]}}`, "flatten")
	if err != nil {
		t.Fatal(err)
	}
	got := strings.TrimSpace(out)
	if got != `{"a.b":1,"a.c[0]":10,"a.c[1]":20}` {
		t.Errorf("got %q", got)
	}
}

func TestJSONFlattenCmd_Pretty(t *testing.T) {
	out, err := runJSONSub(t, `{"a":{"b":1}}`, "flatten", "-p")
	if err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(out, "\n") {
		t.Errorf("--pretty should indent:\n%s", out)
	}
}

func TestJSONFlattenCmd_Sep(t *testing.T) {
	out, _ := runJSONSub(t, `{"a":{"b":1}}`, "flatten", "--sep", "/")
	if !strings.Contains(out, `"a/b"`) {
		t.Errorf("custom sep wrong: %s", out)
	}
}

func TestJSONUnflattenCmd_Stdin(t *testing.T) {
	out, err := runJSONSub(t, `{"a.b":1,"a.c":2}`, "unflatten")
	if err != nil {
		t.Fatal(err)
	}
	if strings.TrimSpace(out) != `{"a":{"b":1,"c":2}}` {
		t.Errorf("got %q", strings.TrimSpace(out))
	}
}

func TestJSONUnflattenCmd_NotObject(t *testing.T) {
	if _, err := runJSONSub(t, `[1,2]`, "unflatten"); err == nil {
		t.Error("unflatten of non-object should error")
	}
}

func TestJSONFlattenCmd_RoundTrip(t *testing.T) {
	in := `{"users":[{"name":"bob","roles":["a"]}],"meta":{"v":null}}`
	flat, err := runJSONSub(t, in, "flatten")
	if err != nil {
		t.Fatal(err)
	}
	out, err := runJSONSub(t, strings.TrimSpace(flat), "unflatten")
	if err != nil {
		t.Fatal(err)
	}
	// 规范化（忽略 key 顺序）后比较
	if canonJSON(t, out) != canonJSON(t, in) {
		t.Errorf("round-trip mismatch:\n got:  %s\n want: %s", canonJSON(t, out), canonJSON(t, in))
	}
}

func TestJSONFlattenCmd_InvalidJSON(t *testing.T) {
	if _, err := runJSONSub(t, `{bad`, "flatten"); err == nil {
		t.Error("invalid JSON should error")
	}
}
