package cli

import (
	"bytes"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func TestB64Enc_Arg(t *testing.T) {
	var buf bytes.Buffer
	cmd := newB64Command(b64CmdDeps{out: &buf})
	cmd.SetArgs([]string{"enc", "hello"})
	if err := cmd.Execute(); err != nil {
		t.Fatal(err)
	}
	got := strings.TrimSpace(buf.String())
	if got != "aGVsbG8=" {
		t.Errorf("got %q, want aGVsbG8=", got)
	}
}

func TestB64Enc_Stdin(t *testing.T) {
	var buf bytes.Buffer
	cmd := newB64Command(b64CmdDeps{
		out: &buf,
		in:  strings.NewReader("hello"),
	})
	cmd.SetArgs([]string{"enc"})
	if err := cmd.Execute(); err != nil {
		t.Fatal(err)
	}
	got := strings.TrimSpace(buf.String())
	if got != "aGVsbG8=" {
		t.Errorf("stdin: got %q, want aGVsbG8=", got)
	}
}

func TestB64Dec_Arg(t *testing.T) {
	var buf bytes.Buffer
	cmd := newB64Command(b64CmdDeps{out: &buf})
	cmd.SetArgs([]string{"dec", "aGVsbG8="})
	if err := cmd.Execute(); err != nil {
		t.Fatal(err)
	}
	if buf.String() != "hello" {
		t.Errorf("got %q, want hello", buf.String())
	}
}

func TestB64Dec_AutoDetectsPadding(t *testing.T) {
	// 没 padding 的 base64 应当自动 fallback 到 RawStdEncoding
	var buf bytes.Buffer
	cmd := newB64Command(b64CmdDeps{out: &buf})
	cmd.SetArgs([]string{"dec", "aGVsbG8"}) // no =
	if err := cmd.Execute(); err != nil {
		t.Fatal(err)
	}
	if buf.String() != "hello" {
		t.Errorf("no-pad input: got %q, want hello", buf.String())
	}
}

func TestB64Enc_URLSafe(t *testing.T) {
	var buf bytes.Buffer
	cmd := newB64Command(b64CmdDeps{out: &buf})
	cmd.SetArgs([]string{"enc", "data?", "--url"})
	if err := cmd.Execute(); err != nil {
		t.Fatal(err)
	}
	got := strings.TrimSpace(buf.String())
	// "data?" std: ZGF0YT8= / URL-safe 也是 ZGF0YT8=（这里没出现 +/- 等）
	if got != "ZGF0YT8=" {
		t.Errorf("URL encode got %q", got)
	}
}

func TestB64Enc_NoPad(t *testing.T) {
	var buf bytes.Buffer
	cmd := newB64Command(b64CmdDeps{out: &buf})
	cmd.SetArgs([]string{"enc", "data", "--no-pad"})
	if err := cmd.Execute(); err != nil {
		t.Fatal(err)
	}
	got := strings.TrimSpace(buf.String())
	if got != "ZGF0YQ" { // 4 bytes → 不带 =
		t.Errorf("no-pad got %q, want ZGF0YQ", got)
	}
}

func TestB64_FileIO(t *testing.T) {
	dir := t.TempDir()
	inPath := filepath.Join(dir, "in.txt")
	outPath := filepath.Join(dir, "out.b64")
	_ = os.WriteFile(inPath, []byte("hello"), 0o644)

	cmd := newB64Command(b64CmdDeps{out: &bytes.Buffer{}})
	cmd.SetArgs([]string{"enc", "-i", inPath, "-o", outPath})
	if err := cmd.Execute(); err != nil {
		t.Fatal(err)
	}
	body, _ := os.ReadFile(outPath)
	if string(body) != "aGVsbG8=" {
		t.Errorf("file write got %q", body)
	}
}

func TestB64Dec_FromStdin(t *testing.T) {
	var buf bytes.Buffer
	cmd := newB64Command(b64CmdDeps{
		out: &buf,
		in:  strings.NewReader("aGVsbG8=\n"), // 末尾换行常见，应当被 trim
	})
	cmd.SetArgs([]string{"dec"})
	if err := cmd.Execute(); err != nil {
		t.Fatal(err)
	}
	if buf.String() != "hello" {
		t.Errorf("got %q", buf.String())
	}
}

func TestB64Dec_InvalidInput_Errors(t *testing.T) {
	cmd := newB64Command(b64CmdDeps{out: &bytes.Buffer{}})
	cmd.SetArgs([]string{"dec", "!!!not base64!!!"})
	if err := cmd.Execute(); err == nil {
		t.Error("invalid input should error")
	}
}

func TestURLEnc_Path(t *testing.T) {
	var buf bytes.Buffer
	cmd := newURLCommand(urlCmdDeps{out: &buf})
	cmd.SetArgs([]string{"enc", "hello world"})
	if err := cmd.Execute(); err != nil {
		t.Fatal(err)
	}
	got := strings.TrimSpace(buf.String())
	if got != "hello%20world" {
		t.Errorf("got %q, want hello%%20world", got)
	}
}

func TestURLEnc_Query(t *testing.T) {
	var buf bytes.Buffer
	cmd := newURLCommand(urlCmdDeps{out: &buf})
	cmd.SetArgs([]string{"enc", "a b", "--query"})
	if err := cmd.Execute(); err != nil {
		t.Fatal(err)
	}
	got := strings.TrimSpace(buf.String())
	if got != "a+b" {
		t.Errorf("query mode: got %q, want a+b", got)
	}
}

func TestURLDec_Path(t *testing.T) {
	var buf bytes.Buffer
	cmd := newURLCommand(urlCmdDeps{out: &buf})
	cmd.SetArgs([]string{"dec", "hello%20world"})
	if err := cmd.Execute(); err != nil {
		t.Fatal(err)
	}
	got := strings.TrimSpace(buf.String())
	if got != "hello world" {
		t.Errorf("got %q", got)
	}
}

func TestURLDec_Query(t *testing.T) {
	var buf bytes.Buffer
	cmd := newURLCommand(urlCmdDeps{out: &buf})
	cmd.SetArgs([]string{"dec", "a+b", "--query"})
	if err := cmd.Execute(); err != nil {
		t.Fatal(err)
	}
	got := strings.TrimSpace(buf.String())
	if got != "a b" {
		t.Errorf("query dec: got %q, want 'a b'", got)
	}
}

func TestURLEnc_StdinTrimsNewline(t *testing.T) {
	var buf bytes.Buffer
	cmd := newURLCommand(urlCmdDeps{
		out: &buf,
		in:  strings.NewReader("hello world\n"), // trailing newline
	})
	cmd.SetArgs([]string{"enc"})
	if err := cmd.Execute(); err != nil {
		t.Fatal(err)
	}
	got := strings.TrimSpace(buf.String())
	// 末尾换行不应当被编码成 %0A
	if strings.Contains(got, "%0A") {
		t.Errorf("stdin trailing newline leaked to output: %q", got)
	}
}

func TestURLDec_InvalidInput_Errors(t *testing.T) {
	cmd := newURLCommand(urlCmdDeps{out: &bytes.Buffer{}})
	cmd.SetArgs([]string{"dec", "bad%ZZ"})
	if err := cmd.Execute(); err == nil {
		t.Error("invalid percent-encoding should error")
	}
}
