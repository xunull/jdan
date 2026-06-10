package cli

import (
	"bytes"
	crypto_rand "crypto/rand"
	"encoding/json"
	"errors"
	"io"
	"strings"
	"testing"

	"github.com/spf13/cobra"
)

// failingRandReader 在 Read 时返回固定错误，用于测试 CSPRNG 失败路径。
type failingRandReader struct{ err error }

func (f failingRandReader) Read(p []byte) (int, error) { return 0, f.err }

// fixedRandReader 返回循环重复的字节序列，用于精确测试。永远不返回错误。
type fixedRandReader struct {
	data []byte
	pos  int
}

func (r *fixedRandReader) Read(p []byte) (int, error) {
	for i := range p {
		if len(r.data) == 0 {
			return 0, io.EOF
		}
		p[i] = r.data[r.pos%len(r.data)]
		r.pos++
	}
	return len(p), nil
}

func newRandTestCmd(buf *bytes.Buffer, exit *exitTracker, reader io.Reader) *cobra.Command {
	if reader == nil {
		reader = crypto_rand.Reader
	}
	return newRandCommand(randCmdDeps{
		out:        buf,
		randReader: reader,
		exit:       exit.fn,
	})
}

// ------ password ------

func TestRandPassword_DefaultLength20(t *testing.T) {
	var buf bytes.Buffer
	ex := &exitTracker{}
	cmd := newRandTestCmd(&buf, ex, nil)
	cmd.SetArgs([]string{"password"})
	if err := cmd.Execute(); err != nil {
		t.Fatal(err)
	}
	s := strings.TrimSpace(buf.String())
	if len(s) != 20 {
		t.Errorf("password default length = %d, want 20", len(s))
	}
}

func TestRandPassword_LengthFlag(t *testing.T) {
	var buf bytes.Buffer
	ex := &exitTracker{}
	cmd := newRandTestCmd(&buf, ex, nil)
	cmd.SetArgs([]string{"password", "-l", "30"})
	if err := cmd.Execute(); err != nil {
		t.Fatal(err)
	}
	if len(strings.TrimSpace(buf.String())) != 30 {
		t.Errorf("expected length 30, got %d", len(strings.TrimSpace(buf.String())))
	}
}

func TestRandPassword_NoSymbols(t *testing.T) {
	var buf bytes.Buffer
	ex := &exitTracker{}
	cmd := newRandTestCmd(&buf, ex, nil)
	cmd.SetArgs([]string{"password", "--no-symbols", "-l", "40"})
	if err := cmd.Execute(); err != nil {
		t.Fatal(err)
	}
	s := strings.TrimSpace(buf.String())
	if strings.ContainsAny(s, "!@#$%^&*()-_=+") {
		t.Errorf("--no-symbols should not contain symbols, got %q", s)
	}
}

func TestRandPassword_IncludeAmbiguous(t *testing.T) {
	// 50 长度 50 的密码，应当能见到歧义字符
	var seenAmbig bool
	for i := 0; i < 50 && !seenAmbig; i++ {
		var buf bytes.Buffer
		ex := &exitTracker{}
		cmd := newRandTestCmd(&buf, ex, nil)
		cmd.SetArgs([]string{"password", "-l", "50", "--include-ambiguous"})
		if err := cmd.Execute(); err != nil {
			t.Fatal(err)
		}
		if strings.ContainsAny(buf.String(), "Il1O0") {
			seenAmbig = true
		}
	}
	if !seenAmbig {
		t.Error("--include-ambiguous: never saw ambiguous chars in 50 long-length passwords")
	}
}

func TestRandPassword_PropagatesCSPRNGError(t *testing.T) {
	var buf bytes.Buffer
	ex := &exitTracker{}
	cmd := newRandTestCmd(&buf, ex, failingRandReader{errors.New("CSPRNG down")})
	cmd.SilenceUsage = true
	cmd.SetArgs([]string{"password"})
	err := cmd.Execute()
	if err == nil || !strings.Contains(err.Error(), "CSPRNG down") {
		t.Errorf("expected CSPRNG failure, got: %v", err)
	}
}

// ------ Count + JSON + no-newline 共享 flag ------

func TestRandPassword_CountFlag(t *testing.T) {
	var buf bytes.Buffer
	ex := &exitTracker{}
	cmd := newRandTestCmd(&buf, ex, nil)
	cmd.SetArgs([]string{"password", "-c", "5"})
	if err := cmd.Execute(); err != nil {
		t.Fatal(err)
	}
	lines := strings.Split(strings.TrimSpace(buf.String()), "\n")
	if len(lines) != 5 {
		t.Errorf("expected 5 lines, got %d", len(lines))
	}
}

