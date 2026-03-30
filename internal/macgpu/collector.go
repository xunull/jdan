package macgpu

import (
	"bufio"
	"bytes"
	"context"
	"os/exec"
	"strconv"

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

func (c *Collector) run() {
	args := []string{
		"--samplers", "gpu_power,thermal",
		"--format", "plist",
		"-i", strconv.Itoa(c.interval),
		"-n", "0",
	}

	cmd := exec.CommandContext(c.ctx, "powermetrics", args...)
	stdout, err := cmd.StdoutPipe()
	if err != nil {
		c.program.Send(ErrMsg{Err: err})
		return
	}

	if err := cmd.Start(); err != nil {
		c.program.Send(ErrMsg{Err: err})
		return
	}

	scanner := bufio.NewScanner(stdout)
	scanner.Split(splitOnNUL)

	// bufio.Scanner 默认缓冲区为 64KB，powermetrics plist 输出可能超过此限制。
	// 将缓冲区扩大至 4MB，以适应多频率档位的大型 plist 块。
	scanner.Buffer(make([]byte, 4*1024*1024), 4*1024*1024)

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
			c.program.Send(ErrMsg{Err: parseErr})
			continue
		}
		c.program.Send(SampleMsg{Snapshot: snapshot})
	}

	if err := scanner.Err(); err != nil {
		select {
		case <-c.ctx.Done():
			// context 取消引起的读取错误是预期行为，不上报
		default:
			c.program.Send(ErrMsg{Err: err})
		}
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
