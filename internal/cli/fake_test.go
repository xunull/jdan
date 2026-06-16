package cli

import (
	"bytes"
	"encoding/json"
	"strings"
	"testing"

	"github.com/xunull/jdan/internal/fake"
)

func runFake(t *testing.T, args ...string) string {
	t.Helper()
	var buf bytes.Buffer
	cmd := newFakeCommand(fakeCmdDeps{out: &buf})
	cmd.SetArgs(args)
	if err := cmd.Execute(); err != nil {
		t.Fatalf("args %v: %v", args, err)
	}
	return buf.String()
}

func TestFakeCmd_Basic(t *testing.T) {
	out := runFake(t, "name", "--seed", "42")
	if strings.TrimSpace(out) == "" {
		t.Error("should produce a name")
	}
	if lines := strings.Count(strings.TrimRight(out, "\n"), "\n") + 1; lines != 1 {
		t.Errorf("expected 1 line, got %d", lines)
	}
}

func TestFakeCmd_Reproducible(t *testing.T) {
	a := runFake(t, "name", "--seed", "42", "-n", "3")
	b := runFake(t, "name", "--seed", "42", "-n", "3")
	if a != b {
		t.Errorf("same seed should give same output:\n%q\nvs\n%q", a, b)
	}
}

func TestFakeCmd_Count(t *testing.T) {
	out := runFake(t, "email", "--seed", "1", "-n", "5")
	lines := strings.Split(strings.TrimRight(out, "\n"), "\n")
	if len(lines) != 5 {
		t.Errorf("expected 5 lines, got %d", len(lines))
	}
	for _, l := range lines {
		if !strings.Contains(l, "@") {
			t.Errorf("not an email: %q", l)
		}
	}
}

func TestFakeCmd_JSONArray(t *testing.T) {
	out := runFake(t, "uuid", "--seed", "3", "--json", "-n", "4")
	var got []string
	if err := json.Unmarshal([]byte(out), &got); err != nil {
		t.Fatalf("output not valid JSON array: %v\n%s", err, out)
	}
	if len(got) != 4 {
		t.Errorf("expected 4 elements, got %d", len(got))
	}
}

func TestFakeCmd_CompositeJSON(t *testing.T) {
	out := runFake(t, "--json", "--seed", "1", "-n", "2")
	var people []fake.Person
	if err := json.Unmarshal([]byte(out), &people); err != nil {
		t.Fatalf("composite not valid JSON: %v\n%s", err, out)
	}
	if len(people) != 2 {
		t.Fatalf("expected 2 people, got %d", len(people))
	}
	if people[0].Name == "" || people[0].Email == "" {
		t.Errorf("person not populated: %+v", people[0])
	}
}

func TestFakeCmd_IntRange(t *testing.T) {
	out := runFake(t, "int", "--seed", "1", "--min", "1", "--max", "6", "-n", "100")
	for _, l := range strings.Split(strings.TrimRight(out, "\n"), "\n") {
		if l < "1" || l > "6" || len(l) != 1 {
			// 粗略检查：1-6 都是单字符
			t.Errorf("int out of [1,6]: %q", l)
		}
	}
}

func TestFakeCmd_SentenceWords(t *testing.T) {
	out := strings.TrimSpace(runFake(t, "sentence", "--seed", "1", "--words", "3"))
	words := strings.Fields(strings.TrimSuffix(out, "."))
	if len(words) != 3 {
		t.Errorf("expected 3 words, got %d: %q", len(words), out)
	}
}

func TestFakeCmd_List(t *testing.T) {
	out := runFake(t, "--list")
	for _, typ := range fake.SupportedTypes {
		if !strings.Contains(out, typ) {
			t.Errorf("--list missing %q:\n%s", typ, out)
		}
	}
}

func TestFakeCmd_UnknownType(t *testing.T) {
	var buf bytes.Buffer
	cmd := newFakeCommand(fakeCmdDeps{out: &buf})
	cmd.SetArgs([]string{"nope"})
	if err := cmd.Execute(); err == nil {
		t.Error("unknown type should error")
	}
}

func TestFakeCmd_NoTypeNoJSONErrors(t *testing.T) {
	var buf bytes.Buffer
	cmd := newFakeCommand(fakeCmdDeps{out: &buf})
	cmd.SetArgs([]string{})
	if err := cmd.Execute(); err == nil {
		t.Error("no type without --json should error")
	}
}

func TestFakeCmd_BadCount(t *testing.T) {
	var buf bytes.Buffer
	cmd := newFakeCommand(fakeCmdDeps{out: &buf})
	cmd.SetArgs([]string{"name", "-n", "0"})
	if err := cmd.Execute(); err == nil {
		t.Error("count < 1 should error")
	}
}
