package cli

import (
	"bytes"
	"strings"
	"testing"
)

func TestNum_Decimal(t *testing.T) {
	var buf bytes.Buffer
	cmd := newNumCommand(numCmdDeps{out: &buf})
	cmd.SetArgs([]string{"255"})
	if err := cmd.Execute(); err != nil {
		t.Fatal(err)
	}
	out := buf.String()
	for _, want := range []string{
		"Decimal:  255",
		"Hex:      0xFF",
		"Binary:   0b11111111",
		"Octal:    0o377",
		"width 8",
	} {
		if !strings.Contains(out, want) {
			t.Errorf("missing %q in:\n%s", want, out)
		}
	}
}

func TestNum_HexInput(t *testing.T) {
	var buf bytes.Buffer
	cmd := newNumCommand(numCmdDeps{out: &buf})
	cmd.SetArgs([]string{"0xDEADBEEF"})
	if err := cmd.Execute(); err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(buf.String(), "Decimal:  3735928559") {
		t.Errorf("got:\n%s", buf.String())
	}
}

func TestNum_OctalInput(t *testing.T) {
	var buf bytes.Buffer
	cmd := newNumCommand(numCmdDeps{out: &buf})
	cmd.SetArgs([]string{"0o755"})
	if err := cmd.Execute(); err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(buf.String(), "Decimal:  493") {
		t.Errorf("got:\n%s", buf.String())
	}
}

func TestNum_BitsFlag(t *testing.T) {
	var buf bytes.Buffer
	cmd := newNumCommand(numCmdDeps{out: &buf})
	cmd.SetArgs([]string{"0b10110", "--bits"})
	if err := cmd.Execute(); err != nil {
		t.Fatal(err)
	}
	out := buf.String()
	if !strings.Contains(out, "bit:") || !strings.Contains(out, "val:") {
		t.Errorf("--bits should show bit/val rows:\n%s", out)
	}
}

func TestNum_WidthPadding(t *testing.T) {
	var buf bytes.Buffer
	cmd := newNumCommand(numCmdDeps{out: &buf})
	cmd.SetArgs([]string{"0xFF", "--width", "16"})
	if err := cmd.Execute(); err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(buf.String(), "Binary:   0b0000000011111111") {
		t.Errorf("got:\n%s", buf.String())
	}
}

func TestNum_JSON(t *testing.T) {
	var buf bytes.Buffer
	cmd := newNumCommand(numCmdDeps{out: &buf})
	cmd.SetArgs([]string{"255", "--json"})
	if err := cmd.Execute(); err != nil {
		t.Fatal(err)
	}
	out := buf.String()
	for _, want := range []string{
		`"decimal": 255`,
		`"hex": "0xFF"`,
		`"detected_base": "decimal"`,
		`"bit_width": 8`,
	} {
		if !strings.Contains(out, want) {
			t.Errorf("missing %q in:\n%s", want, out)
		}
	}
}

func TestNum_InvalidInput(t *testing.T) {
	cmd := newNumCommand(numCmdDeps{out: &bytes.Buffer{}})
	cmd.SetArgs([]string{"0xZZ"})
	if err := cmd.Execute(); err == nil {
		t.Error("invalid input should error")
	}
}

func TestNum_Overflow(t *testing.T) {
	cmd := newNumCommand(numCmdDeps{out: &bytes.Buffer{}})
	cmd.SetArgs([]string{"99999999999999999999999"})
	if err := cmd.Execute(); err == nil {
		t.Error("overflow should error")
	}
}

// ---- bit 子命令 ----

func TestNumBit_AND(t *testing.T) {
	var buf bytes.Buffer
	cmd := newNumCommand(numCmdDeps{out: &buf})
	cmd.SetArgs([]string{"bit", "0xFF AND 0x0F"})
	if err := cmd.Execute(); err != nil {
		t.Fatal(err)
	}
	got := strings.TrimSpace(buf.String())
	if got != "0xF  (15, 0b1111)" {
		t.Errorf("got %q", got)
	}
}

func TestNumBit_ShiftLeft(t *testing.T) {
	var buf bytes.Buffer
	cmd := newNumCommand(numCmdDeps{out: &buf})
	cmd.SetArgs([]string{"bit", "1 << 8"})
	if err := cmd.Execute(); err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(buf.String(), "0x100  (256") {
		t.Errorf("got %q", buf.String())
	}
}

func TestNumBit_NOTWithWidth(t *testing.T) {
	var buf bytes.Buffer
	cmd := newNumCommand(numCmdDeps{out: &buf})
	cmd.SetArgs([]string{"bit", "NOT 0xFF", "--width", "8"})
	if err := cmd.Execute(); err != nil {
		t.Fatal(err)
	}
	if !strings.HasPrefix(strings.TrimSpace(buf.String()), "0x0  (0") {
		t.Errorf("NOT 0xFF (8-bit) got %q", buf.String())
	}
}

func TestNumBit_MultiArg(t *testing.T) {
	// "0xFF AND 0x0F" 作为多个 arg 传入（不加引号）也能用
	var buf bytes.Buffer
	cmd := newNumCommand(numCmdDeps{out: &buf})
	cmd.SetArgs([]string{"bit", "0xFF", "AND", "0x0F"})
	if err := cmd.Execute(); err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(buf.String(), "0xF  (15") {
		t.Errorf("multi-arg got %q", buf.String())
	}
}

func TestNumBit_JSON(t *testing.T) {
	var buf bytes.Buffer
	cmd := newNumCommand(numCmdDeps{out: &buf})
	cmd.SetArgs([]string{"bit", "0xFF AND 0x0F", "--json"})
	if err := cmd.Execute(); err != nil {
		t.Fatal(err)
	}
	out := buf.String()
	for _, want := range []string{
		`"op": "AND"`,
		`"a": 255`,
		`"b": 15`,
		`"result": 15`,
		`"hex": "0xF"`,
	} {
		if !strings.Contains(out, want) {
			t.Errorf("missing %q in:\n%s", want, out)
		}
	}
}

func TestNumBit_InvalidExpr(t *testing.T) {
	cmd := newNumCommand(numCmdDeps{out: &bytes.Buffer{}})
	cmd.SetArgs([]string{"bit", "5 FOO 3"})
	if err := cmd.Execute(); err == nil {
		t.Error("invalid operator should error")
	}
}
