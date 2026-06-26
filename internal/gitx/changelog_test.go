package gitx

import (
	"encoding/json"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"testing"
)

// ---- parseCommit ----

func TestParseCommit_FeatWithScope(t *testing.T) {
	c := parseCommit("h1", "feat(json): add merge", "")
	if c.Type != "feat" || c.Scope != "json" || c.Subject != "add merge" || c.Breaking {
		t.Errorf("%+v", c)
	}
}

func TestParseCommit_NoScope(t *testing.T) {
	c := parseCommit("h", "fix: nil deref", "")
	if c.Type != "fix" || c.Scope != "" || c.Subject != "nil deref" {
		t.Errorf("%+v", c)
	}
}

func TestParseCommit_BangBreaking(t *testing.T) {
	c := parseCommit("h", "feat(api)!: drop v1", "")
	if !c.Breaking || c.Type != "feat" || c.Scope != "api" || c.Subject != "drop v1" {
		t.Errorf("%+v", c)
	}
}

func TestParseCommit_BodyBreaking(t *testing.T) {
	c := parseCommit("h", "refactor: rework", "BREAKING CHANGE: config moved")
	if !c.Breaking {
		t.Errorf("body BREAKING CHANGE should set breaking: %+v", c)
	}
}

func TestParseCommit_BreakingHyphen(t *testing.T) {
	if c := parseCommit("h", "fix: x", "BREAKING-CHANGE: y"); !c.Breaking {
		t.Error("BREAKING-CHANGE (hyphen) should count")
	}
}

func TestParseCommit_NonConforming(t *testing.T) {
	c := parseCommit("h", "just a random message", "")
	if c.Type != "" || c.Subject != "just a random message" {
		t.Errorf("non-conforming should keep full subject, empty type: %+v", c)
	}
}

func TestParseCommit_TypeLowercased(t *testing.T) {
	if c := parseCommit("h", "FEAT: x", ""); c.Type != "feat" {
		t.Errorf("type should be lowercased: %+v", c)
	}
}

// ---- BuildChangelog (fake runner) ----

