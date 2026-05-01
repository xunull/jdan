package tree2

import (
	"fmt"
	"os"
	"path/filepath"
	"sort"
	"strings"
)

const (
	DefaultWidth = 80
	DefaultLimit = 50
)

type Options struct {
	RootPath      string
	Width         int
	Columns       int
	IncludeFiles  bool
	IncludeHidden bool
	Limit         int
}

type Node struct {
	Name      string
	Path      string
	IsDir     bool
	Children  []Node
	MoreCount int
	Err       error
}

func Build(opts Options) ([]Node, error) {
	root := opts.RootPath
	if root == "" {
		root = "."
	}

	info, err := os.Stat(root)
	if err != nil {
		return nil, fmt.Errorf("读取路径失败: %w", err)
	}
	if !info.IsDir() {
		return nil, fmt.Errorf("%s 不是目录", root)
	}

	entries, err := os.ReadDir(root)
	if err != nil {
		return nil, fmt.Errorf("读取目录失败: %w", err)
	}

	nodes := make([]Node, 0, len(entries))
	for _, entry := range filterEntries(entries, opts) {
		fullPath := filepath.Join(root, entry.Name())
		node := Node{
			Name:  entry.Name(),
			Path:  fullPath,
			IsDir: entry.IsDir(),
		}
		if entry.IsDir() {
			children, more, childErr := readChildren(fullPath, opts)
			node.Children = children
			node.MoreCount = more
			node.Err = childErr
		}
		nodes = append(nodes, node)
	}

	sortNodes(nodes)
	return nodes, nil
}

func readChildren(path string, opts Options) ([]Node, int, error) {
	entries, err := os.ReadDir(path)
	if err != nil {
		return nil, 0, err
	}

	filtered := filterEntries(entries, opts)
	limit := normalizedLimit(opts.Limit)
	more := 0
	if limit > 0 && len(filtered) > limit {
		more = len(filtered) - limit
		filtered = filtered[:limit]
	}

	children := make([]Node, 0, len(filtered))
	for _, entry := range filtered {
		children = append(children, Node{
			Name:  entry.Name(),
			Path:  filepath.Join(path, entry.Name()),
			IsDir: entry.IsDir(),
		})
	}
	sortNodes(children)
	return children, more, nil
}

func filterEntries(entries []os.DirEntry, opts Options) []os.DirEntry {
	filtered := make([]os.DirEntry, 0, len(entries))
	for _, entry := range entries {
		name := entry.Name()
		if !opts.IncludeHidden && strings.HasPrefix(name, ".") {
			continue
		}
		if !opts.IncludeFiles && !entry.IsDir() {
			continue
		}
		filtered = append(filtered, entry)
	}
	sort.Slice(filtered, func(i, j int) bool {
		if filtered[i].IsDir() != filtered[j].IsDir() {
			return filtered[i].IsDir()
		}
		return filtered[i].Name() < filtered[j].Name()
	})
	return filtered
}

func sortNodes(nodes []Node) {
	sort.Slice(nodes, func(i, j int) bool {
		if nodes[i].IsDir != nodes[j].IsDir {
			return nodes[i].IsDir
		}
		return nodes[i].Name < nodes[j].Name
	})
}

func normalizedLimit(limit int) int {
	if limit < 0 {
		return DefaultLimit
	}
	return limit
}
