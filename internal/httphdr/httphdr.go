// Package httphdr 实现 jdan http headers 子命令的核心：拉一个 URL，逐跳展示
// 状态行 + 响应头 + 完整重定向链。0 新依赖（纯 stdlib net/http）。
//
// 手动跟重定向（CheckRedirect 设成 ErrUseLastResponse，自己循环），这样能逐跳
// 拿到每一跳的全部响应头——自动跟转做不到。
package httphdr

import (
	"encoding/json"
	"fmt"
	"net/http"
	"net/url"
	"sort"
	"strings"
)

// Hop 是重定向链上的一跳。
type Hop struct {
	URL        string
	Status     string // 如 "301 Moved Permanently"
	StatusCode int
	Header     http.Header
}

// EnsureScheme 给没有 scheme 的 URL 补上 https://。
func EnsureScheme(raw string) string {
	if strings.Contains(raw, "://") {
		return raw
	}
	return "https://" + raw
}

// Fetch 手动跟重定向，逐跳返回。method 默认应传 "GET"；reqHeader 是附加请求头。
// maxRedirects 是最多跟几跳（0 = 不跟）。出错时返回已成功的跳 + error。
//
// 只读响应头、不下载 body（拿到 Header 后即关闭 Body）。
func Fetch(client *http.Client, rawURL, method string, reqHeader http.Header, maxRedirects int) ([]Hop, error) {
	c := *client // 浅拷贝，避免改到调用方的 CheckRedirect
	c.CheckRedirect = func(*http.Request, []*http.Request) error {
		return http.ErrUseLastResponse
	}

	current := rawURL
	var hops []Hop
	for {
		req, err := http.NewRequest(method, current, nil)
		if err != nil {
			return hops, err
		}
		for k, vs := range reqHeader {
			for _, v := range vs {
				req.Header.Add(k, v)
			}
		}
		resp, err := c.Do(req)
		if err != nil {
			return hops, err
		}
		hop := Hop{URL: current, Status: resp.Status, StatusCode: resp.StatusCode, Header: resp.Header.Clone()}
		resp.Body.Close()
		hops = append(hops, hop)

		loc := resp.Header.Get("Location")
		if !isRedirect(resp.StatusCode) || loc == "" {
			break
		}
		if len(hops) > maxRedirects {
			break // 到上限就停，不无限跟
		}
		base, err := url.Parse(current)
		if err != nil {
			break
		}
		next, err := base.Parse(loc) // 相对 Location 用当前 URL 解析
		if err != nil {
			break
		}
		current = next.String()
	}
	return hops, nil
}

func isRedirect(code int) bool {
	return code >= 300 && code < 400
}

// FormatText 渲染重定向链。showAll=false 时重定向跳只显 Location、最终跳显全部头；
// showAll=true 时每一跳都显全部头。
func FormatText(hops []Hop, showAll bool) string {
	var b strings.Builder
	for i, h := range hops {
		if i > 0 {
			b.WriteString("→ ")
		}
		b.WriteString(h.Status + "\n")
		isLast := i == len(hops)-1
		if isLast || showAll {
			writeHeaders(&b, h.Header)
		} else if loc := h.Header.Get("Location"); loc != "" {
			fmt.Fprintf(&b, "  Location: %s\n", loc)
		}
	}
	return b.String()
}

func writeHeaders(b *strings.Builder, h http.Header) {
	keys := make([]string, 0, len(h))
	for k := range h {
		keys = append(keys, k)
	}
	sort.Strings(keys)
	for _, k := range keys {
		for _, v := range h[k] {
			fmt.Fprintf(b, "  %s: %s\n", k, v)
		}
	}
}

type hopJSON struct {
	URL        string              `json:"url"`
	StatusCode int                 `json:"status_code"`
	Status     string              `json:"status"`
	Location   string              `json:"location,omitempty"`
	Headers    map[string][]string `json:"headers"`
}

// FormatJSON 把跳渲染成 JSON 数组。空时输出 "[]"。
func FormatJSON(hops []Hop) (string, error) {
	out := make([]hopJSON, 0, len(hops))
	for _, h := range hops {
		out = append(out, hopJSON{
			URL:        h.URL,
			StatusCode: h.StatusCode,
			Status:     h.Status,
			Location:   h.Header.Get("Location"),
			Headers:    h.Header,
		})
	}
	data, err := json.MarshalIndent(out, "", "  ")
	if err != nil {
		return "", err
	}
	return string(data), nil
}
