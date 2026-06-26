package cli

import (
	"bytes"
	"encoding/json"
	"strings"
	"testing"
)

func runMorse(t *testing.T, stdin string, args ...string) (string, string, error) {
	t.Helper()
	var out, errOut bytes.Buffer
	deps := morseCmdDeps{out: &out, errOut: &errOut}
	if stdin != "" {
		deps.in = strings.NewReader(stdin)
	}
	cmd := newMorseCommand(deps)
	cmd.SetArgs(args)
	err := cmd.Execute()
	return out.String(), errOut.String(), err
}

func TestMorseCmd_Encode(t *testing.T) {
	out, _, err := runMorse(t, "", "SOS")
	if err != nil {
		t.Fatal(err)
	}
	if strings.TrimSpace(out) != "... --- ..." {
		t.Errorf("got %q", out)
	}
}

func TestMorseCmd_AutoDecode(t *testing.T) {
	out, _, err := runMorse(t, "", "... --- ...")
	if err != nil {
		t.Fatal(err)
	}
	if strings.TrimSpace(out) != "SOS" {
		t.Errorf("auto-decode wrong: %q", out)
	}
}

func TestMorseCmd_ForceEncode(t *testing.T) {
	// "." is ambiguous: auto-detect decodes it (→ E), but --encode treats it as
	// the period character → ".-.-.-". This verifies --encode overrides auto-detect.
	out, _, err := runMorse(t, "", ".", "--encode")
	if err != nil {
		t.Fatal(err)
	}
	if strings.TrimSpace(out) != ".-.-.-" {
		t.Errorf("force-encode of '.' (period char) should be '.-.-.-': %q", out)
	}
}

func TestMorseCmd_ForceDecode(t *testing.T) {
	out, _, err := runMorse(t, "", "...", "-d")
	if err != nil {
		t.Fatal(err)
	}
	if strings.TrimSpace(out) != "S" {
		t.Errorf("force-decode wrong: %q", out)
	}
}

func TestMorseCmd_Stdin(t *testing.T) {
	out, _, err := runMorse(t, "SOS\n")
	if err != nil {
		t.Fatal(err)
	}
	if strings.TrimSpace(out) != "... --- ..." {
		t.Errorf("stdin encode wrong: %q", out)
	}
}

func TestMorseCmd_SkipNoteToStderr(t *testing.T) {
	out, errOut, err := runMorse(t, "", "hi你好")
	if err != nil {
		t.Fatal(err)
	}
	if strings.Contains(out, "跳过") {
		t.Error("skip note must go to stderr, not stdout (keeps pipes clean)")
	}
	if !strings.Contains(errOut, "跳过 2") {
		t.Errorf("expected skip note on stderr: %q", errOut)
	}
}

func TestMorseCmd_JSON(t *testing.T) {
	out, _, err := runMorse(t, "", "SOS", "--json")
	if err != nil {
		t.Fatal(err)
	}
	var v struct {
		Direction string `json:"direction"`
		Output    string `json:"output"`
	}
	if err := json.Unmarshal([]byte(out), &v); err != nil {
		t.Fatalf("--json should emit valid JSON: %v\n%s", err, out)
	}
	if v.Direction != "encode" || v.Output != "... --- ..." {
		t.Errorf("got %+v", v)
	}
}

func TestMorseCmd_Empty(t *testing.T) {
	if _, _, err := runMorse(t, ""); err == nil {
		t.Error("empty input should error")
	}
}
