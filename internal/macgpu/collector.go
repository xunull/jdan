package macgpu

import (
	"bufio"
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
// 逐 NUL 块扫描 plist 输出，并通过 p.Send() 向 Bubble Tea 程序推送消息。
type Collector struct {
	ctx      context.Context
	interval int // 采样间隔，单位 ms
	program  *tea.Program
}

// NewCollector 创建一个新的 Collector。
// ctx 取消时，Collector goroutine 和子进程会一同退出。
func NewCollector(ctx context.Context, intervalMS int, program *tea.Program) *Collector {
	return &Collector{
		ctx:      ctx,
		interval: intervalMS,
		program:  program,
	}
}

// Start 在新 goroutine 中启动 powermetrics 子进程并持续采集。
// 采集结果通过 p.Send() 推送为 SampleMsg 或 ErrMsg。
// 调用者无需等待；当 ctx 被取消时，goroutine 自动退出。
func (c *Collector) Start() {
	go c.run()
}

const powermetricsPath = "/usr/bin/powermetrics"

func (c *Collector) run() {
	args := []string{
		"--samplers", "gpu_power,thermal",
		"--format", "plist",
		"-i", strconv.Itoa(c.interval),
		"-n", "0",
	}

	// 使用全路径，避免 sudo 环境下 PATH 不包含 /usr/bin 的问题。
	cmd := exec.CommandContext(c.ctx, powermetricsPath, args...)

	// 捕获 stderr，用于在 powermetrics 异常退出时提供诊断信息。
	var stderrBuf bytes.Buffer
	cmd.Stderr = &stderrBuf

	stdout, err := cmd.StdoutPipe()
	if err != nil {
		c.program.Send(ErrMsg{Err: fmt.Errorf("无法获取 powermetrics 输出管道: %w", err)})
		return
	}

	if err := cmd.Start(); err != nil {
		c.program.Send(ErrMsg{Err: fmt.Errorf("无法启动 powermetrics: %w", err)})
		return
	}

	scanner := bufio.NewScanner(stdout)
	scanner.Split(splitOnNUL)

	// bufio.Scanner 默认缓冲区为 64KB，powermetrics plist 输出可能超过此限制。
	// 将缓冲区扩大至 4MB，以适应多频率档位的大型 plist 块。
	scanner.Buffer(make([]byte, 4*1024*1024), 4*1024*1024)

	samplesReceived := 0

	for scanner.Scan() {
		select {
		case <-c.ctx.Done():
			return
		default:
		}

		data := scanner.Bytes()
		if len(bytes.TrimSpace(data)) == 0 {
			continue
		}

		snapshot, parseErr := ParseSample(data)
		if parseErr != nil {
			c.program.Send(ErrMsg{Err: fmt.Errorf("解析 plist 失败: %w", parseErr)})
			continue
		}
		samplesReceived++
		c.program.Send(SampleMsg{Snapshot: snapshot})
	}

	// 检查 context 是否已取消（用户主动退出），若是则静默退出。
	select {
	case <-c.ctx.Done():
		return
	default:
	}

	// powermetrics 在 context 未取消的情况下退出，属于异常。
	// 收集诊断信息并上报 ErrMsg，让 TUI 显示错误而非永远停在"等待"状态。
	_ = cmd.Wait() // 等待子进程退出，获取退出码

	stderrStr := strings.TrimSpace(stderrBuf.String())
	if scanErr := scanner.Err(); scanErr != nil {
		c.program.Send(ErrMsg{Err: fmt.Errorf("读取 powermetrics 输出出错: %w", scanErr)})
	} else if stderrStr != "" {
		c.program.Send(ErrMsg{Err: fmt.Errorf("powermetrics 异常退出: %s", stderrStr)})
	} else if samplesReceived == 0 {
		c.program.Send(ErrMsg{Err: fmt.Errorf(
			"powermetrics 未输出任何数据即退出（尝试手动执行: sudo %s %s）",
			powermetricsPath, strings.Join(args, " "),
		)})
	} else {
		c.program.Send(ErrMsg{Err: fmt.Errorf("powermetrics 意外退出（已采集 %d 条）", samplesReceived)})
	}
}

// splitOnNUL 是 bufio.Scanner 的自定义分割函数，
// 按 NUL 字节（0x00）分隔 powermetrics 的连续 plist 输出。
func splitOnNUL(data []byte, atEOF bool) (advance int, token []byte, err error) {
	if i := bytes.IndexByte(data, 0x00); i >= 0 {
		return i + 1, data[:i], nil
	}
	if atEOF && len(data) > 0 {
		return len(data), data, nil
	}
	return 0, nil, nil
}