func TestRandHex_JSONIsStringArray(t *testing.T) {
	var buf bytes.Buffer
	ex := &exitTracker{}
	cmd := newRandTestCmd(&buf, ex, nil)
	cmd.SetArgs([]string{"hex", "-l", "8", "-c", "3", "--json"})
	if err := cmd.Execute(); err != nil {
		t.Fatal(err)
	}
	var arr []string
	if err := json.Unmarshal(buf.Bytes(), &arr); err != nil {
		t.Fatalf("invalid JSON: %v\n%s", err, buf.String())
	}
	if len(arr) != 3 {
		t.Errorf("expected 3 elements, got %d", len(arr))
	}
	for _, s := range arr {
		if len(s) != 16 {
			t.Errorf("hex(8) length = %d, want 16", len(s))
		}
	}
}

func TestRandHex_SingleJSONIsStillArray(t *testing.T) {
	// 即使 -c 1，JSON 输出仍是数组（设计文档明确规定）
	var buf bytes.Buffer
	ex := &exitTracker{}
	cmd := newRandTestCmd(&buf, ex, nil)
	cmd.SetArgs([]string{"hex", "-l", "8", "--json"})
	if err := cmd.Execute(); err != nil {
		t.Fatal(err)
	}
	var arr []string
	if err := json.Unmarshal(buf.Bytes(), &arr); err != nil {
		t.Fatalf("invalid JSON: %v", err)
	}
	if len(arr) != 1 {
		t.Errorf("expected single-element array, got %d", len(arr))
	}
}

func TestRandHex_NoNewline(t *testing.T) {
	var buf bytes.Buffer
	ex := &exitTracker{}
	cmd := newRandTestCmd(&buf, ex, nil)
	cmd.SetArgs([]string{"hex", "-l", "4", "--no-newline"})
	if err := cmd.Execute(); err != nil {
		t.Fatal(err)
	}
	if strings.Contains(buf.String(), "\n") {
		t.Errorf("--no-newline should produce no \\n, got %q", buf.String())
	}
}

func TestRandHex_NoNewlineWithCountErrors(t *testing.T) {
	var buf bytes.Buffer
	ex := &exitTracker{}
	cmd := newRandTestCmd(&buf, ex, nil)
	cmd.SilenceUsage = true
	cmd.SetArgs([]string{"hex", "--no-newline", "-c", "3"})
	if err := cmd.Execute(); err == nil {
		t.Error("--no-newline + -c 3 should error")
	}
}

func TestRandHex_JSONNoNewlineMutex(t *testing.T) {
	var buf bytes.Buffer
	ex := &exitTracker{}
	cmd := newRandTestCmd(&buf, ex, nil)
	cmd.SilenceUsage = true
	cmd.SetArgs([]string{"hex", "--json", "--no-newline"})
	if err := cmd.Execute(); err == nil {
		t.Error("--json + --no-newline should error")
	}
}

func TestRandHex_CountZeroErrors(t *testing.T) {
	var buf bytes.Buffer
	ex := &exitTracker{}
	cmd := newRandTestCmd(&buf, ex, nil)
	cmd.SilenceUsage = true
	cmd.SetArgs([]string{"hex", "-c", "0"})
	if err := cmd.Execute(); err == nil {
		t.Error("--count 0 should error")
	}
}

// ------ hex / base64 / base64url / base32 ------

func TestRandHex_DefaultByteLength32(t *testing.T) {
	var buf bytes.Buffer
	ex := &exitTracker{}
	cmd := newRandTestCmd(&buf, ex, nil)
	cmd.SetArgs([]string{"hex"})
	if err := cmd.Execute(); err != nil {
		t.Fatal(err)
	}
	if len(strings.TrimSpace(buf.String())) != 64 {
		t.Errorf("hex default = %d hex chars, want 64", len(strings.TrimSpace(buf.String())))
	}
}

func TestRandBase64URL_NoStandardChars(t *testing.T) {
	var buf bytes.Buffer
	ex := &exitTracker{}
	cmd := newRandTestCmd(&buf, ex, nil)
	cmd.SetArgs([]string{"base64url", "-l", "32"})
	if err := cmd.Execute(); err != nil {
		t.Fatal(err)
	}
	if strings.ContainsAny(buf.String(), "+/=") {
		t.Errorf("base64url leaked +/=: %q", buf.String())
	}
}

