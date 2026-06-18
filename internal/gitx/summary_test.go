package gitx

import (
	"os"
	"os/exec"
	"path/filepath"
	"testing"
	"time"
)

// ---- 临时真仓库 helper ----

func runGit(t *testing.T, dir string, env []string, args ...string) {
	t.Helper()
	cmd := exec.Command("git", args...)
	cmd.Dir = dir
	cmd.Env = append(os.Environ(), env...)
	if out, err := cmd.CombinedOutput(); err != nil {
		t.Fatalf("git %v: %v\n%s", args, err, out)
	}
}

func commit(t *testing.T, dir, name, email, date, file, content string) {
	t.Helper()
	if err := os.WriteFile(filepath.Join(dir, file), []byte(content), 0o644); err != nil {
		t.Fatal(err)
	}
	runGit(t, dir, nil, "add", ".")
	env := []string{
		"GIT_AUTHOR_DATE=" + date, "GIT_COMMITTER_DATE=" + date,
		"GIT_AUTHOR_NAME=" + name, "GIT_AUTHOR_EMAIL=" + email,
		"GIT_COMMITTER_NAME=" + name, "GIT_COMMITTER_EMAIL=" + email,
	}
	runGit(t, dir, env, "commit", "-q", "-m", "change "+file)
}

// makeRepo 建一个有 4 个 commit（Bob×3 / Amy×1）、1 个 tag、fileA 改 3 次的仓库。
func makeRepo(t *testing.T) string {
	t.Helper()
	if _, err := exec.LookPath("git"); err != nil {
		t.Skip("git 不在 PATH，跳过")
	}
	dir := t.TempDir()
	runGit(t, dir, nil, "init", "-q")
	runGit(t, dir, nil, "config", "user.name", "Test")
	runGit(t, dir, nil, "config", "user.email", "test@example.com")
	runGit(t, dir, nil, "config", "commit.gpgsign", "false")

	commit(t, dir, "Bob", "bob@x.com", "2026-01-01T10:00:00", "fileA", "1")
	commit(t, dir, "Bob", "bob@x.com", "2026-02-01T10:00:00", "fileA", "2")
	commit(t, dir, "Amy", "amy@x.com", "2026-03-01T10:00:00", "fileB", "1")
	runGit(t, dir, nil, "tag", "v0.1.0")
	commit(t, dir, "Bob", "bob@x.com", "2026-04-01T10:00:00", "fileA", "3")
	return dir
}

// ---- Summarize ----

func TestSummarize(t *testing.T) {
	dir := makeRepo(t)
	s, err := Summarize(ExecRunner, dir, 5)
	if err != nil {
		t.Fatal(err)
	}
	if s.Commits != 4 {
		t.Errorf("commits = %d, want 4", s.Commits)
	}
	if s.Tags != 1 {
		t.Errorf("tags = %d, want 1", s.Tags)
	}
	if s.Branches != 1 {
		t.Errorf("branches = %d, want 1", s.Branches)
	}
	if s.Repo == "" {
		t.Error("repo name should be set")
	}
}

func TestSummarize_Contributors(t *testing.T) {
	dir := makeRepo(t)
	s, _ := Summarize(ExecRunner, dir, 5)
	if len(s.Contributors) != 2 {
		t.Fatalf("want 2 contributors, got %d: %+v", len(s.Contributors), s.Contributors)
	}
	// 降序：Bob(3) 在前
	if s.Contributors[0].Name != "Bob" || s.Contributors[0].Commits != 3 {
		t.Errorf("top contributor = %+v, want Bob/3", s.Contributors[0])
	}
	if s.Contributors[1].Name != "Amy" || s.Contributors[1].Commits != 1 {
		t.Errorf("2nd contributor = %+v, want Amy/1", s.Contributors[1])
	}
	// 百分比：Bob 3/4 = 75%
	if got := s.Contributors[0].Percent; got < 74.9 || got > 75.1 {
		t.Errorf("Bob percent = %.1f, want 75", got)
	}
}

func TestSummarize_Hotspots(t *testing.T) {
	dir := makeRepo(t)
	s, _ := Summarize(ExecRunner, dir, 5)
	if len(s.Hotspots) == 0 {
		t.Fatal("expected hotspots")
	}
	// fileA 改了 3 次，应当排第一
	if s.Hotspots[0].Path != "fileA" || s.Hotspots[0].Changes != 3 {
		t.Errorf("top hotspot = %+v, want fileA/3", s.Hotspots[0])
	}
}

func TestSummarize_Age(t *testing.T) {
	dir := makeRepo(t)
	s, _ := Summarize(ExecRunner, dir, 5)
	if s.FirstCommit != "2026-01-01" {
		t.Errorf("first = %q, want 2026-01-01", s.FirstCommit)
	}
	if s.LastCommit != "2026-04-01" {
		t.Errorf("last = %q, want 2026-04-01", s.LastCommit)
	}
	if s.Age != "约 3 个月" {
		t.Errorf("age = %q, want 约 3 个月", s.Age)
	}
}

func TestSummarize_TopLimit(t *testing.T) {
	dir := makeRepo(t)
	s, _ := Summarize(ExecRunner, dir, 1)
	if len(s.Contributors) != 1 {
		t.Errorf("--top 1 should give 1 contributor, got %d", len(s.Contributors))
	}
	if len(s.Hotspots) != 1 {
		t.Errorf("--top 1 should give 1 hotspot, got %d", len(s.Hotspots))
	}
}

func TestSummarize_NotARepo(t *testing.T) {
	if _, err := exec.LookPath("git"); err != nil {
		t.Skip("git 不在 PATH，跳过")
	}
	dir := t.TempDir() // 没 git init
	if _, err := Summarize(ExecRunner, dir, 5); err == nil {
		t.Error("non-repo should error")
	}
}

func TestSummarize_EmptyRepo(t *testing.T) {
	if _, err := exec.LookPath("git"); err != nil {
		t.Skip("git 不在 PATH，跳过")
	}
	dir := t.TempDir()
	runGit(t, dir, nil, "init", "-q")
	if _, err := Summarize(ExecRunner, dir, 5); err == nil {
		t.Error("empty repo (0 commit) should error")
	}
}

// ---- humanizeSpan ----

func TestHumanizeSpan(t *testing.T) {
	base := time.Date(2026, 1, 1, 0, 0, 0, 0, time.UTC)
	cases := []struct {
		days int
		want string
	}{
		{0, "今天"},
		{5, "约 5 天"},
		{60, "约 2 个月"},
		{365, "约 1 年"},
		{400, "约 1 年 1 个月"},
	}
	for _, c := range cases {
		got := humanizeSpan(base, base.AddDate(0, 0, c.days))
		if got != c.want {
			t.Errorf("humanizeSpan(+%dd) = %q, want %q", c.days, got, c.want)
		}
	}
}

func TestHumanizeSpan_OrderIndependent(t *testing.T) {
	a := time.Date(2026, 1, 1, 0, 0, 0, 0, time.UTC)
	b := a.AddDate(0, 0, 60)
	if humanizeSpan(a, b) != humanizeSpan(b, a) {
		t.Error("span should be symmetric")
	}
}

// ---- IsRepo ----

func TestIsRepo(t *testing.T) {
	dir := makeRepo(t)
	if !IsRepo(ExecRunner, dir) {
		t.Error("makeRepo should be a repo")
	}
	if _, err := exec.LookPath("git"); err == nil {
		if IsRepo(ExecRunner, t.TempDir()) {
			t.Error("empty temp dir should not be a repo")
		}
	}
}
