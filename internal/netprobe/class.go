package netprobe

import (
	"errors"
	"io"
	"net"
	"strings"
	"syscall"
)

// ErrorClass 是给用户看的"错误是什么"分类。机器可读 stable enum，
// 配合 whatItMeans 文案和 hint 修复建议给用户三层信息：
//   - 标签：CONNECTION_REFUSED（醒目，0.5 秒识别）
//   - 解释：服务端收到 SYN 但回了 RST...（中等长度，理解原因）
//   - 建议：lsof -i :PORT / 防火墙白名单 ...（具体操作）
type ErrorClass string

const (
	ClassNone    ErrorClass = ""
	ClassUnknown ErrorClass = "UNKNOWN"

	// resolve 阶段
	ClassDNSNoSuchHost   ErrorClass = "DNS_NO_SUCH_HOST"
	ClassDNSResolverDown ErrorClass = "DNS_RESOLVER_UNREACHABLE"
	ClassDNSTimeout      ErrorClass = "DNS_TIMEOUT"

	// tcp connect 阶段——"建立连接失败"
	ClassConnRefused    ErrorClass = "CONNECTION_REFUSED"
	ClassConnTimeout    ErrorClass = "CONNECTION_TIMEOUT"
	ClassNoRoute        ErrorClass = "NO_ROUTE_TO_HOST"
	ClassNetUnreachable ErrorClass = "NETWORK_UNREACHABLE"

	// tcp_health 阶段（建好后）——"被远程关闭"
	ClassRemoteReset ErrorClass = "REMOTE_RESET_AFTER_CONNECT"
	ClassRemoteEOF   ErrorClass = "REMOTE_CLOSED_AFTER_CONNECT"
	// 注：server banner（SSH/SMTP welcome line）不算错误，tcphealth.go 里直接特判

	// tls 阶段
	ClassTLSCertInvalid   ErrorClass = "TLS_CERT_INVALID"
	ClassTLSHandshakeFail ErrorClass = "TLS_HANDSHAKE_FAIL"
	ClassTLSNotHTTPS      ErrorClass = "TLS_PLAIN_HTTP_ON_TLS_PORT"

	// http 阶段
	ClassHTTPProtocolErr ErrorClass = "HTTP_PROTOCOL_ERROR"
	ClassHTTPClientError ErrorClass = "HTTP_4XX"
	ClassHTTPServerError ErrorClass = "HTTP_5XX"
)

// ClassifyDNSError 把 DNS 阶段错误分类。
func ClassifyDNSError(err error) ErrorClass {
	if err == nil {
		return ClassNone
	}
	// net.DNSError 暴露 IsNotFound / IsTimeout 字段
	var dnsErr *net.DNSError
	if errors.As(err, &dnsErr) {
		if dnsErr.IsNotFound {
			return ClassDNSNoSuchHost
		}
		if dnsErr.IsTimeout {
			return ClassDNSTimeout
		}
	}
	// 通用 timeout 接口
	var ne net.Error
	if errors.As(err, &ne) && ne.Timeout() {
		return ClassDNSTimeout
	}
	// errno fallback
	if errors.Is(err, syscall.ECONNREFUSED) || errors.Is(err, syscall.ETIMEDOUT) {
		return ClassDNSResolverDown
	}
	s := strings.ToLower(err.Error())
	if strings.Contains(s, "no such host") {
		return ClassDNSNoSuchHost
	}
	if strings.Contains(s, "i/o timeout") || strings.Contains(s, "timeout") {
		return ClassDNSTimeout
	}
	if strings.Contains(s, "connection refused") || strings.Contains(s, "no route") {
		return ClassDNSResolverDown
	}
	return ClassUnknown
}

// ClassifyTCPError 把 TCP connect 阶段错误分类。
func ClassifyTCPError(err error) ErrorClass {
	if err == nil {
		return ClassNone
	}
	// 1. syscall errno（跨 Go 版本最稳定）
	if errors.Is(err, syscall.ECONNREFUSED) {
		return ClassConnRefused
	}
	if errors.Is(err, syscall.ETIMEDOUT) {
		return ClassConnTimeout
	}
	if errors.Is(err, syscall.ENETUNREACH) {
		return ClassNetUnreachable
	}
	if errors.Is(err, syscall.EHOSTUNREACH) {
		return ClassNoRoute
	}
	// 2. net.Error.Timeout() 接口
	var ne net.Error
	if errors.As(err, &ne) && ne.Timeout() {
		return ClassConnTimeout
	}
	// 3. 字符串关键词兜底
	s := strings.ToLower(err.Error())
	if strings.Contains(s, "connection refused") {
		return ClassConnRefused
	}
	if strings.Contains(s, "i/o timeout") || strings.Contains(s, "timed out") {
		return ClassConnTimeout
	}
	if strings.Contains(s, "no route to host") {
		return ClassNoRoute
	}
	if strings.Contains(s, "network is unreachable") {
		return ClassNetUnreachable
	}
	if strings.Contains(s, "host is down") {
		return ClassNoRoute
	}
	return ClassUnknown
}

