package netprobe

import (
	"context"
	"errors"
	"fmt"
	"net"
	"time"
)

// TCPHealthDetail 是 tcp_health 阶段的特有字段。
type TCPHealthDetail struct {
	HoldDuration   time.Duration `json:"hold_duration_ns"`     // 实际 hold 多长（成功时 ≈ Options.HealthDuration，失败时是真实耗时）
	RemoteClosed   bool          `json:"remote_closed"`        // 远端在 hold 期内主动关闭
	GotBanner      bool          `json:"got_banner"`           // 远端主动推送数据（SSH/SMTP/POP3 welcome）
	BannerPreview  string        `json:"banner_preview,omitempty"`
}

// runTCPHealth 在 TCP 阶段已经 SUCCESS 的基础上，再 dial 一次同一 IP，
// 然后 Read 等 hold 时长。可能的结果：
//
//   - hold 时长内 Read 都没返回（i/o timeout）→ 连接健康（server 在等 client 发请求）
//   - Read 返回数据 → server 主动 banner（SSH/SMTP/POP3）
//   - Read 返回 ECONNRESET → REMOTE_RESET (stateful firewall 踢)
//   - Read 返回 EOF → REMOTE_CLOSED (FIN)
//
// 为什么不重用 TCP 阶段的 conn：TCP 阶段已经 Close 了 conn（设计上一阶段一 dial）。
func runTCPHealth(ctx context.Context, t *Target, ip net.IP, opts Options) *StageResult {
	stageStart := time.Now()
	r := &StageResult{Stage: StageTCPHealth}
	d := &TCPHealthDetail{}

	addr := dialAddr(ip, t.Port)
	dialer := &net.Dialer{Timeout: opts.Timeout}

	conn, err := dialer.DialContext(ctx, "tcp", addr)
	if err != nil {
		// 不应该发生（TCP 阶段已成功），保守起见处理
		r.Success = false
		r.Class = ClassifyTCPError(err)
		r.Explanation = WhatItMeans(r.Class)
		r.Hint = HintForClass(r.Class)
		r.Err = err.Error()
		r.Detail = "re-dial failed after TCP success"
		r.Duration = time.Since(stageStart)
		r.TCPHealth = d
		return r
	}
	defer conn.Close()

	hold := opts.HealthDuration
	if hold == 0 {
		hold = 1 * time.Second
	}
	d.HoldDuration = hold

	_ = conn.SetReadDeadline(time.Now().Add(hold))
	buf := make([]byte, 256)
	n, readErr := conn.Read(buf)
	r.Duration = time.Since(stageStart)

	if readErr == nil {
		// 远端主动 push 了数据（banner case）。不是错误。
		d.GotBanner = true
		d.BannerPreview = previewBanner(buf[:n])
		r.Success = true
		r.Detail = fmt.Sprintf("server pushed banner (%d bytes): %s", n, d.BannerPreview)
		r.TCPHealth = d
		return r
	}

	var ne net.Error
	if errors.As(readErr, &ne) && ne.Timeout() {
		// hold 内没消息 = connection 健康（HTTP server 正等 client 发请求）
		r.Success = true
		r.Detail = fmt.Sprintf("held %s without remote close (healthy)", formatHoldShort(hold))
		r.TCPHealth = d
		return r
	}

	// 真正的远端关闭
	d.RemoteClosed = true
	r.Class = ClassifyTCPHealthError(readErr)
	r.Explanation = WhatItMeans(r.Class)
	r.Hint = HintForClass(r.Class)
	r.Err = readErr.Error()
	r.Success = false
	r.Detail = fmt.Sprintf("remote closed during %s hold", formatHoldShort(hold))
	r.TCPHealth = d
	return r
}

func previewBanner(b []byte) string {
	// 把 banner 截短到第一行（去掉 trailing newline）+ 至多 60 字符
	s := string(b)
	if i := indexAny(s, "\r\n"); i >= 0 {
		s = s[:i]
	}
	if len(s) > 60 {
		s = s[:60] + "..."
	}
	return s
}

func indexAny(s, chars string) int {
	for i := 0; i < len(s); i++ {
		for j := 0; j < len(chars); j++ {
			if s[i] == chars[j] {
				return i
			}
		}
	}
	return -1
}

func formatHoldShort(d time.Duration) string {
	if d < time.Second {
		return fmt.Sprintf("%dms", d.Milliseconds())
	}
	return fmt.Sprintf("%ds", int(d.Seconds()))
}
