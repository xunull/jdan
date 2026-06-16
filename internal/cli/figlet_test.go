package cli

import (
	"bytes"
	"strings"
	"testing"
)

func TestFiglet_Basic(t *testing.T) {
	var buf bytes.Buffer
	cmd := newFigletCommand(figletCmdDeps{out: &buf})
	cmd.SetArgs([]string{"I"})
	if err := cmd.Execute(); err != nil {
		t.Fatal(err)
	}
	out := buf.String()
	if !strings.Contains(out, "###") {
		t.Errorf("got:\n%s", out)
	}
	// standard 高度 5
	if n := strings.Count(strings.TrimRight(out, "\n"), "\n") + 1; n != 5 {
		t.Errorf("expected 5 lines, got %d:\n%s", n, out)
	}
}

func TestFiglet_MultiArgJoin(t *testing.T) {
	var buf bytes.Buffer
	cmd := newFigletCommand(figletCmdDeps{out: &buf})
	cmd.SetArgs([]string{"A", "B"})
	if err := cmd.Execute(); err != nil {
		t.Fatal(err)
	}
	// 应当渲染出来（不空）
	if strings.TrimSpace(buf.String()) == "" {
		t.Error("multi-arg should render")
	}
}

func TestFiglet_BlockFont(t *testing.T) {
	var buf bytes.Buffer
	cmd := newFigletCommand(figletCmdDeps{out: &buf})
	cmd.SetArgs([]string{"OK", "--font", "block"})
	if err := cmd.Execute(); err != nil {
		t.Fatal(err)
	}
	if !strings.ContainsRune(buf.String(), '█') {
		t.Errorf("block font should use █:\n%s", buf.String())
	}
}

func TestFiglet_Stdin(t *testing.T) {
	var buf bytes.Buffer
	cmd := newFigletCommand(figletCmdDeps{
		out: &buf,
		in:  strings.NewReader("hi\n"),
	})
	cmd.SetArgs([]string{})
	if err := cmd.Execute(); err != nil {
		t.Fatal(err)
	}
	if strings.TrimSpace(buf.String()) == "" {
		t.Error("stdin should render")
	}
}

func TestFiglet_List(t *testing.T) {
	var buf bytes.Buffer
	cmd := newFigletCommand(figletCmdDeps{out: &buf})
	cmd.SetArgs([]string{"--list"})
	if err := cmd.Execute(); err != nil {
		t.Fatal(err)
	}
	out := buf.String()
	if !strings.Contains(out, "standard") || !strings.Contains(out, "block") {
		t.Errorf("--list should show fonts:\n%s", out)
	}
}

func TestFiglet_UnknownFont(t *testing.T) {
	cmd := newFigletCommand(figletCmdDeps{out: &bytes.Buffer{}})
	cmd.SetArgs([]string{"hi", "--font", "nope"})
	if err := cmd.Execute(); err == nil {
		t.Error("unknown font should error")
	}
}

func TestFiglet_EmptyText(t *testing.T) {
	cmd := newFigletCommand(figletCmdDeps{
		out: &bytes.Buffer{},
		in:  strings.NewReader(""),
	})
	cmd.SetArgs([]string{})
	if err := cmd.Execute(); err == nil {
		t.Error("empty text should error")
	}
}

func TestFiglet_Center(t *testing.T) {
	var buf bytes.Buffer
	cmd := newFigletCommand(figletCmdDeps{out: &buf})
	cmd.SetArgs([]string{"I", "--center", "--width", "40"})
	if err := cmd.Execute(); err != nil {
		t.Fatal(err)
	}
	// 居中后有前导空格
	if !strings.Contains(buf.String(), "   ") {
		t.Errorf("centered should have leading spaces:\n%s", buf.String())
	}
}