// ClassifyTCPHealthError 在 TCP 已建立的连接上 Read 时的错误分类。
// timeout 不算错误（说明连接 healthy），调用方应在 timeout 时返回 ClassNone。
func ClassifyTCPHealthError(err error) ErrorClass {
	if err == nil {
		return ClassNone
	}
	if errors.Is(err, syscall.ECONNRESET) {
		return ClassRemoteReset
	}
	if errors.Is(err, io.EOF) {
		return ClassRemoteEOF
	}
	s := strings.ToLower(err.Error())
	if strings.Contains(s, "connection reset") {
		return ClassRemoteReset
	}
	if strings.Contains(s, "broken pipe") {
		return ClassRemoteReset
	}
	if strings.Contains(s, "eof") {
		return ClassRemoteEOF
	}
	return ClassUnknown
}

// ClassifyTLSError 把 TLS 握手错误分类。
func ClassifyTLSError(err error) ErrorClass {
	if err == nil {
		return ClassNone
	}
	s := strings.ToLower(err.Error())
	// 证书相关问题
	if strings.Contains(s, "unknown authority") ||
		strings.Contains(s, "certificate has expired") ||
		strings.Contains(s, "certificate is not valid") ||
		strings.Contains(s, "x509:") {
		return ClassTLSCertInvalid
	}
	// 服务端是 plain HTTP
	if strings.Contains(s, "first record does not look like a tls handshake") ||
		strings.Contains(s, "http response") {
		return ClassTLSNotHTTPS
	}
	// 握手失败的其他形态
	if strings.Contains(s, "handshake failure") ||
		strings.Contains(s, "protocol version") ||
		strings.Contains(s, "unexpected eof") {
		return ClassTLSHandshakeFail
	}
	return ClassUnknown
}

// ClassifyHTTPError 把 HTTP 阶段的网络/协议错误分类。HTTP status code 由
// ClassifyHTTPStatus 单独处理。
func ClassifyHTTPError(err error) ErrorClass {
	if err == nil {
		return ClassNone
	}
	if errors.Is(err, syscall.ECONNRESET) {
		return ClassRemoteReset
	}
	if errors.Is(err, io.EOF) {
		return ClassRemoteEOF
	}
	s := strings.ToLower(err.Error())
	if strings.Contains(s, "connection reset") {
		return ClassRemoteReset
	}
	if strings.Contains(s, "eof") {
		return ClassRemoteEOF
	}
	if strings.Contains(s, "timeout") {
		return ClassConnTimeout
	}
	return ClassHTTPProtocolErr
}

// ClassifyHTTPStatus 4xx/5xx → class。2xx/3xx 返回 None。
func ClassifyHTTPStatus(code int) ErrorClass {
	switch {
	case code >= 500:
		return ClassHTTPServerError
	case code >= 400:
		return ClassHTTPClientError
	}
	return ClassNone
}

// classInfo 是每个 ErrorClass 对应的"what it means"解释 + 修复建议。
// 分两层让 cli render 能显示"标签 / 解释 / 建议"三层结构。
type classInfo struct {
	WhatItMeans string
	Hint        string
}

