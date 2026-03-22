package httptiming

import (
	"context"
	"crypto/tls"
	"io"
	"net/http"
	"net/http/httptrace"
	"time"
)

type Result struct {
	URL              string
	StatusCode       int
	DNSLookup        time.Duration
	DNSServer        string // ConnectStart 回调中记录的远端地址（即 DNS 解析后连接的 IP）
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
		connectAddr               string
	)

	trace := &httptrace.ClientTrace{
		DNSStart:             func(_ httptrace.DNSStartInfo) { dnsStart = time.Now() },
		DNSDone:              func(_ httptrace.DNSDoneInfo) { dnsDone = time.Now() },
		ConnectStart:         func(_, addr string) { connectStart = time.Now(); connectAddr = addr },
		ConnectDone:          func(_, _ string, _ error) { connectDone = time.Now() },
		TLSHandshakeStart:    func() { tlsStart = time.Now() },
		TLSHandshakeDone:     func(_ tls.ConnectionState, _ error) { tlsDone = time.Now() },
		GotFirstResponseByte: func() { gotFirstByte = time.Now() },
	}

	req, err := http.NewRequestWithContext(httptrace.WithClientTrace(ctx, trace), http.MethodGet, url, nil)
	if err != nil {
		return Result{}, err
	}

	if transport == nil {
		transport = &http.Transport{
			DisableKeepAlives: true,
		}
	} else if tr, ok := transport.(*http.Transport); ok {
		tr.DisableKeepAlives = true
	}
	client := &http.Client{Transport: transport}

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

	r := Result{
		URL:        url,
		StatusCode: resp.StatusCode,
		DNSServer:  connectAddr,
		Total:      bodyDone.Sub(reqStart),
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
