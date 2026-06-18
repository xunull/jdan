// Package gitx 实现 jdan git 子命令的核心：shell out 到 git 拿信息再解析。
// 0 新 Go 依赖，只要运行环境里有 git 可执行文件。
package gitx

import (
	"bytes"
	"fmt"
	"os/exec"
	"strings"
)

// Runner 跑一条 git 命令（在 dir 目录下），返回 stdout。便于测试注入假实现。
type Runner func(dir string, args ...string) (string, error)

// ExecRunner 是生产用的 Runner：真实调用 git 可执行文件。
func ExecRunner(dir string, args ...string) (string, error) {
	cmd := exec.Command("git", args...)
	if dir != "" {
		cmd.Dir = dir
	}
	var stdout, stderr bytes.Buffer
	cmd.Stdout = &stdout
	cmd.Stderr = &stderr
	if err := cmd.Run(); err != nil {
		if _, ok := err.(*exec.Error); ok {
			// git 不在 PATH 上
			return "", fmt.Errorf("找不到 git 可执行文件，请先安装 git")
		}
		msg := strings.TrimSpace(stderr.String())
		if msg == "" {
			msg = err.Error()
		}
		return "", fmt.Errorf("git %s: %s", strings.Join(args, " "), msg)
	}
	return stdout.String(), nil
}

// IsRepo 报告 dir 是否在一个 git 工作区内。
func IsRepo(run Runner, dir string) bool {
	out, err := run(dir, "rev-parse", "--is-inside-work-tree")
	return err == nil && strings.TrimSpace(out) == "true"
}
