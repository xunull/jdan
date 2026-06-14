package sslscan

import (
	"context"
	"crypto/tls"
	"fmt"
	"net/http"
	"strconv"
	"strings"
	"time"
)

// HSTSSection 是 HTTP Strict Transport Security 探测结果。
type HSTSSection struct {
	Present           bool   `json:"present"`
	MaxAge            int    `json:"max_age_seconds"`
	IncludeSubDomains bool   `json:"include_subdomains"`
	Preload           bool   `json:"preload"`
	RawHeader         string `json:"raw_header,omitempty"`
	Err               string `json:"error,omitempty"`
}

// scanHSTS 发一个 HTTPS GET / 抓 Strict-Transport-Security 头。失败不影响
// scan 整体——返回的 section 标 Err，grade 把它视作"无 HSTS"。
func scanHSTS(ctx context.Context, opts Options) *HSTSSection {
	s := &HSTSSection{}
	url := fmt.Sprintf("https://%s/", opts.Host+":"+strconv.Itoa(opts.Port))
	cctx, cancel := context.WithTimeout(ctx, 5*time.Second)
	defer cancel()

	req, err := http.NewRequestWithContext(cctx, "GET", url, nil)
	if err != nil {
		s.Err = err.Error()
		return s
	}
	req.Host = opts.SNI

	client := &http.Client{
		Timeout: 5 * time.Second,
		Transport: &http.Transport{
			TLSClientConfig: &tls.Config{
				ServerName:         opts.SNI,
				InsecureSkipVerify: true,
			},
		},
		// 不 follow redirect——HSTS 在原响应里就有
		CheckRedirect: func(req *http.Request, via []*http.Request) error {
			return http.ErrUseLastResponse
		},
	}
	resp, err := client.Do(req)
	if err != nil {
		s.Err = err.Error()
		return s
	}
	defer resp.Body.Close()

	header := resp.Header.Get("Strict-Transport-Security")
	if header == "" {
		return s
	}
	s.Present = true
	s.RawHeader = header
	parseHSTS(header, s)
	return s
}

// parseHSTS 解析 STS header 像
//
//	max-age=31536000; includeSubDomains; preload
//
// 字段无序，分号分隔，max-age 是必需的。
func parseHSTS(header string, s *HSTSSection) {
	for _, part := range strings.Split(header, ";") {
		p := strings.TrimSpace(part)
		switch {
		case strings.HasPrefix(strings.ToLower(p), "max-age"):
			eq := strings.Index(p, "=")
			if eq > 0 {
				n, err := strconv.Atoi(strings.Trim(strings.TrimSpace(p[eq+1:]), `"`))
				if err == nil {
					s.MaxAge = n
				}
			}
		case strings.EqualFold(p, "includeSubDomains"):
			s.IncludeSubDomains = true
		case strings.EqualFold(p, "preload"):
			s.Preload = true
		}
	}
}

// Strength 返回 HSTS 强度级别给 grade 用：
//   - "none"      未声明
//   - "weak"      max-age < 1 year
//   - "good"      >= 1 year
//   - "preload"   >= 1 year + includeSubDomains + preload
func (h *HSTSSection) Strength() string {
	if h == nil || !h.Present {
		return "none"
	}
	const oneYear = 31536000
	if h.MaxAge < oneYear {
		return "weak"
	}
	if h.IncludeSubDomains && h.Preload {
		return "preload"
	}
	return "good"
}
