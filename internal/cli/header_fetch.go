package cli

import (
	"fmt"
	"net/http"
	"time"

	"github.com/xunull/jdan/internal/httphdr"
)

// fetchResponseHeader 抓 URL（跟随重定向）并取最终响应里某个头的全部值（如多条 Set-Cookie）。
// 供 csp / cookie 命令复用；client 为 nil 时用带超时的默认 client。
func fetchResponseHeader(client *http.Client, raw, headerName string) (vals []string, final string, err error) {
	if client == nil {
		client = &http.Client{Timeout: 15 * time.Second}
	}
	url := httphdr.EnsureScheme(raw)
	hops, err := httphdr.Fetch(client, url, http.MethodGet, nil, 10)
	if err != nil {
		return nil, "", fmt.Errorf("抓取失败：%w", err)
	}
	if len(hops) == 0 {
		return nil, url, fmt.Errorf("无响应")
	}
	last := hops[len(hops)-1]
	return last.Header.Values(headerName), last.URL, nil
}
