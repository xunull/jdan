package cli

import (
	"bytes"
	"encoding/json"
	"strings"
	"testing"

	"github.com/xunull/jdan/internal/gitx"
)

// fakeRunner 按 git 子命令返回固定输出，CLI 测试不依赖真实 git。
func fakeRunner(isRepo bool) gitx.Runner {
	return func(dir string, args ...string) (string, error) {
		switch strings.Join(args, " ") {
		case "rev-parse --is-inside-work-tree":
			if isRepo {
				return "true\n", nil
			}
			return "false\n", nil
		case "rev-parse --show-toplevel":
			return "/tmp/myrepo\n", nil
		case "rev-list --count HEAD":
			return "10\n", nil
		case "for-each-ref --format=%(refname) refs/heads":
			return "refs/heads/main\nrefs/heads/dev\n", nil
		case "tag":
			return "v1.0.0\nv1.1.0\n", nil
		case "log --reverse --format=%cI --max-parents=0":
			return "2026-01-01T10:00:00Z\n", nil
		case "log -1 --format=%cI":
			return "2026-04-01T10:00:00Z\n", nil
		case "log --format=%an":
			return "Bob\nBob\nBob\nBob\nBob\nBob\nBob\nAmy\nAmy\nCarol\n", nil
		case "log --pretty=format: --name-only":
			return "a.go\n\na.go\nb.go\na.go\nREADME.md\n", nil
		}
		return "", nil
	}
}

func runGitSummary(t *testing.T, run gitx.Runner, args ...string) (string, error) {
	t.Helper()
	var buf bytes.Buffer
	cmd := newGitSummaryCommand(gitSummaryDeps{out: &buf, run: run})
	cmd.SetArgs(args)
	err := cmd.Execute()
	return buf.String(), err
}

func TestGitSummaryCmd_Text(t *testing.T) {
	out, err := runGitSummary(t, fakeRunner(true))
	if err != nil {
		t.Fatal(err)
	}
	for _, want := range []string{"仓库: myrepo", "commit: 10", "分支: 2", "tag: 2", "Bob", "a.go"} {
		if !strings.Contains(out, want) {
			t.Errorf("output missing %q:\n%s", want, out)
		}
	}
}

func TestGitSummaryCmd_JSON(t *testing.T) {
	out, err := runGitSummary(t, fakeRunner(true), "--json")
	if err != nil {
		t.Fatal(err)
	}
	var s gitx.Summary
	if err := json.Unmarshal([]byte(out), &s); err != nil {
		t.Fatalf("invalid JSON: %v\n%s", err, out)
	}
	if s.Commits != 10 || s.Tags != 2 || s.Branches != 2 {
		t.Errorf("bad summary: %+v", s)
	}
	if s.Repo != "myrepo" {
		t.Errorf("repo = %q, want myrepo", s.Repo)
	}
	// 贡献者降序，Bob 最高
	if len(s.Contributors) == 0 || s.Contributors[0].Name != "Bob" {
		t.Errorf("top contributor wrong: %+v", s.Contributors)
	}
}

func TestGitSummaryCmd_Top(t *testing.T) {
	out, err := runGitSummary(t, fakeRunner(true), "--top", "1", "--json")
	if err != nil {
		t.Fatal(err)
	}
	var s gitx.Summary
	if err := json.Unmarshal([]byte(out), &s); err != nil {
		t.Fatal(err)
	}
	if len(s.Contributors) != 1 {
		t.Errorf("--top 1 → 1 contributor, got %d", len(s.Contributors))
	}
	if len(s.Hotspots) != 1 {
		t.Errorf("--top 1 → 1 hotspot, got %d", len(s.Hotspots))
	}
}

func TestGitSummaryCmd_NotARepo(t *testing.T) {
	_, err := runGitSummary(t, fakeRunner(false))
	if err == nil {
		t.Error("non-repo should error")
	}
}

func TestGitCmd_HasSummarySubcommand(t *testing.T) {
	g := newGitCommand()
	var found bool
	for _, c := range g.Commands() {
		if c.Name() == "summary" {
			found = true
		}
	}
	if !found {
		t.Error("git command should have summary subcommand")
	}
}
