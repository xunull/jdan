package wsx

import (
	"bufio"
	"bytes"
	"encoding/base64"
	"encoding/json"
	"net"
	"net/http"
	"testing"
	"time"
)

func TestAcceptKey_RFCExample(t *testing.T) {
	// RFC6455 §1.3 的示例向量。
	if got := AcceptKey("dGhlIHNhbXBsZSBub25jZQ=="); got != "s3pPLMBiTxaQ9kYGzzhZRbK+xOo=" {
		t.Errorf("AcceptKey = %q, want s3pPLMBiTxaQ9kYGzzhZRbK+xOo=", got)
	}
}

func TestGenKey_16Bytes(t *testing.T) {
	k, err := GenKey()
	if err != nil {
		t.Fatal(err)
	}
	raw, err := base64.StdEncoding.DecodeString(k)
	if err != nil {
		t.Fatalf("key 不是合法 base64: %q", k)
	}
	if len(raw) != 16 {
		t.Errorf("key 应为 16 字节，得 %d", len(raw))
	}
}

func TestFrame_RoundTrip(t *testing.T) {
	payloads := [][]byte{
		{},
		[]byte("hi"),
		bytes.Repeat([]byte("x"), 200),   // 触发 2 字节扩展长度
		bytes.Repeat([]byte("y"), 70000), // 触发 8 字节扩展长度
	}
	for _, p := range payloads {
		// 客户端帧：掩码
		enc := EncodeFrame(OpText, p, []byte{1, 2, 3, 4})
		f, err := DecodeFrame(bytes.NewReader(enc))
		if err != nil {
			t.Fatalf("masked len=%d: %v", len(p), err)
		}
		if f.Opcode != OpText || !bytes.Equal(f.Payload, p) {
			t.Errorf("masked round-trip 失败 len=%d", len(p))
		}
		// 服务端帧：不掩码
		enc2 := EncodeFrame(OpPong, p, nil)
		f2, err := DecodeFrame(bytes.NewReader(enc2))
		if err != nil {
			t.Fatalf("unmasked len=%d: %v", len(p), err)
		}
		if f2.Opcode != OpPong || !bytes.Equal(f2.Payload, p) {
			t.Errorf("unmasked round-trip 失败 len=%d", len(p))
		}
	}
}

func TestFrame_ClientFrameIsMasked(t *testing.T) {
	// 客户端帧的 MASK 位必须置 1（否则真服务器会拒）。
	enc := EncodeFrame(OpPing, []byte("jdan"), []byte{9, 9, 9, 9})
	if enc[1]&0x80 == 0 {
		t.Error("客户端帧应带掩码位")
	}
	enc2 := EncodeFrame(OpPong, []byte("jdan"), nil)
	if enc2[1]&0x80 != 0 {
		t.Error("服务端帧不应带掩码位")
	}
}

// --- Probe（用 net.Pipe 起假 WS 服务端）---

// serveWS 在 srv 上扮演 WS 服务端：读握手 → 回 101（可选带 subproto）→ 收 ping 回 pong。
func serveWS(srv net.Conn, subproto string, autoPong bool) {
	go func() {
		defer srv.Close()
		br := bufio.NewReader(srv)
		req, err := http.ReadRequest(br)
		if err != nil {
			return
		}
		extra := ""
		if subproto != "" {
			extra = "Sec-WebSocket-Protocol: " + subproto + "\r\n"
		}
		resp := "HTTP/1.1 101 Switching Protocols\r\nUpgrade: websocket\r\nConnection: Upgrade\r\n" +
			"Sec-WebSocket-Accept: " + AcceptKey(req.Header.Get("Sec-WebSocket-Key")) + "\r\n" + extra + "\r\n"
		if _, err := srv.Write([]byte(resp)); err != nil {
			return
		}
		if !autoPong {
			return
		}
		f, err := DecodeFrame(br)
		if err != nil {
			return
		}
		if f.Opcode == OpPing {
			srv.Write(EncodeFrame(OpPong, f.Payload, nil)) // 服务端帧不掩码
		}
	}()
}

func TestProbe_SuccessWithPong(t *testing.T) {
	cli, srv := net.Pipe()
	serveWS(srv, "chat", true)
	res, err := Probe(cli, Request{Host: "x.test", Path: "/", Subprotocol: "chat", Ping: true}, 2*time.Second)
	cli.Close()
	if err != nil {
		t.Fatal(err)
	}
	if !res.Accepted {
		t.Errorf("应握手成功: %+v", res)
	}
	if res.Subprotocol != "chat" {
		t.Errorf("子协议 = %q, want chat", res.Subprotocol)
	}
	if !res.PingSent || !res.PongOK {
		t.Errorf("应收到 pong: %+v", res)
	}
}

func TestProbe_NoPingFlag(t *testing.T) {
	cli, srv := net.Pipe()
	serveWS(srv, "", false)
	res, err := Probe(cli, Request{Host: "x.test", Ping: false}, 2*time.Second)
	cli.Close()
	if err != nil {
		t.Fatal(err)
	}
	if !res.Accepted {
		t.Errorf("应握手成功: %+v", res)
	}
	if res.PingSent {
		t.Error("--no-ping 时不应发 ping")
	}
}

func TestProbe_Non101(t *testing.T) {
	cli, srv := net.Pipe()
	go func() {
		defer srv.Close()
		br := bufio.NewReader(srv)
		http.ReadRequest(br)
		srv.Write([]byte("HTTP/1.1 401 Unauthorized\r\nContent-Length: 0\r\n\r\n"))
	}()
	res, err := Probe(cli, Request{Host: "x.test", Ping: true}, 2*time.Second)
	cli.Close()
	if err != nil {
		t.Fatal(err)
	}
	if res.Accepted {
		t.Error("401 不应算握手成功")
	}
	if res.StatusCode != 401 {
		t.Errorf("StatusCode = %d, want 401", res.StatusCode)
	}
}

func TestProbe_AcceptMismatch(t *testing.T) {
	cli, srv := net.Pipe()
	go func() {
		defer srv.Close()
		br := bufio.NewReader(srv)
		http.ReadRequest(br)
		// 回 101 但 Accept 是错的
		srv.Write([]byte("HTTP/1.1 101 Switching Protocols\r\nUpgrade: websocket\r\nConnection: Upgrade\r\nSec-WebSocket-Accept: WRONGWRONGWRONG=\r\n\r\n"))
	}()
	res, err := Probe(cli, Request{Host: "x.test", Ping: true}, 2*time.Second)
	cli.Close()
	if err != nil {
		t.Fatal(err)
	}
	if res.Accepted {
		t.Error("Accept 不匹配不应算成功")
	}
	if !res.AcceptMismatch {
		t.Errorf("应标记 AcceptMismatch: %+v", res)
	}
}

func TestFormatJSON(t *testing.T) {
	r := Result{StatusCode: 101, Status: "101 Switching Protocols", Accepted: true, HandshakeMS: 12.3, PingSent: true, PongOK: true, PongMS: 4.1}
	s, err := r.FormatJSON()
	if err != nil {
		t.Fatal(err)
	}
	var v map[string]any
	if err := json.Unmarshal([]byte(s), &v); err != nil {
		t.Fatalf("非法 json:\n%s", s)
	}
	if v["accepted"] != true {
		t.Errorf("accepted 应为 true: %v", v)
	}
}
