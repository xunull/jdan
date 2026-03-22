package httptiming

import (
	"context"
	"crypto/tls"
	"io"
	"net"
	"net/http"
	"net/http/httptrace"
	"strings"
	"time"
)

type Result struct {
	URL              string
	StatusCode       int
	DNSLookup        time.Duration
	ResolvedAddrs    string // DNS 解析到的目标 IP（可能多个，逗号分隔）
	DNSServer        string // 本次 DNS 查询使用的 DNS 服务器地址
	TCPConnect       time.Duration
	TLSHandshake     time.Duration
	ServerProcessing time.Duration
	ContentTransfer  time.Duration
	Total            time.Duration
}

// Measure performs a single HTTP GET and records per-phase timing via httptrace.
func Measure(ctx context.Context, url string, transport http.RoundTripper) (Result, error) {
	var (
		dnsStart, dnsDone         time.Time
		connectStart, connectDone time.Time
		tlsStart, tlsDone         time.Time
		gotFirstByte              time.Time
		reqStart                  time.Time
		resolvedAddrs             []net.IPAddr
		dnsServerUsed             string
	)

	trace := &httptrace.ClientTrace{
		DNSStart: func(_ httptrace.DNSStartInfo) { dnsStart = time.Now() },
		DNSDone: func(info httptrace.DNSDoneInfo) {
			dnsDone = time.Now()
			resolvedAddrs = info.Addrs
		},
		ConnectStart:         func(_, _ string) { connectStart = time.Now() },
		ConnectDone:          func(_, _ string, _ error) { connectDone = time.Now() },
		TLSHandshakeStart:    func() { tlsStart = time.Now() },
		TLSHandshakeDone:     func(_ tls.ConnectionState, _ error) { tlsDone = time.Now() },
		GotFirstResponseByte: func() { gotFirstByte = time.Now() },
	}

	req, err := http.NewRequestWithContext(httptrace.WithClientTrace(ctx, trace), http.MethodGet, url, nil)
	if err != nil {
		return Result{}, err
	}

	// Intercept the DNS server address via a custom Resolver.Dial
	resolver := &net.Resolver{
		PreferGo: true,
		Dial: func(ctx context.Context, network, address string) (net.Conn, error) {
			dnsServerUsed = address
			return (&net.Dialer{}).DialContext(ctx, network, address)
		},
	}

	var tlsCfg *tls.Config
	if t, ok := transport.(*http.Transport); ok && t != nil && t.TLSClientConfig != nil {
		tlsCfg = t.TLSClientConfig
	}

	tr := &http.Transport{
		DisableKeepAlives: true,
		DialContext:       (&net.Dialer{Resolver: resolver}).DialContext,
		TLSClientConfig:   tlsCfg,
	}
	client := &http.Client{Transport: tr}

	reqStart = time.Now()
	resp, err := client.Do(req)
	if err != nil {
		return Result{}, err
	}
	defer resp.Body.Close()
	_, _ = io.Copy(io.Discard, resp.Body)
	bodyDone := time.Now()

	// server processing baseline: after TLS (HTTPS) or after TCP connect (HTTP)
	serverBase := connectDone
	if !tlsDone.IsZero() {
		serverBase = tlsDone
	}

	var addrStrs []string
	for _, a := range resolvedAddrs {
		addrStrs = append(addrStrs, a.IP.String())
	}

	r := Result{
		URL:           url,
		StatusCode:    resp.StatusCode,
		ResolvedAddrs: strings.Join(addrStrs, ", "),
		DNSServer:     dnsServerUsed,
		Total:         bodyDone.Sub(reqStart),
	}
	if !dnsStart.IsZero() && !dnsDone.IsZero() {
		r.DNSLookup = dnsDone.Sub(dnsStart)
	}
	if !connectStart.IsZero() && !connectDone.IsZero() {
		r.TCPConnect = connectDone.Sub(connectStart)
	}
	if !tlsStart.IsZero() && !tlsDone.IsZero() {
		r.TLSHandshake = tlsDone.Sub(tlsStart)
	}
	if !gotFirstByte.IsZero() && !serverBase.IsZero() {
		r.ServerProcessing = gotFirstByte.Sub(serverBase)
	}
	if !gotFirstByte.IsZero() {
		r.ContentTransfer = bodyDone.Sub(gotFirstByte)
	}
	return r, nil
}
