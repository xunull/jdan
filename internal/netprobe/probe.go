// Package netprobe 实现 jdan net probe 的核心：把一个 URL/host:port 的连接
// 过程拆成 DNS → TCP → TLS → HTTP 四个独立阶段，逐阶段执行并报告。
//
// 设计要点：
//   - 阶段之间显式串行。前一阶段失败，后续阶段不跑。
//   - 多 IP 时逐个 TCP connect（不用 Go 默认的 happy eyeballs），让用户能
//     看到每个 IP 的具体结果（探查工具的核心价值）。
//   - 失败时根据错误类型映射到人类可读的 hint（见 hints.go）。
//   - 默认 HEAD，405 时 fallback GET 一次。
package netprobe

import (
	"context"
	"crypto/tls"
	"errors"
	"fmt"
	"net"
	"net/url"
	"strconv"
	"strings"
	"time"
)

// Options 控制一次 Probe 的行为。
type Options struct {
	// Timeout 是单个阶段的最大超时；总耗时上限 ≈ 4 × Timeout。
	// 0 时取默认 10s。
	Timeout time.Duration

	// Resolver 是显式指定的 DNS server（host[:port]），为空时走系统 resolver。
	Resolver string

	// Method 是 HTTP 阶段用的方法。空时默认 "HEAD"，405 时自动 fallback 到 GET。
	Method string

	// Insecure 跳过 TLS 证书验证（自签场景）。
	Insecure bool

	// Verbose 控制 TLS / HTTP 阶段是否捕获额外细节（全 cert chain、所有响应 header）。
	Verbose bool
}

// Target 是一个解析过的探查目标，区分 host / port / scheme。
type Target struct {
	Original string // 用户原始输入
	Scheme   string // http / https / tcp（没有 scheme 时默认 https）
	Host     string // hostname or literal IP
	Port     int    // 显式或推断
	Path     string // 只有 http/https scheme 时有意义，默认 "/"
}

// ParseTarget 解析用户输入：
//   - "https://example.com/x"  → https + example.com + 443 + /x
//   - "example.com"            → https + example.com + 443
//   - "example.com:80"         → http + example.com + 80（推断 http）
//   - "example.com:8443"       → https + example.com + 8443（非标准端口默认仍 https）
//   - "192.168.1.1:8080"       → http + 192.168.1.1 + 8080
//   - "192.168.1.1"            → 缺端口 + 缺 scheme，error
func ParseTarget(in string) (*Target, error) {
	in = strings.TrimSpace(in)
	if in == "" {
		return nil, errors.New("empty target")
	}

	// 有 scheme 的情况优先用 url.Parse
	if strings.Contains(in, "://") {
		u, err := url.Parse(in)
		if err != nil {
			return nil, fmt.Errorf("invalid URL: %w", err)
		}
		if u.Host == "" {
			return nil, errors.New("URL missing host")
		}
		host, portStr := splitHostPort(u.Host)
		port, err := resolvePort(portStr, u.Scheme)
		if err != nil {
			return nil, err
		}
		path := u.Path
		if path == "" && (u.Scheme == "http" || u.Scheme == "https") {
			path = "/"
		}
		return &Target{
			Original: in,
			Scheme:   u.Scheme,
			Host:     host,
			Port:     port,
			Path:     path,
		}, nil
	}

	// 没 scheme：可能是 "host"、"host:port"、IPv6 literal
	host, portStr := splitHostPort(in)
	if portStr == "" {
		// 没 port 也没 scheme → 默认 https + 443
		if host == "" {
			host = in
		}
		return &Target{
			Original: in,
			Scheme:   "https",
			Host:     host,
			Port:     443,
			Path:     "/",
		}, nil
	}
	port, err := strconv.Atoi(portStr)
	if err != nil {
		return nil, fmt.Errorf("invalid port %q", portStr)
	}
	scheme := inferSchemeFromPort(port)
	return &Target{
		Original: in,
		Scheme:   scheme,
		Host:     host,
		Port:     port,
		Path:     defaultPath(scheme),
	}, nil
}

func splitHostPort(s string) (host, port string) {
	if strings.HasPrefix(s, "[") {
		// IPv6 literal: [::1]:8080
		end := strings.Index(s, "]")
		if end == -1 {
			return s, ""
		}
		host = s[1:end]
		rest := s[end+1:]
		if strings.HasPrefix(rest, ":") {
			port = rest[1:]
		}
		return host, port
	}
	if idx := strings.LastIndex(s, ":"); idx >= 0 {
		// 不是 IPv6 的情况下，最后一个 ":" 是 port 分隔符
		// 但 "host.com" 没有 ":" 会返回 -1
		// 注意：如果 host 是 IPv4 + port，是 a.b.c.d:port，正确
		return s[:idx], s[idx+1:]
	}
	return s, ""
}

