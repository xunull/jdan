// Package wsx 实现 jdan net ws 的核心：WebSocket 握手探测 + 最小 RFC6455 帧收发。
//
// WebSocket 握手就是一个 HTTP Upgrade 请求，服务端回 101 + Sec-WebSocket-Accept
// （= base64(sha1(客户端 key + 固定 GUID))）。验这个 Accept 就能确认对面是真 WS
// 端点；握手后发一个 ping 帧收 pong，就证明数据真能通。纯 stdlib，0 新依赖。
package wsx

import (
	"bufio"
	"crypto/rand"
	"crypto/sha1"
	"encoding/base64"
	"encoding/json"
	"fmt"
	"io"
	"net"
	"net/http"
	"strings"
	"time"
)

// RFC6455 固定 GUID，用于算 Sec-WebSocket-Accept。
const wsMagic = "258EAFA5-E914-47DA-95CA-C5AB0DC85B11"

// 帧 opcode。
const (
	OpText   = 0x1
	OpBinary = 0x2
	OpClose  = 0x8
	OpPing   = 0x9
	OpPong   = 0xA
)

// AcceptKey 按 RFC6455 算 Sec-WebSocket-Accept。
func AcceptKey(clientKey string) string {
	h := sha1.New()
	h.Write([]byte(clientKey + wsMagic))
	return base64.StdEncoding.EncodeToString(h.Sum(nil))
}

// GenKey 生成 16 随机字节的 base64 客户端 key。
func GenKey() (string, error) {
	b := make([]byte, 16)
	if _, err := rand.Read(b); err != nil {
		return "", err
	}
	return base64.StdEncoding.EncodeToString(b), nil
}

// Frame 是一个解码后的帧（只处理 FIN 单帧，probe 够用）。
type Frame struct {
	Opcode  byte
	Payload []byte
}

// EncodeFrame 编码一个 FIN 帧。mask 为 4 字节时按客户端规则掩码（客户端发的帧必须
// 掩码）；nil/空时不掩码（服务端帧）。
func EncodeFrame(opcode byte, payload, mask []byte) []byte {
	doMask := len(mask) == 4
	buf := []byte{0x80 | opcode} // FIN + opcode
	n := len(payload)

	var lenByte byte
	switch {
	case n < 126:
		lenByte = byte(n)
	case n < 65536:
		lenByte = 126
	default:
		lenByte = 127
	}
	if doMask {
		lenByte |= 0x80
	}
	buf = append(buf, lenByte)

	switch {
	case n >= 126 && n < 65536:
		buf = append(buf, byte(n>>8), byte(n))
	case n >= 65536:
		for i := 7; i >= 0; i-- {
			buf = append(buf, byte(n>>(8*i)))
		}
	}

	if doMask {
		buf = append(buf, mask...)
		m := make([]byte, n)
		for i := 0; i < n; i++ {
			m[i] = payload[i] ^ mask[i%4]
		}
		buf = append(buf, m...)
	} else {
		buf = append(buf, payload...)
	}
	return buf
}

// DecodeFrame 从 r 读一个帧。
func DecodeFrame(r io.Reader) (Frame, error) {
	hdr := make([]byte, 2)
	if _, err := io.ReadFull(r, hdr); err != nil {
		return Frame{}, err
	}
	opcode := hdr[0] & 0x0f
	masked := hdr[1]&0x80 != 0
	n := int(hdr[1] & 0x7f)

	switch n {
	case 126:
		ext := make([]byte, 2)
		if _, err := io.ReadFull(r, ext); err != nil {
			return Frame{}, err
		}
		n = int(ext[0])<<8 | int(ext[1])
	case 127:
		ext := make([]byte, 8)
		if _, err := io.ReadFull(r, ext); err != nil {
			return Frame{}, err
		}
		n = 0
		for _, b := range ext {
			n = n<<8 | int(b)
		}
	}

	var maskKey [4]byte
	if masked {
		if _, err := io.ReadFull(r, maskKey[:]); err != nil {
			return Frame{}, err
		}
	}
	payload := make([]byte, n)
	if _, err := io.ReadFull(r, payload); err != nil {
		return Frame{}, err
	}
	if masked {
		for i := range payload {
			payload[i] ^= maskKey[i%4]
		}
	}
	return Frame{Opcode: opcode, Payload: payload}, nil
}

// Request 描述一次握手探测的输入。
type Request struct {
	Host        string      // Host header（host 或 host:port）
	Path        string      // 请求路径（含 query），空 → "/"
	Origin      string      // 可选 Origin
	Subprotocol string      // 可选 Sec-WebSocket-Protocol
	Header      http.Header // 额外请求头
	Ping        bool        // 握手后发 ping 验往返
}

// Result 是探测结果。
type Result struct {
	StatusCode     int           `json:"status_code"`
	Status         string        `json:"status"`
	Accepted       bool          `json:"accepted"`        // 101 且 Accept 校验通过
	AcceptMismatch bool          `json:"accept_mismatch"` // 101 但 Accept 不对（可疑）
	Subprotocol    string        `json:"subprotocol,omitempty"`
	Extensions     string        `json:"extensions,omitempty"`
	Server         string        `json:"server,omitempty"`
	HandshakeMS    float64       `json:"handshake_ms"`
	PingSent       bool          `json:"ping_sent"`
	PongOK         bool          `json:"pong_ok"`
	PongMS         float64       `json:"pong_ms,omitempty"`
	dur            time.Duration // 内部：握手耗时原值
}

