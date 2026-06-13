package netprobe

import (
	"context"
	"fmt"
	"net"
	"strings"
	"time"
)

// TCPAttempt 是对单个 IP 的 TCP 连接尝试结果。
type TCPAttempt struct {
	IP        net.IP        `json:"ip"`
	Success   bool          `json:"success"`
	Duration  time.Duration `json:"duration_ns"`
	LocalAddr string        `json:"local_addr,omitempty"`
	Err       string        `json:"error,omitempty"`

	// lastErr 保留原始 error 对象供 ClassifyTCPError 用 errors.Is/As。
	// unexported 字段 encoding/json 不会序列化，正合我意。
	lastErr error
}

// TCPDetail 是 TCP 阶段的特有字段。
type TCPDetail struct {
	Attempts  []TCPAttempt `json:"attempts"`
	WinningIP net.IP       `json:"winning_ip,omitempty"`
}

// runTCP 对每个 IP 做串行 TCP connect。第一个成功的 IP 成为 WinningIP，
// 后续阶段（TLS / HTTP）针对它做。所有 IP 全失败 → 阶段失败。
//
// 为什么不用 happy eyeballs：探查工具的核心价值是"告诉我每个 IP 的具体结果"。
// happy eyeballs 隐藏了非赢家的失败。
func runTCP(ctx context.Context, t *Target, ips []net.IP, opts Options) *StageResult {
	stageStart := time.Now()
	r := &StageResult{Stage: StageTCP}
	d := &TCPDetail{}

	dialer := &net.Dialer{Timeout: opts.Timeout}

	for _, ip := range ips {
		attempt := tryTCP(ctx, dialer, ip, t.Port)
		d.Attempts = append(d.Attempts, attempt)
		if attempt.Success && d.WinningIP == nil {
			d.WinningIP = ip
		}
	}

	r.Duration = time.Since(stageStart)
	r.TCP = d

	if d.WinningIP == nil {
		r.Success = false
		r.Detail = fmt.Sprintf("%d attempt(s), all failed", len(d.Attempts))
		// 用第一次失败的 error 推断 class（实际上各 IP 失败原因往往相同）
		if len(d.Attempts) > 0 {
			r.Err = d.Attempts[0].Err
			// 重新分类需要原始 error，但 Attempts 存的是 string；TCP 阶段
			// 比较特殊——我们这里保留原错误对象在临时 slice 里以便 classify
			if d.Attempts[0].lastErr != nil {
				r.Class = ClassifyTCPError(d.Attempts[0].lastErr)
			} else {
				// 兜底：从字符串重新分类
				r.Class = classifyTCPErrorFromString(d.Attempts[0].Err)
			}
			r.Explanation = WhatItMeans(r.Class)
			r.Hint = HintForClass(r.Class)
		}
		return r
	}
	r.Success = true
	r.Detail = fmt.Sprintf("connected to %s:%d in %s",
		d.WinningIP, t.Port, formatDurationMs(d.attemptDuration(d.WinningIP)))
	return r
}

func (d *TCPDetail) attemptDuration(ip net.IP) time.Duration {
	for _, a := range d.Attempts {
		if a.IP.Equal(ip) {
			return a.Duration
		}
	}
	return 0
}

func tryTCP(ctx context.Context, dialer *net.Dialer, ip net.IP, port int) TCPAttempt {
	addr := dialAddr(ip, port)
	start := time.Now()
	conn, err := dialer.DialContext(ctx, "tcp", addr)
	dur := time.Since(start)
	if err != nil {
		return TCPAttempt{IP: ip, Success: false, Duration: dur, Err: err.Error(), lastErr: err}
	}
	local := conn.LocalAddr().String()
	_ = conn.Close()
	return TCPAttempt{IP: ip, Success: true, Duration: dur, LocalAddr: local}
}

// classifyTCPErrorFromString 是 ClassifyTCPError 的 string-only 兜底版本，
// 用于 lastErr 因某种原因丢失时（不应发生，但保守）。
func classifyTCPErrorFromString(s string) ErrorClass {
	low := strings.ToLower(s)
	switch {
	case strings.Contains(low, "connection refused"):
		return ClassConnRefused
	case strings.Contains(low, "i/o timeout"), strings.Contains(low, "timed out"):
		return ClassConnTimeout
	case strings.Contains(low, "no route to host"), strings.Contains(low, "host is down"):
		return ClassNoRoute
	case strings.Contains(low, "network is unreachable"):
		return ClassNetUnreachable
	}
	return ClassUnknown
}

func formatDurationMs(d time.Duration) string {
	if d < time.Millisecond {
		return fmt.Sprintf("%dµs", d.Microseconds())
	}
	return fmt.Sprintf("%dms", d.Milliseconds())
}