func resolvePort(portStr, scheme string) (int, error) {
	if portStr == "" {
		switch scheme {
		case "http":
			return 80, nil
		case "https":
			return 443, nil
		case "tcp":
			return 0, errors.New("tcp scheme requires explicit port")
		}
		return 443, nil
	}
	return strconv.Atoi(portStr)
}

func inferSchemeFromPort(port int) string {
	switch port {
	case 80, 8080, 8000, 5000, 3000:
		return "http"
	case 443, 8443:
		return "https"
	}
	// 其他端口默认 http（最小惊讶 — TLS 默认会跑 TLS 阶段，用户能看出）
	return "http"
}

func defaultPath(scheme string) string {
	if scheme == "http" || scheme == "https" {
		return "/"
	}
	return ""
}

// Stage 表示一个探查阶段的名字。
type Stage string

const (
	StageResolve Stage = "resolve"
	StageTCP     Stage = "tcp"
	StageTLS     Stage = "tls"
	StageHTTP    Stage = "http"
)

// StageResult 是一个阶段的结果。Success 决定后续阶段是否跑。
type StageResult struct {
	Stage    Stage         `json:"stage"`
	Success  bool          `json:"success"`
	Duration time.Duration `json:"duration_ns"`
	Detail   string        `json:"detail,omitempty"` // 一行人类可读概述
	Err      string        `json:"error,omitempty"`
	Hint     string        `json:"hint,omitempty"` // 失败时的修复 hint

	// 阶段特有字段
	Resolve *ResolveDetail `json:"resolve,omitempty"`
	TCP     *TCPDetail     `json:"tcp,omitempty"`
	TLS     *TLSDetail     `json:"tls,omitempty"`
	HTTP    *HTTPDetail    `json:"http,omitempty"`
}

// Result 是 Probe 的完整结果。
type Result struct {
	Target  *Target        `json:"target"`
	Stages  []*StageResult `json:"stages"`
	OK      bool           `json:"ok"`
	Total   time.Duration  `json:"total_ns"`
	Stopped Stage          `json:"stopped_at,omitempty"` // 如果中途停止，记录在哪一阶段
}

// stageEmit 是阶段进度回调，cli 层用它流式输出。可为 nil。
type stageEmit func(*StageResult)

// Probe 执行一次完整探查。
func Probe(ctx context.Context, target string, opts Options, emit stageEmit) (*Result, error) {
	tgt, err := ParseTarget(target)
	if err != nil {
		return nil, err
	}
	if opts.Timeout == 0 {
		opts.Timeout = 10 * time.Second
	}
	if opts.Method == "" {
		opts.Method = "HEAD"
	}

	start := time.Now()
	res := &Result{Target: tgt, OK: true}

	// stage 1: resolve
	stage := runResolve(ctx, tgt, opts)
	res.Stages = append(res.Stages, stage)
	if emit != nil {
		emit(stage)
	}
	if !stage.Success {
		res.OK = false
		res.Stopped = StageResolve
		res.Total = time.Since(start)
		return res, nil
	}
	ips := stage.Resolve.IPs

	// stage 2: tcp（对每个 IP 串行）
	tcpStage := runTCP(ctx, tgt, ips, opts)
	res.Stages = append(res.Stages, tcpStage)
	if emit != nil {
		emit(tcpStage)
	}
	if !tcpStage.Success {
		res.OK = false
		res.Stopped = StageTCP
		res.Total = time.Since(start)
		return res, nil
	}
	winningIP := tcpStage.TCP.WinningIP

	// stage 3: tls（仅 https）
	var tlsState *tls.ConnectionState
	if tgt.Scheme == "https" {
		tlsStage := runTLS(ctx, tgt, winningIP, opts)
		res.Stages = append(res.Stages, tlsStage)
		if emit != nil {
			emit(tlsStage)
		}
		if !tlsStage.Success {
			res.OK = false
			res.Stopped = StageTLS
			res.Total = time.Since(start)
			return res, nil
		}
		tlsState = tlsStage.TLS.connState
	}

	// stage 4: http（仅 http/https scheme）
	if tgt.Scheme == "http" || tgt.Scheme == "https" {
		httpStage := runHTTP(ctx, tgt, winningIP, tlsState, opts)
		res.Stages = append(res.Stages, httpStage)
		if emit != nil {
			emit(httpStage)
		}
		if !httpStage.Success {
			res.OK = false
			res.Stopped = StageHTTP
		}
	}

	res.Total = time.Since(start)
	return res, nil
}

// 给 net.JoinHostPort 用的封装，避免在 caller 散写
func dialAddr(ip net.IP, port int) string {
	return net.JoinHostPort(ip.String(), strconv.Itoa(port))
}
