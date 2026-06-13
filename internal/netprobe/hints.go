package netprobe

import "strings"

// 失败信息 → 人类可读的修复建议。
// 关键词匹配，case-insensitive。匹配到第一个就停。
//
// 设计意图：用户撞墙时，30 秒内能看到"接下来该查什么"。

type errHint struct {
	match string // 错误信息里的关键词（小写）
	hint  string // 多行 hint，已经包含换行
}

var resolveHints = []errHint{
	{"no such host", "hostname does not exist; check spelling and DNS server reachability"},
	{"i/o timeout", "DNS server unreachable or slow (timeout); try --resolver 1.1.1.1"},
	{"server misbehaving", "DNS server returned malformed response; try --resolver 8.8.8.8"},
	{"connection refused", "DNS server refused query; --resolver may be unreachable"},
}

var tcpHints = []errHint{
	{"connection refused",
		"likely causes:\n" +
			"  • target host not listening on this port (check: lsof -i :PORT on target)\n" +
			"  • OS firewall blocking inbound (macOS App Firewall, ufw, Windows Defender)\n" +
			"  • if you're on a different LAN segment, check routing\n" +
			"  ↳ run `jdan net selfcheck :PORT` on the target host to investigate"},
	{"i/o timeout",
		"likely causes:\n" +
			"  • host firewall silently dropping packets\n" +
			"  • routing issue (no return path)\n" +
			"  • target host is down or behind NAT without port-forwarding"},
	{"no route to host",
		"no path to target IP. likely you're on a different LAN segment;\n" +
			"check `route get <ip>` or VPN status"},
	{"network is unreachable",
		"local network down or no default route. check `ifconfig` / `ip addr`"},
	{"host is down",
		"host is unreachable. check power state, WiFi, or LAN cable"},
}

var tlsHints = []errHint{
	{"x509: certificate signed by unknown authority",
		"self-signed or untrusted CA. add cert to system trust store, or use --insecure to skip verification"},
	{"x509: certificate has expired",
		"server's TLS certificate has expired. fix on server side or use --insecure to bypass"},
	{"x509: certificate is not valid for",
		"hostname mismatch (SAN doesn't include the host you requested).\n" +
			"  if you're probing by IP, target server SNI is different"},
	{"tls: handshake failure",
		"TLS handshake rejected; server may require client cert, or protocol mismatch (try --insecure to isolate)"},
	{"first record does not look like a tls handshake",
		"server is plain HTTP, not HTTPS. retry with http:// scheme"},
	{"unexpected eof",
		"connection cut during TLS handshake. possible middlebox / firewall / loadbalancer aborting"},
	{"protocol version",
		"TLS version mismatch (server too old or too new)"},
}

var httpHints = []errHint{
	{"connection reset",
		"server forcibly closed the connection mid-request. check server logs"},
	{"timeout",
		"server accepted TCP but didn't respond in time. check if server is stuck"},
	{"eof", "server closed before sending response. check server logs"},
}

var statusHints = map[int]string{
	400: "bad request: malformed URL or headers",
	401: "unauthorized: need credentials (`--auth user:pass` if jdan supports)",
	403: "forbidden: IP allowlist / WAF / hotlinking protection",
	404: "not found: path doesn't exist on server",
	405: "method not allowed: server rejects HEAD; try --method GET",
	408: "request timeout: server gave up waiting for our request",
	500: "internal server error: bug on server side",
	502: "bad gateway: upstream service down",
	503: "service unavailable: server overloaded or in maintenance",
	504: "gateway timeout: upstream took too long",
}

func hintForResolveError(err error) string {
	return lookup(strings.ToLower(err.Error()), resolveHints)
}

func hintForTCPError(msg string) string {
	return lookup(strings.ToLower(msg), tcpHints)
}

func hintForTLSError(msg string) string {
	return lookup(strings.ToLower(msg), tlsHints)
}

func hintForHTTPError(msg string) string {
	return lookup(strings.ToLower(msg), httpHints)
}

func hintForHTTPStatus(code int) string {
	if h, ok := statusHints[code]; ok {
		return h
	}
	switch {
	case code >= 500:
		return "server-side error; not a network connectivity issue"
	case code >= 400:
		return "client-side error; not a network connectivity issue"
	}
	return ""
}

func lookup(msgLower string, hints []errHint) string {
	for _, h := range hints {
		if strings.Contains(msgLower, h.match) {
			return h.hint
		}
	}
	return ""
}
