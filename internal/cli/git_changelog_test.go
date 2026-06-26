package cli

import (
	"bytes"
	"encoding/json"
	"fmt"
	"strings"
	"testing"

	"github.com/xunull/jdan/internal/gitx"
)

// fakeChangelogRunner 按 git 子命令返回固定输出，CLI 测试不依赖真实 git。
func fakeChangelogRunner(isRepo bool, tag, logOut string) gitx.Runner {
	return func(dir string, args ...string) (string, error) {
		switch args[0] {
		case "rev-parse":
			if isRepo {
				return "true\n", nil
			}
			return "false\n", nil
		case "describe":
			if tag == "" {
				return "", fmt.Errorf("no tags")
			}
			return tag + "\n", nil
		case "log":
			return logOut, nil
		}
		return "", fmt.Errorf("unexpected git %v", args)
	}
}

func runGitChangelog(t *testing.T, run gitx.Runner, args ...string) (string, error) {
	t.Helper()
	var buf bytes.Buffer
	cmd := newGitChangelogCommand(gitChangelogDeps{out: &buf, run: run})
	cmd.SetArgs(args)
	err := cmd.Execute()
	return buf.String(), err
}

func clRec(h, s, b string) string { return h + "\x1f" + s + "\x1f" + b + "\x1e" }

func TestGitChangelogCmd_Text(t *testing.T) {
	logOut := clRec("h1", "feat(json): add merge", "") + "\n" + clRec("h2", "fix: a bug", "") + "\n"
	out, err := runGitChangelog(t, fakeChangelogRunner(true, "v0.1.0", logOut))
	if err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(out, "### Features") || !strings.Contains(out, "- (json) add merge") {
		t.Errorf("text output wrong:\n%s", out)
	}
	if !strings.Contains(out, "### Bug Fixes") {
		t.Errorf("missing Bug Fixes:\n%s", out)
	}
}

func TestGitChangelogCmd_JSON(t *testing.T) {
	logOut := clRec("h1", "feat: x", "") + "\n"
	out, err := runGitChangelog(t, fakeChangelogRunner(true, "v0.1.0", logOut), "--json")
	if err != nil {
		t.Fatal(err)
	}
	var v map[string]any
	if err := json.Unmarshal([]byte(out), &v); err != nil {
		t.Fatalf("--json should emit valid JSON: %v\n%s", err, out)
	}
}

func TestGitChangelogCmd_FromTo(t *testing.T) {
	logOut := clRec("h1", "feat: x", "") + "\n"
	out, err := runGitChangelog(t, fakeChangelogRunner(true, "v0.1.0", logOut), "--from", "v0.1.0", "--to", "v0.2.0")
	if err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(out, "## v0.2.0 (自 v0.1.0)") {
		t.Errorf("--from/--to heading wrong:\n%s", out)
	}
}

func TestGitChangelogCmd_NotRepo(t *testing.T) {
	if _, err := runGitChangelog(t, fakeChangelogRunner(false, "", "")); err == nil {
		t.Error("non-repo should error")
	}
}