func fakeChangelogRunner(isRepo bool, tag, logOut string) Runner {
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

func rec(h, s, b string) string { return h + "\x1f" + s + "\x1f" + b + "\x1e" }

func TestBuildChangelog_RangeAndParse(t *testing.T) {
	logOut := rec("h1", "feat(json): add merge", "") + "\n" + rec("h2", "fix(cli): bug", "") + "\n"
	cl, err := BuildChangelog(fakeChangelogRunner(true, "v0.1.0", logOut), ".", "", "")
	if err != nil {
		t.Fatal(err)
	}
	if cl.From != "v0.1.0" {
		t.Errorf("from should default to latest tag: %q", cl.From)
	}
	if cl.To != "HEAD" {
		t.Errorf("to should default to HEAD: %q", cl.To)
	}
	if len(cl.Commits) != 2 {
		t.Fatalf("want 2 commits, got %d: %+v", len(cl.Commits), cl.Commits)
	}
	if cl.Commits[0].Type != "feat" || cl.Commits[1].Type != "fix" {
		t.Errorf("parse wrong: %+v", cl.Commits)
	}
}

func TestBuildChangelog_NotRepo(t *testing.T) {
	if _, err := BuildChangelog(fakeChangelogRunner(false, "", ""), ".", "", ""); err == nil {
		t.Error("non-repo should error")
	}
}

func TestBuildChangelog_NoTagWholeHistory(t *testing.T) {
	logOut := rec("h1", "feat: first", "") + "\n"
	cl, err := BuildChangelog(fakeChangelogRunner(true, "", logOut), ".", "", "")
	if err != nil {
		t.Fatal(err)
	}
	if cl.From != "" {
		t.Errorf("no tag → from should be empty (whole history): %q", cl.From)
	}
}

// ---- Markdown ----

func TestMarkdown_Grouping(t *testing.T) {
	cl := Changelog{From: "v0.1.0", To: "HEAD", Commits: []Commit{
		{Hash: "1", Type: "feat", Scope: "json", Subject: "add merge"},
		{Hash: "2", Type: "fix", Subject: "a bug"},
		{Hash: "3", Type: "feat", Scope: "api", Subject: "drop v1", Breaking: true},
		{Hash: "4", Type: "chore", Subject: "bump deps"},
		{Hash: "5", Subject: "random non-conforming"},
	}}
	md := cl.Markdown()
	if !strings.Contains(md, "## 未发布 (自 v0.1.0)") {
		t.Errorf("header wrong: %s", md)
	}
	if !strings.Contains(md, "### ⚠ Breaking Changes") {
		t.Error("missing breaking section")
	}
	if strings.Count(md, "drop v1") != 1 {
		t.Error("breaking commit should appear exactly once (not duplicated into Features)")
	}
	if !strings.Contains(md, "- (json) add merge") {
		t.Error("scope rendering wrong")
	}
	if !strings.Contains(md, "### Bug Fixes") {
		t.Error("missing Bug Fixes")
	}
	if !strings.Contains(md, "### Other") || !strings.Contains(md, "bump deps") || !strings.Contains(md, "random non-conforming") {
		t.Error("chore + non-conforming should land in Other")
	}
}

func TestMarkdown_SectionOrder(t *testing.T) {
	cl := Changelog{From: "v1", To: "HEAD", Commits: []Commit{
		{Type: "fix", Subject: "f"}, {Type: "feat", Subject: "x"},
	}}
	md := cl.Markdown()
	if strings.Index(md, "### Features") > strings.Index(md, "### Bug Fixes") {
		t.Error("Features should come before Bug Fixes")
	}
}

func TestMarkdown_Empty(t *testing.T) {
	if md := (Changelog{From: "v1", To: "HEAD"}).Markdown(); !strings.Contains(md, "无提交") {
		t.Errorf("empty changelog should note no commits: %s", md)
	}
}

func TestMarkdown_TaggedHeading(t *testing.T) {
	cl := Changelog{From: "v0.1.0", To: "v0.2.0", Commits: []Commit{{Type: "feat", Subject: "x"}}}
	if md := cl.Markdown(); !strings.Contains(md, "## v0.2.0 (自 v0.1.0)") {
		t.Errorf("tagged heading wrong: %s", md)
	}
}

func TestMarkdown_WholeHistory(t *testing.T) {
	cl := Changelog{From: "", To: "HEAD", Commits: []Commit{{Type: "feat", Subject: "x"}}}
	if md := cl.Markdown(); !strings.Contains(md, "全部历史") {
		t.Errorf("no-tag heading should say whole history: %s", md)
	}
}

// ---- JSON ----

func TestChangelogJSON(t *testing.T) {
	cl := Changelog{From: "v1", To: "HEAD", Commits: []Commit{
		{Hash: "h", Type: "feat", Scope: "x", Subject: "s", Breaking: true},
	}}
	s, err := cl.JSON()
	if err != nil {
		t.Fatal(err)
	}
	var parsed struct {
		From    string           `json:"from"`
		To      string           `json:"to"`
		Commits []map[string]any `json:"commits"`
	}
	if err := json.Unmarshal([]byte(s), &parsed); err != nil {
		t.Fatal(err)
	}
	if parsed.From != "v1" || len(parsed.Commits) != 1 || parsed.Commits[0]["breaking"] != true {
		t.Errorf("%+v", parsed)
	}
}

func TestChangelogJSON_EmptyCommitsArray(t *testing.T) {
	if s, _ := (Changelog{From: "v1", To: "HEAD"}).JSON(); !strings.Contains(s, `"commits": []`) {
		t.Errorf("empty commits should be [] not null: %s", s)
	}
}

// ---- 集成（临时真仓库）----

func ccommit(t *testing.T, dir, msg string) {
	t.Helper()
	f := filepath.Join(dir, "log.txt")
	prev, _ := os.ReadFile(f)
	if err := os.WriteFile(f, append(prev, []byte(msg+"\n")...), 0o644); err != nil {
		t.Fatal(err)
	}
	runGit(t, dir, nil, "add", ".")
	env := []string{
		"GIT_AUTHOR_NAME=T", "GIT_AUTHOR_EMAIL=t@x.com",
		"GIT_COMMITTER_NAME=T", "GIT_COMMITTER_EMAIL=t@x.com",
	}
	runGit(t, dir, env, "commit", "-q", "-m", msg)
}

func TestBuildChangelog_Integration(t *testing.T) {
	if _, err := exec.LookPath("git"); err != nil {
		t.Skip("git 不在 PATH，跳过")
	}
	dir := t.TempDir()
	runGit(t, dir, nil, "init", "-q")
	runGit(t, dir, nil, "config", "user.name", "T")
	runGit(t, dir, nil, "config", "user.email", "t@x.com")
	runGit(t, dir, nil, "config", "commit.gpgsign", "false")

	ccommit(t, dir, "feat(core): initial") // 在 tag 之前，不应出现在范围里
	runGit(t, dir, nil, "tag", "v0.1.0")
	ccommit(t, dir, "feat(json): add merge")
	ccommit(t, dir, "fix(cli): nil deref")

	cl, err := BuildChangelog(ExecRunner, dir, "", "")
	if err != nil {
		t.Fatal(err)
	}
	if cl.From != "v0.1.0" {
		t.Errorf("from should resolve to latest tag, got %q", cl.From)
	}
	if len(cl.Commits) != 2 {
		t.Fatalf("only post-tag commits should be in range, got %d: %+v", len(cl.Commits), cl.Commits)
	}
	md := cl.Markdown()
	if !strings.Contains(md, "add merge") || !strings.Contains(md, "nil deref") {
		t.Errorf("md missing expected commits: %s", md)
	}
	if strings.Contains(md, "initial") {
		t.Error("pre-tag commit should be excluded from range")
	}
}
