package netprobe

import (
	"context"
	"fmt"
	"net"
	"time"
)

// TCPAttempt 是对单个 IP 的 TCP 连接尝试结果。
type TCPAttempt struct {
	IP        net.IP        `json:"ip"`
	Success   bool          `json:"success"`
	Duration  time.Duration `json:"duration_ns"`
	LocalAddr string        `json:"local_addr,omitempty"`
	Err       string        `json:"error,omitempty"`
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
		// hint 用第一次失败的 error 推断
		if len(d.Attempts) > 0 {
			r.Err = d.Attempts[0].Err
			r.Hint = hintForTCPError(d.Attempts[0].Err)
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
		return TCPAttempt{IP: ip, Success: false, Duration: dur, Err: err.Error()}
	}
	local := conn.LocalAddr().String()
	_ = conn.Close()
	return TCPAttempt{IP: ip, Success: true, Duration: dur, LocalAddr: local}
}

func formatDurationMs(d time.Duration) string {
	if d < time.Millisecond {
		return fmt.Sprintf("%dµs", d.Microseconds())
	}
	return fmt.Sprintf("%dms", d.Milliseconds())
}
