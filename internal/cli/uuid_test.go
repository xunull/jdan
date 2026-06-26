package cli

import (
	"bytes"
	"encoding/json"
	"strings"
	"testing"
)

func runUUID(t *testing.T, stdin string, args ...string) (string, error) {
	t.Helper()
	var buf bytes.Buffer
	deps := uuidCmdDeps{out: &buf}
	if stdin != "" {
		deps.in = strings.NewReader(stdin)
	}
	cmd := newUUIDCommand(deps)
	cmd.SetArgs(args)
	err := cmd.Execute()
	return buf.String(), err
}

func TestUUIDCmd_Inspect(t *testing.T) {
	out, err := runUUID(t, "", "3f2504e0-4f89-41d3-9a0c-0305e82c3301")
	if err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(out, "version:   4") || !strings.Contains(out, "RFC 4122") {
		t.Errorf("inspect output wrong:\n%s", out)
	}
}

func TestUUIDCmd_Stdin(t *testing.T) {
	out, err := runUUID(t, "00000000-0000-7000-8000-000000000000\n")
	if err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(out, "version:   7") {
		t.Errorf("stdin inspect wrong:\n%s", out)
	}
}

func TestUUIDCmd_JSON(t *testing.T) {
	out, err := runUUID(t, "", "3f2504e0-4f89-41d3-9a0c-0305e82c3301", "--json")
	if err != nil {
		t.Fatal(err)
	}
	var v map[string]any
	if err := json.Unmarshal([]byte(out), &v); err != nil {
		t.Fatalf("--json should emit valid JSON: %v\n%s", err, out)
	}
}

func TestUUIDCmd_Invalid(t *testing.T) {
	if _, err := runUUID(t, "", "not-a-uuid"); err == nil {
		t.Error("invalid uuid should error")
	}
}

func TestUUIDCmd_NoInput(t *testing.T) {
	if _, err := runUUID(t, ""); err == nil {
		t.Error("no token (no arg, empty stdin) should error")
	}
}

func TestUUIDCmd_NewV4(t *testing.T) {
	out, err := runUUID(t, "", "new")
	if err != nil {
		t.Fatal(err)
	}
	got := strings.TrimSpace(out)
	// 生成的应是合法 v4，能被自身检视器解析
	out2, err := runUUID(t, "", got)
	if err != nil {
		t.Fatalf("generated uuid %q not parseable: %v", got, err)
	}
	if !strings.Contains(out2, "version:   4") {
		t.Errorf("new default should be v4:\n%s", out2)
	}
}

func TestUUIDCmd_NewV7Count(t *testing.T) {
	out, err := runUUID(t, "", "new", "--v7", "-n", "3")
	if err != nil {
		t.Fatal(err)
	}
	lines := strings.Split(strings.TrimSpace(out), "\n")
	if len(lines) != 3 {
		t.Fatalf("want 3 lines, got %d: %q", len(lines), out)
	}
	out2, _ := runUUID(t, "", lines[0])
	if !strings.Contains(out2, "version:   7") {
		t.Errorf("--v7 should generate v7:\n%s", out2)
	}
}