func TestRandBase32_RFC4648Charset(t *testing.T) {
	var buf bytes.Buffer
	ex := &exitTracker{}
	cmd := newRandTestCmd(&buf, ex, nil)
	cmd.SetArgs([]string{"base32", "-l", "20"})
	if err := cmd.Execute(); err != nil {
		t.Fatal(err)
	}
	s := strings.TrimSpace(buf.String())
	for _, c := range s {
		if !strings.ContainsRune("ABCDEFGHIJKLMNOPQRSTUVWXYZ234567=", c) {
			t.Errorf("non-RFC4648 char: %q in %q", c, s)
		}
	}
}

// ------ alnum ------

func TestRandAlnum_ExcludesAmbiguousByDefault(t *testing.T) {
	for i := 0; i < 50; i++ {
		var buf bytes.Buffer
		ex := &exitTracker{}
		cmd := newRandTestCmd(&buf, ex, nil)
		cmd.SetArgs([]string{"alnum", "-l", "30"})
		if err := cmd.Execute(); err != nil {
			t.Fatal(err)
		}
		if strings.ContainsAny(buf.String(), "Il1O0") {
			t.Fatalf("alnum default leaked ambiguous: %q", buf.String())
		}
	}
}

func TestRandAlnum_LengthOneAllowed(t *testing.T) {
	// 与 password 区别：alnum 无类约束，length=1 应当成功
	var buf bytes.Buffer
	ex := &exitTracker{}
	cmd := newRandTestCmd(&buf, ex, nil)
	cmd.SetArgs([]string{"alnum", "-l", "1"})
	if err := cmd.Execute(); err != nil {
		t.Errorf("alnum -l 1 should work (no class constraint), got: %v", err)
	}
}

// ------ uuid ------

func TestRandUUID_DefaultV4(t *testing.T) {
	var buf bytes.Buffer
	ex := &exitTracker{}
	cmd := newRandTestCmd(&buf, ex, nil)
	cmd.SetArgs([]string{"uuid"})
	if err := cmd.Execute(); err != nil {
		t.Fatal(err)
	}
	s := strings.TrimSpace(buf.String())
	if len(s) != 36 {
		t.Errorf("UUID length = %d", len(s))
	}
	if s[14] != '4' {
		t.Errorf("default should be v4, version nibble = %q", s[14])
	}
}

func TestRandUUID_V7(t *testing.T) {
	var buf bytes.Buffer
	ex := &exitTracker{}
	cmd := newRandTestCmd(&buf, ex, nil)
	cmd.SetArgs([]string{"uuid", "-V", "7"})
	if err := cmd.Execute(); err != nil {
		t.Fatal(err)
	}
	s := strings.TrimSpace(buf.String())
	if s[14] != '7' {
		t.Errorf("v7 version nibble = %q, want '7'", s[14])
	}
}

func TestRandUUID_RejectsUnsupportedVersions(t *testing.T) {
	for _, v := range []string{"1", "3", "5", "0", "8"} {
		var buf bytes.Buffer
		ex := &exitTracker{}
		cmd := newRandTestCmd(&buf, ex, nil)
		cmd.SilenceUsage = true
		cmd.SetArgs([]string{"uuid", "-V", v})
		if err := cmd.Execute(); err == nil {
			t.Errorf("UUID -V %s should error", v)
		}
	}
}

// ------ int ------

func TestRandInt_BasicRange(t *testing.T) {
	for i := 0; i < 50; i++ {
		var buf bytes.Buffer
		ex := &exitTracker{}
		cmd := newRandTestCmd(&buf, ex, nil)
		cmd.SetArgs([]string{"int", "1", "10"})
		if err := cmd.Execute(); err != nil {
			t.Fatal(err)
		}
		var n int
		if err := json.Unmarshal([]byte(strings.TrimSpace(buf.String())), &n); err != nil {
			t.Fatalf("not a valid int: %q", buf.String())
		}
		if n < 1 || n > 10 {
			t.Errorf("got %d, want [1,10]", n)
		}
	}
}

func TestRandInt_NegativeRange(t *testing.T) {
	var buf bytes.Buffer
	ex := &exitTracker{}
	cmd := newRandTestCmd(&buf, ex, nil)
	cmd.SetArgs([]string{"int", "-c", "3", "--", "-10", "10"})
	if err := cmd.Execute(); err != nil {
		t.Fatal(err)
	}
	lines := strings.Split(strings.TrimSpace(buf.String()), "\n")
	if len(lines) != 3 {
		t.Errorf("expected 3 lines, got %d", len(lines))
	}
}

