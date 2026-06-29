package cli

import (
	"bytes"
	"io"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func runHtpasswd(t *testing.T, in io.Reader, args ...string) (string, error) {
	t.Helper()
	var out bytes.Buffer
	cmd := newHtpasswdCommand(htpasswdCmdDeps{out: &out, in: in})
	cmd.SetArgs(args)
	cmd.SetOut(io.Discard)
	cmd.SetErr(io.Discard)
	err := cmd.Execute() // 先执行再读 buffer（求值顺序坑）
	return out.String(), err
}

func TestHtpasswd_Bcrypt(t *testing.T) {
	out, err := runHtpasswd(t, strings.NewReader("hunter2\n"), "alice")
	if err != nil {
		t.Fatal(err)
	}
	if !strings.HasPrefix(out, "alice:$2y$") {
		t.Errorf("应输出 alice:$2y$...，got %q", out)
	}
}

func TestHtpasswd_APR1(t *testing.T) {
	out, err := runHtpasswd(t, strings.NewReader("hunter2\n"), "alice", "--apr1")
	if err != nil {
		t.Fatal(err)
	}
	if !strings.HasPrefix(out, "alice:$apr1$") {
		t.Errorf("应输出 apr1，got %q", out)
	}
}

func TestHtpasswd_SHA(t *testing.T) {
	out, err := runHtpasswd(t, strings.NewReader("hunter2\n"), "alice", "--sha")
	if err != nil {
		t.Fatal(err)
	}
	if !strings.HasPrefix(out, "alice:{SHA}") {
		t.Errorf("应输出 {SHA}，got %q", out)
	}
}

func TestHtpasswd_VerifyMatch(t *testing.T) {
	// {SHA} of "password"（openssl 金标准）
	out, err := runHtpasswd(t, strings.NewReader("password\n"),
		"--verify", "{SHA}W6ph5Mm5Pz8GgiULbPgzG37mj9g=")
	if err != nil {
		t.Fatalf("正确密码不该报错：%v", err)
	}
	if !strings.Contains(out, "匹配") {
		t.Errorf("应提示匹配，got %q", out)
	}
}

func TestHtpasswd_VerifyMismatch(t *testing.T) {
	_, err := runHtpasswd(t, strings.NewReader("wrongpw\n"),
		"--verify", "{SHA}W6ph5Mm5Pz8GgiULbPgzG37mj9g=")
	if err == nil {
		t.Error("错误密码应返回非 nil error（脚本可凭退出码判断）")
	}
}

func TestHtpasswd_FileUpsert(t *testing.T) {
	f := filepath.Join(t.TempDir(), ".htpasswd")
	if _, err := runHtpasswd(t, strings.NewReader("p1\n"), "alice", "-f", f); err != nil {
		t.Fatal(err)
	}
	if _, err := runHtpasswd(t, strings.NewReader("p2\n"), "bob", "-f", f); err != nil {
		t.Fatal(err)
	}
	if _, err := runHtpasswd(t, strings.NewReader("p3\n"), "alice", "-f", f); err != nil {
		t.Fatal(err)
	}
	b, err := os.ReadFile(f)
	if err != nil {
		t.Fatal(err)
	}
	content := string(b)
	if strings.Count(content, "alice:") != 1 {
		t.Errorf("alice 应只有一行（被替换非重复）:\n%s", content)
	}
	if !strings.Contains(content, "bob:") {
		t.Errorf("bob 应保留:\n%s", content)
	}
}

func TestHtpasswd_ConflictFlags(t *testing.T) {
	_, err := runHtpasswd(t, strings.NewReader("x\n"), "alice", "--apr1", "--sha")
	if err == nil || !strings.Contains(err.Error(), "只能选一个") {
		t.Errorf("--apr1 + --sha 应报错，got %v", err)
	}
}

func TestHtpasswd_NoUser(t *testing.T) {
	_, err := runHtpasswd(t, strings.NewReader("x\n"))
	if err == nil {
		t.Error("无用户名又非校验模式应报错")
	}
}
