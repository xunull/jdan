package jsonx

import (
	"fmt"
	"sort"
	"strings"
)

// Keys 列出 v 的 key。
//   - allPaths=false: 只列顶层 key（v 必须是 object）
//   - allPaths=true:  递归列所有叶子路径（dot-path 风格，含 [N] 表数组）
//   - maxDepth>0:     只在 allPaths=true 下生效，限制最大递归深度
func Keys(v any, allPaths bool, maxDepth int) ([]string, error) {
	if !allPaths {
		m, ok := v.(map[string]any)
		if !ok {
			return nil, fmt.Errorf("top-level is not an object (%T)", v)
		}
		out := make([]string, 0, len(m))
		for k := range m {
			out = append(out, k)
		}
		sort.Strings(out)
		return out, nil
	}
	var paths []string
	walkPaths(v, "", &paths, 0, maxDepth)
	sort.Strings(paths)
	return paths, nil
}

func walkPaths(v any, prefix string, paths *[]string, depth, maxDepth int) {
	if maxDepth > 0 && depth >= maxDepth {
		if prefix != "" {
			*paths = append(*paths, prefix)
		}
		return
	}
	switch node := v.(type) {
	case map[string]any:
		if len(node) == 0 && prefix != "" {
			*paths = append(*paths, prefix)
			return
		}
		for k, child := range node {
			escK := strings.ReplaceAll(k, ".", "\\.")
			p := escK
			if prefix != "" {
				p = prefix + "." + escK
			}
			walkPaths(child, p, paths, depth+1, maxDepth)
		}
	case []any:
		if len(node) == 0 && prefix != "" {
			*paths = append(*paths, prefix)
			return
		}
		for i, child := range node {
			p := fmt.Sprintf("%s[%d]", prefix, i)
			walkPaths(child, p, paths, depth+1, maxDepth)
		}
	default:
		if prefix != "" {
			*paths = append(*paths, prefix)
		}
	}
}