func TestRandInt_RejectsMaxLessThanMin(t *testing.T) {
	var buf bytes.Buffer
	ex := &exitTracker{}
	cmd := newRandTestCmd(&buf, ex, nil)
	cmd.SilenceUsage = true
	cmd.SetArgs([]string{"int", "10", "5"})
	if err := cmd.Execute(); err == nil {
		t.Error("max < min should error")
	}
}

func TestRandInt_RejectsNonIntArg(t *testing.T) {
	var buf bytes.Buffer
	ex := &exitTracker{}
	cmd := newRandTestCmd(&buf, ex, nil)
	cmd.SilenceUsage = true
	cmd.SetArgs([]string{"int", "abc", "10"})
	if err := cmd.Execute(); err == nil {
		t.Error("non-int min should error")
	}
}

func TestRandInt_JSONIsIntArray(t *testing.T) {
	var buf bytes.Buffer
	ex := &exitTracker{}
	cmd := newRandTestCmd(&buf, ex, nil)
	cmd.SetArgs([]string{"int", "1", "5", "-c", "4", "--json"})
	if err := cmd.Execute(); err != nil {
		t.Fatal(err)
	}
	var arr []int64
	if err := json.Unmarshal(buf.Bytes(), &arr); err != nil {
		t.Fatalf("invalid JSON int array: %v\n%s", err, buf.String())
	}
	if len(arr) != 4 {
		t.Errorf("got %d elements, want 4", len(arr))
	}
}

func TestRandInt_RequiresTwoArgs(t *testing.T) {
	var buf bytes.Buffer
	ex := &exitTracker{}
	cmd := newRandTestCmd(&buf, ex, nil)
	cmd.SilenceUsage = true
	cmd.SetArgs([]string{"int", "5"})
	if err := cmd.Execute(); err == nil {
		t.Error("int with 1 arg should error")
	}
}

// ------ word ------

func TestRandWord_DefaultSixWords(t *testing.T) {
	var buf bytes.Buffer
	ex := &exitTracker{}
	cmd := newRandTestCmd(&buf, ex, nil)
	cmd.SetArgs([]string{"word"})
	if err := cmd.Execute(); err != nil {
		t.Fatal(err)
	}
	s := strings.TrimSpace(buf.String())
	parts := strings.Split(s, "-")
	if len(parts) != 6 {
		t.Errorf("default words = %d, want 6 (output: %q)", len(parts), s)
	}
}

func TestRandWord_CustomWordCount(t *testing.T) {
	var buf bytes.Buffer
	ex := &exitTracker{}
	cmd := newRandTestCmd(&buf, ex, nil)
	cmd.SetArgs([]string{"word", "-w", "8"})
	if err := cmd.Execute(); err != nil {
		t.Fatal(err)
	}
	parts := strings.Split(strings.TrimSpace(buf.String()), "-")
	if len(parts) != 8 {
		t.Errorf("expected 8 words, got %d", len(parts))
	}
}

func TestRandWord_CustomSep(t *testing.T) {
	var buf bytes.Buffer
	ex := &exitTracker{}
	cmd := newRandTestCmd(&buf, ex, nil)
	cmd.SetArgs([]string{"word", "--sep", "_"})
	if err := cmd.Execute(); err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(buf.String(), "_") {
		t.Errorf("expected _ separator in output: %q", buf.String())
	}
}

func TestRandWord_MultiplePassphrases(t *testing.T) {
	var buf bytes.Buffer
	ex := &exitTracker{}
	cmd := newRandTestCmd(&buf, ex, nil)
	cmd.SetArgs([]string{"word", "-c", "3"})
	if err := cmd.Execute(); err != nil {
		t.Fatal(err)
	}
	lines := strings.Split(strings.TrimSpace(buf.String()), "\n")
	if len(lines) != 3 {
		t.Errorf("expected 3 passphrases, got %d", len(lines))
	}
}

// ------ rand 父命令 ------

func TestRand_NoSubcommandShowsHelp(t *testing.T) {
	var buf bytes.Buffer
	ex := &exitTracker{}
	cmd := newRandTestCmd(&buf, ex, nil)
	// 父命令 cmd 本身就是 rand；不带任何 args → cobra 默认显示 help（不报错）
	cmd.SetArgs([]string{})
	err := cmd.Execute()
	if err != nil {
		t.Errorf("rand 无子命令不应报错，got: %v", err)
	}
}
