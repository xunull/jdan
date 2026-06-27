package cli

import (
	"bytes"
	"encoding/json"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func runSecretsScan(t *testing.T, stdin string, args ...string) (out, errOut string, code int) {
	t.Helper()
	var o, e bytes.Buffer
	code = 0
	deps := secretsScanCmdDeps{
		out:    &o,
		errOut: &e,
		in:     strings.NewReader(stdin),
		exit:   func(c int) { code = c },
	}
	cmd := newSecretsScanCommand(deps)
	cmd.SetArgs(args)
	if err := cmd.Execute(); err != nil {
		t.Fatalf("execute: %v", err)
	}
	return o.String(), e.String(), code
}

// 前缀与主体拆开的字面量，避免源码出现连续的「真密钥形状」（否则 GitHub push
// protection / secret scanner 会把测试夹具误当真密钥拦截）。运行时拼成完整串。
var (
	fakeSecret = "ghp_" + "abcdefghijklmnopqrstuvwxyz0123456789"
	fakeAWS    = "AKIA" + "1234567890ABCDEF"
)

func TestSecretsScanCmd_FileFound(t *testing.T) {
	dir := t.TempDir()
	f := filepath.Join(dir, "leak.env")
	if err := os.WriteFile(f, []byte("TOKEN="+fakeSecret+"\n"), 0o600); err != nil {
		t.Fatal(err)
	}
	out, _, code := runSecretsScan(t, "", f)
	if code != 1 {
		t.Errorf("发现 secret 应 exit 1，得到 %d", code)
	}
	if !strings.Contains(out, "github-pat") {
		t.Errorf("应报 github-pat:\n%s", out)
	}
	// 安全：完整 secret 不得出现
	if strings.Contains(out, fakeSecret) {
		t.Errorf("输出泄露了完整 secret:\n%s", out)
	}
}

func TestSecretsScanCmd_Clean(t *testing.T) {
	dir := t.TempDir()
	f := filepath.Join(dir, "ok.txt")
	if err := os.WriteFile(f, []byte("hello world\njust some text\n"), 0o600); err != nil {
		t.Fatal(err)
	}
	_, _, code := runSecretsScan(t, "", f)
	if code != 0 {
		t.Errorf("无发现应 exit 0，得到 %d", code)
	}
}

func TestSecretsScanCmd_Stdin(t *testing.T) {
	out, _, code := runSecretsScan(t, "api="+fakeAWS+"\n")
	if code != 1 {
		t.Errorf("stdin 发现应 exit 1，得到 %d", code)
	}
	if !strings.Contains(out, "aws-access-key") {
		t.Errorf("stdin 应报 aws-access-key:\n%s", out)
	}
}

func TestSecretsScanCmd_JSONNoLeak(t *testing.T) {
	dir := t.TempDir()
	f := filepath.Join(dir, "leak.env")
	if err := os.WriteFile(f, []byte("TOKEN="+fakeSecret+"\n"), 0o600); err != nil {
		t.Fatal(err)
	}
	out, _, _ := runSecretsScan(t, "", f, "--json")
	var v []map[string]any
	if err := json.Unmarshal([]byte(out), &v); err != nil {
		t.Fatalf("--json 应合法:\n%s", out)
	}
	if len(v) == 0 {
		t.Error("json 应有命中")
	}
	if strings.Contains(out, fakeSecret) {
		t.Errorf("--json 泄露了完整 secret:\n%s", out)
	}
}

func TestSecretsScanCmd_SkipsNodeModules(t *testing.T) {
	dir := t.TempDir()
	nm := filepath.Join(dir, "node_modules")
	if err := os.MkdirAll(nm, 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(nm, "x.js"), []byte("k="+fakeAWS+"\n"), 0o600); err != nil {
		t.Fatal(err)
	}
	_, _, code := runSecretsScan(t, "", dir)
	if code != 0 {
		t.Errorf("默认应跳过 node_modules → exit 0，得到 %d", code)
	}
	// -a 才扫到
	_, _, codeAll := runSecretsScan(t, "", dir, "-a")
	if codeAll != 1 {
		t.Errorf("-a 应扫 node_modules → exit 1，得到 %d", codeAll)
	}
}

func TestSecretsScanCmd_SkipsBinary(t *testing.T) {
	dir := t.TempDir()
	f := filepath.Join(dir, "bin.dat")
	// 含 NUL → 二进制，默认跳过（即便里面有像密钥的串）
	if err := os.WriteFile(f, []byte(fakeAWS+"\x00\x00binary"), 0o600); err != nil {
		t.Fatal(err)
	}
	_, _, code := runSecretsScan(t, "", dir)
	if code != 0 {
		t.Errorf("默认应跳过二进制文件 → exit 0，得到 %d", code)
	}
}
