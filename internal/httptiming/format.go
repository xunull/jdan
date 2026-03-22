package httptiming

import (
	"encoding/json"
	"fmt"
	"strings"
	"time"
)

type jsonResult struct {
	URL              string  `json:"url"`
	StatusCode       int     `json:"status_code"`
	DNSLookupMs      float64 `json:"dns_lookup_ms"`
	TCPConnectMs     float64 `json:"tcp_connect_ms"`
	TLSHandshakeMs   float64 `json:"tls_handshake_ms"`
	ServerProcessMs  float64 `json:"server_processing_ms"`
	ContentTransferMs float64 `json:"content_transfer_ms"`
	TotalMs          float64 `json:"total_ms"`
}

type jsonOutput struct {
	Results []jsonResult `json:"results"`
	Average *jsonResult  `json:"average,omitempty"`
}

func ms(d time.Duration) float64 {
	return float64(d.Microseconds()) / 1000.0
}

func fmtMs(d time.Duration) string {
	return fmt.Sprintf("%.2fms", ms(d))
}

func formatSingle(b *strings.Builder, r Result, label string) {
	if label != "" {
		fmt.Fprintf(b, "--- %s ---\n", label)
	}
	fmt.Fprintf(b, "URL:            %s\n", r.URL)
	fmt.Fprintf(b, "状态码:         %d\n", r.StatusCode)
	if r.DNSServer != "" {
		fmt.Fprintf(b, "DNS 查询:       %s (%s)\n", fmtMs(r.DNSLookup), r.DNSServer)
	} else {
		fmt.Fprintf(b, "DNS 查询:       %s\n", fmtMs(r.DNSLookup))
	}
	fmt.Fprintf(b, "TCP 连接:       %s\n", fmtMs(r.TCPConnect))
	fmt.Fprintf(b, "TLS 握手:       %s\n", fmtMs(r.TLSHandshake))
	fmt.Fprintf(b, "服务端处理:     %s\n", fmtMs(r.ServerProcessing))
	fmt.Fprintf(b, "内容传输:       %s\n", fmtMs(r.ContentTransfer))
	fmt.Fprintf(b, "总耗时:         %s\n", fmtMs(r.Total))
}

func average(results []Result) Result {
	n := len(results)
	if n == 0 {
		return Result{}
	}
	var avg Result
	avg.URL = results[0].URL
	for _, r := range results {
		avg.DNSLookup += r.DNSLookup
		avg.TCPConnect += r.TCPConnect
		avg.TLSHandshake += r.TLSHandshake
		avg.ServerProcessing += r.ServerProcessing
		avg.ContentTransfer += r.ContentTransfer
		avg.Total += r.Total
	}
	avg.DNSLookup /= time.Duration(n)
	avg.TCPConnect /= time.Duration(n)
	avg.TLSHandshake /= time.Duration(n)
	avg.ServerProcessing /= time.Duration(n)
	avg.ContentTransfer /= time.Duration(n)
	avg.Total /= time.Duration(n)
	return avg
}

// FormatText returns human-readable output. Multiple results get numbered labels and an average.
func FormatText(results []Result) string {
	var b strings.Builder
	if len(results) == 1 {
		formatSingle(&b, results[0], "")
	} else {
		for i, r := range results {
			formatSingle(&b, r, fmt.Sprintf("#%d", i+1))
			b.WriteString("\n")
		}
		formatSingle(&b, average(results), "平均值")
	}
	return b.String()
}

func toJSON(r Result) jsonResult {
	return jsonResult{
		URL:               r.URL,
		StatusCode:        r.StatusCode,
		DNSLookupMs:       ms(r.DNSLookup),
		TCPConnectMs:      ms(r.TCPConnect),
		TLSHandshakeMs:    ms(r.TLSHandshake),
		ServerProcessMs:   ms(r.ServerProcessing),
		ContentTransferMs: ms(r.ContentTransfer),
		TotalMs:           ms(r.Total),
	}
}

// FormatJSON returns JSON output. Single result: one object. Multiple: array + average.
func FormatJSON(results []Result) (string, error) {
	if len(results) == 1 {
		data, err := json.MarshalIndent(toJSON(results[0]), "", "  ")
		return string(data), err
	}
	out := jsonOutput{}
	for _, r := range results {
		out.Results = append(out.Results, toJSON(r))
	}
	avg := toJSON(average(results))
	out.Average = &avg
	data, err := json.MarshalIndent(out, "", "  ")
	return string(data), err
}
