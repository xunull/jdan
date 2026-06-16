package cli

import (
	"bytes"
	"strings"
	"testing"
)

func TestCalc_Basic(t *testing.T) {
	var buf bytes.Buffer
	cmd := newCalcCommand(calcCmdDeps{out: &buf})
	cmd.SetArgs([]string{"3 * (4 + 5) / 2"})
	if err := cmd.Execute(); err != nil {
		t.Fatal(err)
	}
	if strings.TrimSpace(buf.String()) != "13.5" {
		t.Errorf("got %q", buf.String())
	}
}

func TestCalc_IntegerResultNoDecimal(t *testing.T) {
	var buf bytes.Buffer
	cmd := newCalcCommand(calcCmdDeps{out: &buf})
	cmd.SetArgs([]string{"2 ^ 10"})
	if err := cmd.Execute(); err != nil {
		t.Fatal(err)
	}
	if strings.TrimSpace(buf.String()) != "1024" {
		t.Errorf("integer result should have no decimal, got %q", buf.String())
	}
}

func TestCalc_LeadingNegative(t *testing.T) {
	var buf bytes.Buffer
	cmd := newCalcCommand(calcCmdDeps{out: &buf})
	// 关键：表达式以 '-' 开头不该被当 flag
	cmd.SetArgs([]string{"-5 + 3"})
	if err := cmd.Execute(); err != nil {
		t.Fatalf("leading negative should work: %v", err)
	}
	if strings.TrimSpace(buf.String()) != "-2" {
		t.Errorf("got %q", buf.String())
	}
}

func TestCalc_MultiArgJoin(t *testing.T) {
	var buf bytes.Buffer
	cmd := newCalcCommand(calcCmdDeps{out: &buf})
	// 不加引号，多个 arg 拼接
	cmd.SetArgs([]string{"2", "+", "3", "*", "4"})
	if err := cmd.Execute(); err != nil {
		t.Fatal(err)
	}
	if strings.TrimSpace(buf.String()) != "14" {
		t.Errorf("got %q", buf.String())
	}
}

func TestCalc_Function(t *testing.T) {
	var buf bytes.Buffer
	cmd := newCalcCommand(calcCmdDeps{out: &buf})
	cmd.SetArgs([]string{"sqrt(16)"})
	if err := cmd.Execute(); err != nil {
		t.Fatal(err)
	}
	if strings.TrimSpace(buf.String()) != "4" {
		t.Errorf("got %q", buf.String())
	}
}

func TestCalc_HexOutput(t *testing.T) {
	var buf bytes.Buffer
	cmd := newCalcCommand(calcCmdDeps{out: &buf})
	cmd.SetArgs([]string{"255 + 1", "--hex"})
	if err := cmd.Execute(); err != nil {
		t.Fatal(err)
	}
	if strings.TrimSpace(buf.String()) != "0x100" {
		t.Errorf("got %q", buf.String())
	}
}

func TestCalc_BinOutput(t *testing.T) {
	var buf bytes.Buffer
	cmd := newCalcCommand(calcCmdDeps{out: &buf})
	cmd.SetArgs([]string{"10", "--bin"})
	if err := cmd.Execute(); err != nil {
		t.Fatal(err)
	}
	if strings.TrimSpace(buf.String()) != "0b1010" {
		t.Errorf("got %q", buf.String())
	}
}

func TestCalc_HexNonIntegerErrors(t *testing.T) {
	cmd := newCalcCommand(calcCmdDeps{out: &bytes.Buffer{}})
	cmd.SetArgs([]string{"10 / 3", "--hex"})
	if err := cmd.Execute(); err == nil {
		t.Error("hex output of non-integer should error")
	}
}

func TestCalc_Precision(t *testing.T) {
	var buf bytes.Buffer
	cmd := newCalcCommand(calcCmdDeps{out: &buf})
	cmd.SetArgs([]string{"10 / 3", "--precision", "2"})
	if err := cmd.Execute(); err != nil {
		t.Fatal(err)
	}
	if strings.TrimSpace(buf.String()) != "3.33" {
		t.Errorf("got %q", buf.String())
	}
}

func TestCalc_PrecisionEquals(t *testing.T) {
	var buf bytes.Buffer
	cmd := newCalcCommand(calcCmdDeps{out: &buf})
	cmd.SetArgs([]string{"1 / 3", "--precision=4"})
	if err := cmd.Execute(); err != nil {
		t.Fatal(err)
	}
	if strings.TrimSpace(buf.String()) != "0.3333" {
		t.Errorf("got %q", buf.String())
	}
}

func TestCalc_JSON(t *testing.T) {
	var buf bytes.Buffer
	cmd := newCalcCommand(calcCmdDeps{out: &buf})
	cmd.SetArgs([]string{"2 ^ 8", "--json"})
	if err := cmd.Execute(); err != nil {
		t.Fatal(err)
	}
	out := buf.String()
	if !strings.Contains(out, `"expr": "2 ^ 8"`) || !strings.Contains(out, `"result": 256`) {
		t.Errorf("got %q", out)
	}
}

func TestCalc_Stdin(t *testing.T) {
	var buf bytes.Buffer
	cmd := newCalcCommand(calcCmdDeps{
		out: &buf,
		in:  strings.NewReader("1 + 2 * 3\n"),
	})
	cmd.SetArgs([]string{})
	if err := cmd.Execute(); err != nil {
		t.Fatal(err)
	}
	if strings.TrimSpace(buf.String()) != "7" {
		t.Errorf("got %q", buf.String())
	}
}

func TestCalc_InvalidExpr(t *testing.T) {
	cmd := newCalcCommand(calcCmdDeps{out: &bytes.Buffer{}})
	cmd.SetArgs([]string{"1 +"})
	if err := cmd.Execute(); err == nil {
		t.Error("invalid expression should error")
	}
}

func TestCalc_DivByZero(t *testing.T) {
	cmd := newCalcCommand(calcCmdDeps{out: &bytes.Buffer{}})
	cmd.SetArgs([]string{"1 / 0"})
	if err := cmd.Execute(); err == nil {
		t.Error("division by zero should error")
	}
}

func TestCalc_HexBinMutuallyExclusive(t *testing.T) {
	cmd := newCalcCommand(calcCmdDeps{out: &bytes.Buffer{}})
	cmd.SetArgs([]string{"255", "--hex", "--bin"})
	if err := cmd.Execute(); err == nil {
		t.Error("--hex and --bin together should error")
	}
}

func TestCalc_EmptyExpr(t *testing.T) {
	cmd := newCalcCommand(calcCmdDeps{
		out: &bytes.Buffer{},
		in:  strings.NewReader(""),
	})
	cmd.SetArgs([]string{})
	if err := cmd.Execute(); err == nil {
		t.Error("empty expression should error")
	}
}
