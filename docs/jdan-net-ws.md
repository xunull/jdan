# jdan net ws

探测一个 **WebSocket 端点**：发握手、验它是不是真 WS 服务器、再发一个 ping 收 pong 确认数据真能通。**0 新依赖**（纯 stdlib 手搓握手 + 最小 RFC6455 帧）。

跟 `jdan net probe`（DNS/TCP/TLS/HTTP 分阶段）互补：那个探到 HTTP 层，这个再往上探一层 WebSocket 升级。

## 原理

WebSocket 握手就是一个 **HTTP Upgrade** 请求：

```
GET /path HTTP/1.1
Host: …
Upgrade: websocket
Connection: Upgrade
Sec-WebSocket-Key: <16 随机字节的 base64>
Sec-WebSocket-Version: 13
```

真 WS 服务器回 `101 Switching Protocols`，并带 `Sec-WebSocket-Accept = base64(sha1(你的 Key + 固定 GUID))`。

`net ws` 干三件事：

1. **验握手**：拿到 101 后,自己按同样公式算一遍 Accept,比对服务端回的值 —— 对上才算真 WS 端点（防「随便回个 101」的假阳性）。
2. **验往返**：握手后发一个**掩码 ping 帧**（RFC6455 要求客户端帧必须掩码），读到 **pong** 就证明数据真能双向流动,不只是握手。
3. **报协商结果**：握手耗时、协商到的 `Sec-WebSocket-Protocol`（子协议）/ `Sec-WebSocket-Extensions` / `Server`。

全程 stdlib：`crypto/tls` 拨号、`crypto/sha1`+`encoding/base64` 算 Accept、手写帧收发。

## 用法

```bash
jdan net ws echo.websocket.org              # 无 scheme 自动补 wss://
jdan net ws wss://host/path --json
jdan net ws ws://localhost:8080/ws          # 明文 ws
jdan net ws wss://host --origin https://app.example.com    # 按 Origin 校验的端点
jdan net ws wss://host --subprotocol chat -H "Authorization: Bearer x"
jdan net ws wss://host --no-ping            # 只握手，不发 ping/pong
jdan net ws wss://host -k                   # 跳过 TLS 证书验证
```

输出示例：

```
WebSocket 握手：✓ 101 Switching Protocols  (握手 271.0ms)  wss://ws.postman-echo.com/raw
  Server:   nginx
  Ping/Pong: ✓ pong 369.8ms
```

失败时（非 WS 端点）：

```
WebSocket 握手：✗ 200 OK  (握手 515.0ms)  wss://example.com
  Server:   cloudflare
```

### Flags

| flag | 默认 | 说明 |
|------|------|------|
| `--origin` | — | Origin 头（很多 WS 端点按 Origin 校验） |
| `--subprotocol` | — | Sec-WebSocket-Protocol 协商 |
| `--header` `-H` | — | 加请求头(可重复),如 `Authorization` |
| `--no-ping` | false | 只握手,不发 ping/pong 往返 |
| `--json` | false | JSON 输出 |
| `--insecure` `-k` | false | 跳过 TLS 证书验证(wss) |
| `--timeout` | 10s | 整体超时 |

### 退出码

`0` 握手成功 / 非 `0` 失败(连不上 / 非 101 / Accept 不匹配 / 超时)——是/否判定,可当 WS 探活放进 CI/监控,跟 `net cdn`、`git secrets` 一路。

> **ping/pong 是附加提示,不影响退出码**：没收到 pong 时报告里标出来,但只要握手成功仍退出 0。有些服务端不自动回 pong(或要先协商子协议),不该因此判 WS 端点「挂了」。

scheme 规整：无 scheme 默认 `wss://`(安全默认);`http://`→`ws://`、`https://`→`wss://`;默认端口 wss=443 / ws=80。

## 实现

```
internal/wsx/wsx.go     AcceptKey / 帧 Encode+Decode / Probe(conn, req)   纯逻辑
internal/cli/net_ws.go  拨号(tls/tcp) + URL 规整 + 接线                    IO
```

- **纯函数好测**：`AcceptKey` 用 RFC6455 §1.3 的示例向量做 byte-equal 断言;帧编解码 masked/unmasked 各尺寸 round-trip;`Probe` 接 `net.Conn`,测试用 `net.Pipe` 起一个假 WS 服务端跑完整握手 + ping/pong。
- **客户端帧必掩码**：`EncodeFrame` 传 4 字节 mask 时置掩码位并 XOR(客户端规则);传 nil 时不掩码(服务端规则)——测试两条都覆盖。

## 有意不做

| 不做 | 原因 |
|------|------|
| 交互式 WS 客户端 REPL | 那是 `wscat` / `websocat` 的活;这里只做一次性探活/往返 |
| 压测 / 并发洪泛 | 会变成攻击工具 |
| WS 服务器 | 另一类东西(跟 `http serve` 不同范畴) |
| 绕鉴权 / 攻击 | 只被动探测,跟 `net cdn`/`http grade` 同一条非攻击性线 |

跟 `jdan net probe`（分阶段探查）、`jdan net cdn`（识别 CDN）、`jdan net selfcheck`（服务端自检）同属 `net` 套件。
