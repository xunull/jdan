package cli

import (
	"bufio"
	"bytes"
	"fmt"
	"net"
	"net/http"
	"net/url"
	"testing"
	"time"

	"github.com/xunull/jdan/internal/wsx"
)

// fakeDial 返回一个 net.Pipe 的客户端端，另一端交给 serve 扮演 WS 服务器。
func fakeDial(serve func(srv net.Conn)) func(*url.URL, bool, time.Duration) (net.Conn, error) {
	return func(_ *url.URL, _ bool, _ time.Duration) (net.Conn, error) {
		cli, srv := net.Pipe()
		go serve(srv)
		return cli, nil
	}
}

// wsServer101 完整走一遍：读握手 → 回 101 + 正确 Accept → 收 ping 回 pong。
func wsServer101(srv net.Conn) {
	defer srv.Close()
	br := bufio.NewReader(srv)
	req, err := http.ReadRequest(br)
	if err != nil {
		return
	}
	resp := "HTTP/1.1 101 Switching Protocols\r\nUpgrade: websocket\r\nConnection: Upgrade\r\n" +
		"Sec-WebSocket-Accept: " + wsx.AcceptKey(req.Header.Get("Sec-WebSocket-Key")) + "\r\n\r\n"
	if _, err := srv.Write([]byte(resp)); err != nil {
		return
	}
	f, err := wsx.DecodeFrame(br)
	if err != nil {
		return
	}
	if f.Opcode == wsx.OpPing {
		srv.Write(wsx.EncodeFrame(wsx.OpPong, f.Payload, nil))
	}
}

func runNetWS(t *testing.T, serve func(net.Conn), args ...string) (string, error) {
	t.Helper()
	var out bytes.Buffer
	cmd := newNetWSCommand(netWSDeps{out: &out, dial: fakeDial(serve)})
	cmd.SetArgs(args)
	err := cmd.Execute()
	return out.String(), err
}

func TestNetWS_Success(t *testing.T) {
	out, err := runNetWS(t, wsServer101, "wss://x.test/ws")
	if err != nil {
		t.Fatalf("握手成功不该报错: %v\n%s", err, out)
	}
	if !bytes.Contains([]byte(out), []byte("✓")) || !bytes.Contains([]byte(out), []byte("101")) {
		t.Errorf("应显示成功握手:\n%s", out)
	}
	if !bytes.Contains([]byte(out), []byte("pong")) {
		t.Errorf("应显示 pong 往返:\n%s", out)
	}
}

func TestNetWS_JSON(t *testing.T) {
	out, err := runNetWS(t, wsServer101, "wss://x.test/ws", "--json")
	if err != nil {
		t.Fatal(err)
	}
	if !bytes.Contains([]byte(out), []byte(`"accepted": true`)) {
		t.Errorf("--json 应含 accepted:true:\n%s", out)
	}
}

func TestNetWS_Failure401(t *testing.T) {
	out, err := runNetWS(t, func(srv net.Conn) {
		defer srv.Close()
		br := bufio.NewReader(srv)
		http.ReadRequest(br)
		srv.Write([]byte("HTTP/1.1 401 Unauthorized\r\nContent-Length: 0\r\n\r\n"))
	}, "wss://x.test/ws")
	if err == nil {
		t.Error("401 应报错（非0 退出）")
	}
	if !bytes.Contains([]byte(out), []byte("✗")) {
		t.Errorf("报错前应打印失败报告:\n%s", out)
	}
}

func TestNetWS_DialError(t *testing.T) {
	var out bytes.Buffer
	cmd := newNetWSCommand(netWSDeps{
		out: &out,
		dial: func(*url.URL, bool, time.Duration) (net.Conn, error) {
			return nil, fmt.Errorf("connection refused")
		},
	})
	cmd.SetArgs([]string{"wss://x.test"})
	if err := cmd.Execute(); err == nil {
		t.Error("拨号失败应报错")
	}
}

func TestNormalizeWSURL(t *testing.T) {
	cases := map[string]string{
		"host":              "wss://host",  // 无 scheme → wss
		"http://host/x":     "ws://host/x", // http → ws
		"https://host":      "wss://host",  // https → wss
		"ws://host:8080/ws": "ws://host:8080/ws",
	}
	for in, want := range cases {
		u, err := normalizeWSURL(in)
		if err != nil {
			t.Errorf("%q: %v", in, err)
			continue
		}
		if u.String() != want {
			t.Errorf("normalizeWSURL(%q) = %q, want %q", in, u.String(), want)
		}
	}
	if _, err := normalizeWSURL("ftp://host"); err == nil {
		t.Error("非 ws/http scheme 应报错")
	}
}