// Probe 在已建立的 conn 上做一次 WebSocket 握手（+可选 ping/pong）。
// conn 应已完成 TCP/TLS 拨号。timeout>0 时给整个过程设一个 deadline。
func Probe(conn net.Conn, req Request, timeout time.Duration) (Result, error) {
	var res Result
	key, err := GenKey()
	if err != nil {
		return res, err
	}

	path := req.Path
	if path == "" {
		path = "/"
	}
	var b strings.Builder
	fmt.Fprintf(&b, "GET %s HTTP/1.1\r\n", path)
	fmt.Fprintf(&b, "Host: %s\r\n", req.Host)
	b.WriteString("Upgrade: websocket\r\n")
	b.WriteString("Connection: Upgrade\r\n")
	fmt.Fprintf(&b, "Sec-WebSocket-Key: %s\r\n", key)
	b.WriteString("Sec-WebSocket-Version: 13\r\n")
	if req.Origin != "" {
		fmt.Fprintf(&b, "Origin: %s\r\n", req.Origin)
	}
	if req.Subprotocol != "" {
		fmt.Fprintf(&b, "Sec-WebSocket-Protocol: %s\r\n", req.Subprotocol)
	}
	for k, vs := range req.Header {
		for _, v := range vs {
			fmt.Fprintf(&b, "%s: %s\r\n", k, v)
		}
	}
	b.WriteString("\r\n")

	if timeout > 0 {
		_ = conn.SetDeadline(time.Now().Add(timeout))
	}
	start := time.Now()
	if _, err := conn.Write([]byte(b.String())); err != nil {
		return res, err
	}

	br := bufio.NewReader(conn)
	httpReq, _ := http.NewRequest("GET", "http://"+req.Host+path, nil)
	resp, err := http.ReadResponse(br, httpReq)
	if err != nil {
		return res, err
	}
	res.dur = time.Since(start)
	res.HandshakeMS = float64(res.dur.Microseconds()) / 1000
	res.StatusCode = resp.StatusCode
	res.Status = resp.Status
	res.Server = resp.Header.Get("Server")
	res.Subprotocol = resp.Header.Get("Sec-WebSocket-Protocol")
	res.Extensions = resp.Header.Get("Sec-WebSocket-Extensions")
	if resp.StatusCode == http.StatusSwitchingProtocols {
		if resp.Header.Get("Sec-WebSocket-Accept") == AcceptKey(key) {
			res.Accepted = true
		} else {
			res.AcceptMismatch = true
		}
	}
	_ = resp.Body.Close()

	if !res.Accepted || !req.Ping {
		return res, nil
	}

	// 握手成功 + 要求 ping：发一个 masked ping，读到 pong 为止。
	res.PingSent = true
	mask := make([]byte, 4)
	if _, err := rand.Read(mask); err != nil {
		return res, err
	}
	pstart := time.Now()
	if _, err := conn.Write(EncodeFrame(OpPing, []byte("jdan"), mask)); err != nil {
		return res, nil // 握手已成功，写 ping 失败只影响 PongOK
	}
	for {
		f, err := DecodeFrame(br)
		if err != nil {
			return res, nil // 没等到 pong（超时/断开），PongOK 保持 false
		}
		switch f.Opcode {
		case OpPong:
			res.PongOK = true
			res.PongMS = float64(time.Since(pstart).Microseconds()) / 1000
			return res, nil
		case OpClose:
			return res, nil
		default:
			// 服务端可能先推 text/binary，忽略继续读
		}
	}
}

// Render 渲染成人类可读文本。url 仅用于回显。
func (r Result) Render(url string) string {
	var b strings.Builder
	mark := "✗"
	if r.Accepted {
		mark = "✓"
	}
	fmt.Fprintf(&b, "WebSocket 握手：%s %s  (握手 %.1fms)", mark, r.Status, r.HandshakeMS)
	if url != "" {
		fmt.Fprintf(&b, "  %s", url)
	}
	b.WriteString("\n")

	if r.AcceptMismatch {
		b.WriteString("  ⚠ 回了 101 但 Sec-WebSocket-Accept 不匹配（对面可能不是标准 WS 端点）\n")
	}
	if r.Subprotocol != "" {
		fmt.Fprintf(&b, "  子协议:   %s\n", r.Subprotocol)
	}
	if r.Extensions != "" {
		fmt.Fprintf(&b, "  扩展:     %s\n", r.Extensions)
	}
	if r.Server != "" {
		fmt.Fprintf(&b, "  Server:   %s\n", r.Server)
	}
	if r.PingSent {
		if r.PongOK {
			fmt.Fprintf(&b, "  Ping/Pong: ✓ pong %.1fms\n", r.PongMS)
		} else {
			b.WriteString("  Ping/Pong: ✗ 没收到 pong（服务端可能不自动回 pong，不影响握手判定）\n")
		}
	}
	return b.String()
}

// FormatJSON 输出结构化结果。
func (r Result) FormatJSON() (string, error) {
	data, err := json.MarshalIndent(r, "", "  ")
	if err != nil {
		return "", err
	}
	return string(data), nil
}
