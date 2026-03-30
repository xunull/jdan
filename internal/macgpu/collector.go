package macgpu

import (
	"bytes"
	"context"
	"fmt"
	"os/exec"
	"strconv"
	"strings"

	tea "charm.land/bubbletea/v2"
)

// SampleMsg 携带一次成功采样的 GPU 快照，由 Collector 发送给 Bubble Tea 程序。
type SampleMsg struct {
	Snapshot *GPUSnapshot
}

// ErrMsg 携带采集或解析过程中发生的错误。
type ErrMsg struct {
	Err error
}

// Collector 管理 powermetrics 子进程的生命周期，
// 并通过 p.Send() 向 Bubble Tea 程序推送采样结果。
type Collector struct {
	ctx      context.Context
	interval int // 采样间隔，单位 ms
	program  *tea.Program
}

// powermetricsPath 使用绝对路径，避免 sudo 环境下 PATH 被精简导致找不到命令。
const powermetricsPath = "/usr/bin/powermetrics"

// NewCollector 创建一个新的 Collector。
func NewCollector(ctx context.Context, intervalMS int, program *tea.Program) *Collector {
	return &Collector{
		ctx:      ctx,
		interval: intervalMS,
		program:  program,
	}
}

// Start 在新 goroutine 中启动采集循环。
// 使用 -n 1 循环采集，而非 -n 0（部分 macOS 版本将 -n 0 解释为"采集 0 条"导致立即退出）。
func (c *Collector) Start() {
	go c.run()
}

func (c *Collector) run() {
	for {
		select {
		case <-c.ctx.Done():
			return
		default:
		}

		if err := c.collectOnce(); err != nil {
			c.program.Send(ErrMsg{Err: err})
			return
		}
	}
}

// collectOnce 以 -n 1 运行一次 powermetrics，阻塞直到输出一条采样后返回。
// 返回 nil 表示成功或 ctx 已取消（应退出循环）；返回 error 表示不可恢复的失败。
func (c *Collector) collectOnce() error {
	args := []string{
		"--samplers", "gpu_power,thermal",
		"--format", "plist",
		"-i", strconv.Itoa(c.interval),
		"-n", "1",
	}

	cmd := exec.CommandContext(c.ctx, powermetricsPath, args...)

	// 捕获 stderr 用于诊断，避免错误信息泄漏到终端破坏 TUI 渲染。
	var stderrBuf bytes.Buffer
	cmd.Stderr = &stderrBuf

	out, err := cmd.Output()

	// 优先检查 ctx：ExitError 可能是 context 引起的 kill，不应视为错误。
	select {
	case <-c.ctx.Done():
		return nil
	default:
	}

	if err != nil {
		stderrStr := strings.TrimSpace(stderrBuf.String())
		if stderrStr != "" {
			return fmt.Errorf("powermetrics 错误: %s", stderrStr)
		}
		return fmt.Errorf("powermetrics 执行失败: %w", err)
	}

	// powermetrics plist 输出以 \x00 结尾，去除后解析。
	data := bytes.TrimRight(out, "\x00")
	data = bytes.TrimSpace(data)
	if len(data) == 0 {
		return nil // 空输出，跳过本次采样继续循环
	}

	snapshot, parseErr := ParseSample(data)
	if parseErr != nil {
		// 单次解析失败不终止循环，报错后继续下一次采样。
		c.program.Send(ErrMsg{Err: fmt.Errorf("解析 plist 失败: %w", parseErr)})
		return nil
	}

	c.program.Send(SampleMsg{Snapshot: snapshot})
	return nil
}