var classCatalog = map[ErrorClass]classInfo{
	ClassDNSNoSuchHost: {
		WhatItMeans: "hostname does not exist in DNS.",
		Hint: "check the spelling; try a different resolver:\n" +
			"  jdan net probe <host> --resolver 8.8.8.8",
	},
	ClassDNSResolverDown: {
		WhatItMeans: "DNS resolver unreachable (timeout or connection refused).",
		Hint: "try a known-good public resolver:\n" +
			"  jdan net probe <host> --resolver 1.1.1.1",
	},
	ClassDNSTimeout: {
		WhatItMeans: "DNS query timed out before any response.",
		Hint:        "local DNS may be slow or filtered; try --resolver 8.8.8.8",
	},
	ClassConnRefused: {
		WhatItMeans: "target host received our SYN but responded with RST.\n" +
			"either no process is listening on this port, or a host-level\n" +
			"firewall is actively rejecting connections.",
		Hint: "what to check:\n" +
			"  • target host not listening (check: lsof -i :PORT on target)\n" +
			"  • OS firewall blocking (macOS App Firewall, ufw, Windows Defender)\n" +
			"  ↳ run `jdan net selfcheck :PORT` on the target host to investigate",
	},
	ClassConnTimeout: {
		WhatItMeans: "SYN sent but no response within timeout.\n" +
			"packets are being silently dropped — host-level firewall, network\n" +
			"middlebox, or target is offline.",
		Hint: "what to check:\n" +
			"  • host firewall silently dropping packets\n" +
			"  • routing issue (no return path back to you)\n" +
			"  • target host is down or behind NAT without port-forwarding",
	},
	ClassNoRoute: {
		WhatItMeans: "no path from your host to the target's network.\n" +
			"the IP stack itself rejected the destination.",
		Hint: "what to check:\n" +
			"  • you may be on a different LAN segment\n" +
			"  • check VPN status and `route get <ip>`",
	},
	ClassNetUnreachable: {
		WhatItMeans: "your local network has no usable default route.\n" +
			"the OS itself rejected before any packet went out.",
		Hint: "check `ifconfig` / `ip addr`; verify Wi-Fi/Ethernet is up",
	},
	ClassRemoteReset: {
		WhatItMeans: "TCP handshake succeeded, but the remote sent RST\n" +
			"before we sent any application data. typical cause:\n" +
			"stateful firewall or IPS terminating after policy check.",
		Hint: "what to check:\n" +
			"  • stateful firewall / IPS with IP allowlist\n" +
			"  • cloud LB with health check failures\n" +
			"  • reverse proxy refusing the source IP",
	},
	ClassRemoteEOF: {
		WhatItMeans: "remote closed the connection (FIN) right after TCP\n" +
			"handshake. server accepted but immediately said goodbye.",
		Hint: "what to check:\n" +
			"  • server is in shutdown / drain state\n" +
			"  • protocol mismatch (e.g. sending to a UDP-only port that exists as TCP)",
	},
	ClassTLSCertInvalid: {
		WhatItMeans: "TLS cert is rejected: self-signed, expired, wrong hostname,\n" +
			"or untrusted CA. handshake itself worked.",
		Hint: "to bypass for inspection: add --insecure",
	},
	ClassTLSHandshakeFail: {
		WhatItMeans: "TLS handshake failed before completion (protocol error,\n" +
			"version mismatch, server requires client cert, or aborted mid-handshake).",
		Hint: "if you suspect a middlebox cutting the handshake,\n" +
			"  try --insecure to isolate cert verification from protocol failures",
	},
	ClassTLSNotHTTPS: {
		WhatItMeans: "server is speaking plain HTTP on a port we tried TLS on.\n" +
			"common with misconfigured reverse proxies.",
		Hint: "retry with http:// scheme on this port",
	},
	ClassHTTPProtocolErr: {
		WhatItMeans: "HTTP exchange failed at protocol level\n" +
			"(malformed response, unexpected close, etc.)",
		Hint: "check server logs; --verbose to see what was sent",
	},
	ClassHTTPClientError: {
		WhatItMeans: "HTTP 4xx — server returned a client-side error.\n" +
			"connection itself is healthy.",
		Hint: "this is application-layer, not a network problem",
	},
	ClassHTTPServerError: {
		WhatItMeans: "HTTP 5xx — server reported its own internal error.\n" +
			"connection itself is healthy.",
		Hint: "check server logs; not a network connectivity issue",
	},
	ClassUnknown: {
		WhatItMeans: "an error we couldn't classify.",
		Hint:        "see raw error message above",
	},
}

// WhatItMeans 返回 class 的人话解释。空 class 返回空串。
func WhatItMeans(c ErrorClass) string {
	if info, ok := classCatalog[c]; ok {
		return info.WhatItMeans
	}
	return ""
}

// HintForClass 返回 class 的修复建议文本。
func HintForClass(c ErrorClass) string {
	if info, ok := classCatalog[c]; ok {
		return info.Hint
	}
	return ""
}
