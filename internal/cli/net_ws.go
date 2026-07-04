package cli

import (
	"crypto/tls"
	"fmt"
	"io"
	"net"
	"net/url"
	"os"
	"strings"
	"time"

	"github.com/spf13/cobra"

	"github.com/xunull/jdan/internal/wsx"
)

type netWSDeps struct {
	out  io.Writer
	dial func(u *url.URL, insecure bool, timeout time.Duration) (net.Conn, error) // 注入；nil → 真实拨号
}

func newNetWSCommand(deps netWSDeps) *cobra.Command {
	if deps.out == nil {
		deps.out = os.Stdout
	}
	if deps.dial == nil {
		deps.dial = realDialWS
	}
	cmd := &cobra.Command{
		Use:   "ws <url>",
		Short: "探测 WebSocket 端点（握手 + ping/pong 往返）",
		Long: `探测一个 WebSocket 端点：发 HTTP Upgrade 握手，验 101 + Sec-WebSocket-Accept
（确认对面是真 WS 端点），再发一个 ping 帧收 pong（证明数据真能通）。0 新依赖
（纯 stdlib 手搓握手 + 最小 RFC6455 帧）。

例：
  jdan net ws echo.websocket.org           # 无 scheme 自动补 wss://
  jdan net ws wss://host/path --json
  jdan net ws ws://localhost:8080/ws       # 明文 ws
  jdan net ws wss://host --origin https://app.example.com   # 按 Origin 校验的端点
  jdan net ws wss://host --subprotocol chat -H "Authorization: Bearer x"
  jdan net ws wss://host --no-ping         # 只握手，不发 ping/pong

退出码：0 握手成功 / 非0 失败（连不上/非101/Accept 不对/超时）——可当探活。
ping/pong 只是附加的连通性提示：没收到 pong 不影响握手判定（有些服务端不自动回 pong）。
有意不做：不做交互式 WS 客户端（那是 wscat/websocat）、不压测、不绕鉴权。`,
		Args:          cobra.ExactArgs(1),
		SilenceUsage:  true,
		SilenceErrors: true,
		RunE: func(cmd *cobra.Command, args []string) error {
			rawHeaders, _ := cmd.Flags().GetStringArray("header")
			origin, _ := cmd.Flags().GetString("origin")
			subproto, _ := cmd.Flags().GetString("subprotocol")
			noPing, _ := cmd.Flags().GetBool("no-ping")
			asJSON, _ := cmd.Flags().GetBool("json")
			insecure, _ := cmd.Flags().GetBool("insecure")
			timeout, _ := cmd.Flags().GetDuration("timeout")

			u, err := normalizeWSURL(args[0])
			if err != nil {
				return err
			}
			reqHeader, err := parseRequestHeaders(rawHeaders)
			if err != nil {
				return err
			}

			conn, err := deps.dial(u, insecure, timeout)
			if err != nil {
				return fmt.Errorf("连接失败：%w", err)
			}
			defer conn.Close()

			res, err := wsx.Probe(conn, wsx.Request{
				Host:        u.Host,
				Path:        u.RequestURI(),
				Origin:      origin,
				Subprotocol: subproto,
				Header:      reqHeader,
				Ping:        !noPing,
			}, timeout)
			if err != nil {
				return err
			}

			if asJSON {
				s, jerr := res.FormatJSON()
				if jerr != nil {
					return jerr
				}
				fmt.Fprintln(deps.out, s)
			} else {
				fmt.Fprint(deps.out, res.Render(u.String()))
			}

			if !res.Accepted {
				return fmt.Errorf("WebSocket 握手失败：%s", res.Status)
			}
			return nil
		},
	}
	cmd.Flags().StringArrayP("header", "H", nil, `加请求头（可重复），如 -H "Authorization: Bearer x"`)
	cmd.Flags().String("origin", "", "Origin 头（很多 WS 端点按 Origin 校验）")
	cmd.Flags().String("subprotocol", "", "Sec-WebSocket-Protocol 协商")
	cmd.Flags().Bool("no-ping", false, "只握手，不发 ping/pong 往返")
	cmd.Flags().Bool("json", false, "JSON 输出")
	cmd.Flags().BoolP("insecure", "k", false, "跳过 TLS 证书验证（wss）")
	cmd.Flags().Duration("timeout", 10*time.Second, "整体超时")
	return cmd
}

// normalizeWSURL 规整 URL：无 scheme 默认 wss://，http(s) 映射到 ws(s)。
func normalizeWSURL(raw string) (*url.URL, error) {
	if !strings.Contains(raw, "://") {
		raw = "wss://" + raw
	}
	u, err := url.Parse(raw)
	if err != nil {
		return nil, err
	}
	switch u.Scheme {
	case "http":
		u.Scheme = "ws"
	case "https":
		u.Scheme = "wss"
	case "ws", "wss":
		// ok
	default:
		return nil, fmt.Errorf("不支持的 scheme %q（用 ws/wss）", u.Scheme)
	}
	if u.Host == "" {
		return nil, fmt.Errorf("缺少主机名")
	}
	return u, nil
}

// realDialWS 真实拨号：wss 走 TLS，ws 走明文 TCP。默认端口 wss=443 / ws=80。
func realDialWS(u *url.URL, insecure bool, timeout time.Duration) (net.Conn, error) {
	host := u.Hostname()
	port := u.Port()
	secure := u.Scheme == "wss"
	if port == "" {
		if secure {
			port = "443"
		} else {
			port = "80"
		}
	}
	addr := net.JoinHostPort(host, port)
	d := net.Dialer{Timeout: timeout}
	if secure {
		return tls.DialWithDialer(&d, "tcp", addr, &tls.Config{ServerName: host, InsecureSkipVerify: insecure})
	}
	return d.Dial("tcp", addr)
}

func init() {
	netCmd.AddCommand(newNetWSCommand(netWSDeps{}))
}
