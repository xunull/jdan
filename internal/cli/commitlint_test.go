package cli

import (
	"bytes"
	"io"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func runCommitlint(t *testing.T, deps commitlintDeps, args ...string) (string, error) {
	t.Helper()
	var out bytes.Buffer
	deps.out = &out
	if deps.in == nil {
		deps.in = strings.NewReader("") // 默认空 stdin（非 nil，避免读到真 os.Stdin）
	}
	cmd := newCommitlintCommand(deps)
	cmd.SetArgs(args)
	cmd.SetOut(io.Discard)
	cmd.SetErr(io.Discard)
	err := cmd.Execute() // 先执行再读 buffer（求值顺序坑）
	return out.String(), err
}

func TestCommitlint_LiteralValid(t *testing.T) {
	out, err := runCommitlint(t, commitlintDeps{}, "-m", "feat(api): add pagination")
	if err != nil {
		t.Fatalf("合规信息不该报错：%v", err)
	}
	if !strings.Contains(out, "✓") {
		t.Errorf("应标记合规，got %q", out)
	}
}

func TestCommitlint_LiteralInvalid_ExitErr(t *testing.T) {
	out, err := runCommitlint(t, commitlintDeps{}, "-m", "updated stuff")
	if err == nil {
		t.Error("不合规信息应返回非 nil error（退出码非 0）")
	}
	if !strings.Contains(out, "header-structure") {
		t.Errorf("应报 header-structure，got %q", out)
	}
}

func TestCommitlint_WarnOnly_ExitsZero(t *testing.T) {
	_, err := runCommitlint(t, commitlintDeps{}, "-m", "updated stuff", "--warn")
	if err != nil {
		t.Errorf("--warn 即使不合规也应退出 0，got %v", err)
	}
}

func TestCommitlint_FileMode(t *testing.T) {
	f := filepath.Join(t.TempDir(), "COMMIT_EDITMSG")
	if err := os.WriteFile(f, []byte("fix: 修个空指针\n# comment\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	out, err := runCommitlint(t, commitlintDeps{}, "-f", f)
	if err != nil {
		t.Fatalf("合规文件不该报错：%v", err)
	}
	if !strings.Contains(out, "✓") {
		t.Errorf("应合规，got %q", out)
	}
}

func TestCommitlint_Stdin(t *testing.T) {
	deps := commitlintDeps{in: strings.NewReader("chore: bump deps")}
	out, err := runCommitlint(t, deps)
	if err != nil {
		t.Fatalf("stdin 合规不该报错：%v", err)
	}
	if !strings.Contains(out, "✓") {
		t.Errorf("应合规，got %q", out)
	}
}

func TestCommitlint_RangeMode_MockGit(t *testing.T) {
	// 注入假 git：返回两条提交（hash<US>body<RS>），一条合规一条不合规
	fakeRun := func(_ string, args ...string) (string, error) {
		rec := "a1b2c3\x1ffeat: good one\x1e" + "d4e5f6\x1fbad commit msg\x1e"
		return rec, nil
	}
	out, err := runCommitlint(t, commitlintDeps{run: fakeRun}, "HEAD~2..HEAD")
	if err == nil {
		t.Error("range 里有不合规提交应返回 error")
	}
	if !strings.Contains(out, "a1b2c3") || !strings.Contains(out, "d4e5f6") {
		t.Errorf("应按短 hash 标注每条提交，got %q", out)
	}
	if !strings.Contains(out, "1/2 条提交不合规") {
		t.Errorf("应汇总 1/2 不合规，got %q", out)
	}
}

func TestCommitlint_JSON(t *testing.T) {
	out, err := runCommitlint(t, commitlintDeps{}, "-m", "feat: ok", "--json")
	if err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(out, `"ok": true`) {
		t.Errorf("JSON 应含 ok:true，got %q", out)
	}
}

func TestCommitlint_CustomTypes(t *testing.T) {
	_, err := runCommitlint(t, commitlintDeps{}, "-m", "feat: x", "--types", "task,bug")
	if err == nil {
		t.Error("自定义白名单不含 feat，应判不合规")
	}
}
