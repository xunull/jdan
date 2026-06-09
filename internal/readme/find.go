package readme

import (
	"fmt"
	"os"
	"path/filepath"
	"strings"
)

// FindReadme 在 dir 中查找 README.md 文件（大小写不敏感）。
// 优先返回精确匹配的 "README.md"，否则返回第一个大小写变体。
func FindReadme(dir string) (string, error) {
	info, err := os.Stat(dir)
	if err != nil {
		return "", fmt.Errorf("无法访问目录 %s: %w", dir, err)
	}
	if !info.IsDir() {
		return "", fmt.Errorf("%s 不是一个目录", dir)
	}

	entries, err := os.ReadDir(dir)
	if err != nil {
		return "", fmt.Errorf("读取目录 %s 失败: %w", dir, err)
	}

	var variant string
	for _, e := range entries {
		if e.IsDir() {
			continue
		}
		name := e.Name()
		if name == "README.md" {
			return filepath.Join(dir, name), nil
		}
		if variant == "" && strings.EqualFold(name, "README.md") {
			variant = name
		}
	}
	if variant != "" {
		return filepath.Join(dir, variant), nil
	}
	return "", fmt.Errorf("在 %s 中未找到 README.md", dir)
}
