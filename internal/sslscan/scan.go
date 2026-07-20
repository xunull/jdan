// Package sslscan 实现 jdan ssl scan 子命令：对 HTTPS host 做综合 TLS
// 配置审计——版本、cipher、ALPN、HSTS、session resumption、cert 概要——
// 加权打分给出 ssllabs 风格的 grade (A+/A/B/C/D/F)。
//
// 设计意图：替代 ssllabs.com 在内网 / 私有 host 场景的本地能力；CI/CD
// 安全门禁；快速回答"我这 server TLS 配置安全吗"。
//
// 不在 scope 内：SSL 3.0（Go stdlib 已移除），weak cipher 的密码学评估
// （走静态分类表，不做实时分析）。
package sslscan

import (
	"context"
	"errors"
	"time"
)

// Options 控制一次 Scan 的范围与行为。
type Options struct {
	Host       string
	Port       int           // 默认 443
	SNI        string        // 默认 Host
	Timeout    time.Duration // 整体超时；默认 15s
	FullCipher bool          // 默认 16 常见 cipher；true 试 Go stdlib 暴露的 ~40 个
	SkipCipher bool          // 跳过 cipher 枚举（更快）
	SkipHSTS   bool          // 跳过 HTTPS GET 抓 HSTS header
	SkipResume bool          // 跳过 session resumption 测试
}

// ScanReport 是一次完整审计的输出。各 section 字段对应 cli render 的 box。
type ScanReport struct {
	Target   string          `json:"target"`
	Cert     *CertSection    `json:"cert,omitempty"`
	Versions VersionsSection `json:"tls_versions"`
	Ciphers  CiphersSection  `json:"cipher_suites"`
	ALPN     ALPNSection     `json:"alpn"`
	HSTS     *HSTSSection    `json:"hsts,omitempty"`
	Resume   ResumeSection   `json:"session_resumption"`
	Grade    GradeReport     `json:"grade"`
	Elapsed  time.Duration   `json:"elapsed_ns"`
}

// Scan 主入口。各 section 串行跑（不并行：握手对 TLS 状态机有副作用，并行
// 让结果可重现性更差）。每个 section 失败不传播——失败的 section 留空，
// 让 grade.go 用现有数据评分。
func Scan(ctx context.Context, opts Options) (*ScanReport, error) {
	if opts.Host == "" {
		return nil, errors.New("host required")
	}
	if opts.Port == 0 {
		opts.Port = 443
	}
	if opts.SNI == "" {
		opts.SNI = opts.Host
	}
	if opts.Timeout == 0 {
		opts.Timeout = 15 * time.Second
	}

	ctx, cancel := context.WithTimeout(ctx, opts.Timeout)
	defer cancel()

	start := time.Now()
	r := &ScanReport{
		Target: opts.Host,
	}

	// cert 概要（复用 internal/sslcert）
	r.Cert = scanCert(ctx, opts)

	// TLS 版本支持
	r.Versions = scanVersions(ctx, opts)

	// cipher 枚举（默认只跑 TLS 1.2，因为 1.3 cipher 固定）
	if !opts.SkipCipher {
		r.Ciphers = scanCiphers(ctx, opts)
	}

	// ALPN 协议探测
	r.ALPN = scanALPN(ctx, opts)

	// HSTS（HTTPS GET 看响应 header）
	if !opts.SkipHSTS {
		r.HSTS = scanHSTS(ctx, opts)
	}

	// Session resumption
	if !opts.SkipResume {
		r.Resume = scanResume(ctx, opts)
	}

	// 综合评分
	r.Grade = computeGrade(r)

	r.Elapsed = time.Since(start)
	return r, nil
}
