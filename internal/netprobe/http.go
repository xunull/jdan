package netprobe

import (
	"context"
	"crypto/tls"
	"fmt"
	"net"
	"net/http"
	"strings"
	"time"
)

// HTTPDetail 是 HTTP 阶段的特有字段。
type HTTPDetail struct {
	Method        string            `json:"method"`
	Status        int               `json:"status"`
	Proto         string            `json:"proto"`
	Server        string            `json:"server,omitempty"`
	ContentType   string            `json:"content_type,omitempty"`
	ContentLength int64             `json:"content_length"`
	Headers       map[string]string `json:"headers,omitempty"` // 仅 verbose 模式
	FellBackToGET bool              `json:"fell_back_to_get,omitempty"`
}

func runHTTP(ctx context.Context, t *Target, ip net.IP, tlsState *tls.ConnectionState, opts Options) *StageResult {
	stageStart := time.Now()
	r := &StageResult{Stage: StageHTTP}

	method := strings.ToUpper(opts.Method)
	if method == "" {
		method = "HEAD"
	}

	resp, err := doRequest(ctx, t, ip, tlsState, method, opts)
	r.Duration = time.Since(stageStart)
	if err != nil {
		r.Success = false
		r.Class = ClassifyHTTPError(err)
		r.Err = err.Error()
		r.Explanation = WhatItMeans(r.Class)
		r.Hint = HintForClass(r.Class)
		r.HTTP = &HTTPDetail{Method: method}
		return r
	}
	defer resp.Body.Close()

	// 405 → fallback 到 GET 一次（HEAD 不被支持的情况）
	fellBack := false
	if resp.StatusCode == http.StatusMethodNotAllowed && method == "HEAD" {
		_ = resp.Body.Close()
		resp2, err2 := doRequest(ctx, t, ip, tlsState, "GET", opts)
		if err2 == nil {
			resp = resp2
			defer resp.Body.Close()
			method = "GET"
			fellBack = true
			r.Duration = time.Since(stageStart)
		}
	}

	d := &HTTPDetail{
		Method:        method,
		Status:        resp.StatusCode,
		Proto:         resp.Proto,
		Server:        resp.Header.Get("Server"),
		ContentType:   resp.Header.Get("Content-Type"),
		ContentLength: resp.ContentLength,
		FellBackToGET: fellBack,
	}
	if opts.Verbose {
		d.Headers = make(map[string]string, len(resp.Header))
		for k, v := range resp.Header {
			d.Headers[k] = strings.Join(v, ", ")
		}
	}
	r.HTTP = d
	r.Detail = fmt.Sprintf("%s %s, %d %s", method, resp.Proto, resp.StatusCode, http.StatusText(resp.StatusCode))

	// 视为成功的范围：所有 1xx-5xx 响应都算 "HTTP 拿到了"；
	// 业务层 4xx/5xx 不是网络问题，但分类 + hint 一下让用户知道
	r.Success = true
	if cls := ClassifyHTTPStatus(resp.StatusCode); cls != ClassNone {
		r.Class = cls
		r.Explanation = WhatItMeans(cls)
		r.Hint = HintForClass(cls)
	}
	return r
}

// doRequest 构造一个手工 dial 的 http.Client：复用 TLS 阶段已经验证过的目标 IP，
// 不让 Go 重新做 DNS（探查的本意是测特定 IP）。
func doRequest(ctx context.Context, t *Target, ip net.IP, tlsState *tls.ConnectionState, method string, opts Options) (*http.Response, error) {
	url := fmt.Sprintf("%s://%s%s", t.Scheme, net.JoinHostPort(t.Host, fmt.Sprintf("%d", t.Port)), t.Path)
	req, err := http.NewRequestWithContext(ctx, method, url, nil)
	if err != nil {
		return nil, err
	}
	req.Header.Set("User-Agent", "jdan-net-probe/1.0")
	req.Host = t.Host // 显式设 Host header

	target := dialAddr(ip, t.Port)
	dialer := &net.Dialer{Timeout: opts.Timeout}

	transport := &http.Transport{
		// 强制把所有请求 dial 到 target IP，绕过 DNS
		DialContext: func(ctx context.Context, network, addr string) (net.Conn, error) {
			return dialer.DialContext(ctx, network, target)
		},
		TLSClientConfig: &tls.Config{
			ServerName:         t.Host,
			InsecureSkipVerify: opts.Insecure,
			NextProtos:         []string{"h2", "http/1.1"},
		},
		ForceAttemptHTTP2:     true,
		MaxIdleConns:          1,
		IdleConnTimeout:       90 * time.Second,
		TLSHandshakeTimeout:   opts.Timeout,
		ExpectContinueTimeout: 1 * time.Second,
		ResponseHeaderTimeout: opts.Timeout,
	}

	client := &http.Client{
		Transport: transport,
		Timeout:   opts.Timeout * 2, // request 整体上限
		CheckRedirect: func(req *http.Request, via []*http.Request) error {
			// 不自动 follow redirect — 探查阶段想看到 30x
			return http.ErrUseLastResponse
		},
	}
	return client.Do(req)
}
