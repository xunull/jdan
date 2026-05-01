package tree2

import (
	"os"
	"path/filepath"
	"runtime"
	"strings"
	"testing"
)

func TestBuildFiltersFilesByDefault(t *testing.T) {
	root := t.TempDir()
	mkdir(t, root, "docs")
	writeFile(t, root, "README.md")

	nodes, err := Build(Options{RootPath: root})
	if err != nil {
		t.Fatal(err)
	}

	if got := nodeNames(nodes); strings.Join(got, ",") != "docs" {
		t.Fatalf("nodes = %v, want docs", got)
	}
}

func TestBuildIncludesFilesWhenRequested(t *testing.T) {
	root := t.TempDir()
	mkdir(t, root, "docs")
	writeFile(t, root, "README.md")

	nodes, err := Build(Options{RootPath: root, IncludeFiles: true})
	if err != nil {
		t.Fatal(err)
	}

	if got := nodeNames(nodes); strings.Join(got, ",") != "docs,README.md" {
		t.Fatalf("nodes = %v, want docs,README.md", got)
	}
}

func TestBuildFiltersHiddenByDefault(t *testing.T) {
	root := t.TempDir()
	mkdir(t, root, ".git")
	mkdir(t, root, "internal")

	nodes, err := Build(Options{RootPath: root})
	if err != nil {
		t.Fatal(err)
	}

	if got := nodeNames(nodes); strings.Join(got, ",") != "internal" {
		t.Fatalf("nodes = %v, want internal", got)
	}
}

func TestBuildIncludesHiddenWhenRequested(t *testing.T) {
	root := t.TempDir()
	mkdir(t, root, ".git")
	mkdir(t, root, "internal")

	nodes, err := Build(Options{RootPath: root, IncludeHidden: true})
	if err != nil {
		t.Fatal(err)
	}

	if got := nodeNames(nodes); strings.Join(got, ",") != ".git,internal" {
		t.Fatalf("nodes = %v, want .git,internal", got)
	}
}

func TestBuildReturnsRootErrors(t *testing.T) {
	_, err := Build(Options{RootPath: filepath.Join(t.TempDir(), "missing")})
	if err == nil {
		t.Fatal("expected missing root error")
	}

	file := filepath.Join(t.TempDir(), "file.txt")
	if err := os.WriteFile(file, []byte("x"), 0o644); err != nil {
		t.Fatal(err)
	}
	_, err = Build(Options{RootPath: file})
	if err == nil || !strings.Contains(err.Error(), "不是目录") {
		t.Fatalf("expected not directory error, got %v", err)
	}
}

func TestBuildKeepsChildReadErrorOnNode(t *testing.T) {
	if runtime.GOOS == "windows" {
		t.Skip("permission bits behave differently on Windows")
	}

	root := t.TempDir()
	locked := mkdir(t, root, "locked")
	if err := os.Chmod(locked, 0); err != nil {
		t.Fatal(err)
	}
	defer os.Chmod(locked, 0o755)

	nodes, err := Build(Options{RootPath: root})
	if err != nil {
		t.Fatal(err)
	}
	if len(nodes) != 1 {
		t.Fatalf("nodes len = %d, want 1", len(nodes))
	}
	if nodes[0].Err == nil {
		t.Fatal("expected child read error on node")
	}
}

func TestBuildLimitsChildrenByDefaultAndCanDisableLimit(t *testing.T) {
	root := t.TempDir()
	parent := mkdir(t, root, "parent")
	for i := 0; i < DefaultLimit+3; i++ {
		mkdir(t, parent, string(rune('a'+i)))
	}

	nodes, err := Build(Options{RootPath: root, Limit: DefaultLimit})
	if err != nil {
		t.Fatal(err)
	}
	if got := len(nodes[0].Children); got != DefaultLimit {
		t.Fatalf("children len = %d, want %d", got, DefaultLimit)
	}
	if nodes[0].MoreCount != 3 {
		t.Fatalf("more = %d, want 3", nodes[0].MoreCount)
	}

	nodes, err = Build(Options{RootPath: root, Limit: 0})
	if err != nil {
		t.Fatal(err)
	}
	if got := len(nodes[0].Children); got != DefaultLimit+3 {
		t.Fatalf("children len = %d, want %d", got, DefaultLimit+3)
	}
}

func TestBuildDoesNotRecurseIntoSymlinkDirectory(t *testing.T) {
	if runtime.GOOS == "windows" {
		t.Skip("symlink creation requires extra privileges on Windows")
	}

	root := t.TempDir()
	realDir := mkdir(t, root, "real")
	mkdir(t, realDir, "child")
	if err := os.Symlink(realDir, filepath.Join(root, "link")); err != nil {
		t.Fatal(err)
	}

	nodes, err := Build(Options{RootPath: root, IncludeFiles: true})
	if err != nil {
		t.Fatal(err)
	}

	for _, node := range nodes {
		if node.Name == "link" && node.IsDir {
			t.Fatal("symlink should not be treated as directory")
		}
		if node.Name == "link" && len(node.Children) > 0 {
			t.Fatal("symlink should not have children")
		}
	}
}

func nodeNames(nodes []Node) []string {
	names := make([]string, 0, len(nodes))
	for _, node := range nodes {
		names = append(names, node.Name)
	}
	return names
}

func mkdir(t *testing.T, root, name string) string {
	t.Helper()
	path := filepath.Join(root, name)
	if err := os.MkdirAll(path, 0o755); err != nil {
		t.Fatal(err)
	}
	return path
}

func writeFile(t *testing.T, root, name string) {
	t.Helper()
	if err := os.WriteFile(filepath.Join(root, name), []byte("x"), 0o644); err != nil {
		t.Fatal(err)
	}
}
