# jdan http timing：URL 访问耗时拆解（设计）

**状态：** 已定稿（2025-03-21）  
**范围：** 新增 `jdan http timing <url>` 子命令，拆解 HTTP 请求各阶段耗时。

## 目标

让用户快速了解访问一个 URL 时**时间花在哪里**：DNS、TCP、TLS、服务端处理、内容传输，以及总耗时与 HTTP 状态码。类似 `curl -w` 的耗时变量，但以 jdan 子命令形式提供，支持多次请求取平均值和 JSON 输出。

## 命令形式

```
jdan http timing <url> [-n 次数] [--json]
```

- `http`：新顶层分组（与 `file` 平级），未来可扩展 `http header`、`http status` 等。
- `timing`：`http` 下的子命令。
- `<url>`：必填参数（`cobra.ExactArgs(1)`）。
- `-n`：整数，默认 `1`。大于 1 时每次单独输出，末尾追加**平均值**。
- `--json`：布尔，默认 `false`。开启后输出 JSON。

## 技术方案

使用 Go 标准库 **`net/http/httptrace.ClientTrace`** 的回调记录各阶段时间戳，零外部依赖，跨平台无差异。

## 架构

- `internal/cli/http_timing.go`：Cobra `httpCmd`（分组）+ `httpTimingCmd`（子命令），注册到 `rootCmd`。
- `internal/httptiming/`：纯逻辑包，不依赖 CLI 或日志。
  - `measure.go`：`Measure(ctx, url) (Result, error)` — 单次 HTTP GET + httptrace 回调。
  - `format.go`：文本与 JSON 格式化。

## 数据结构

```go
type Result struct {
    URL              string
    StatusCode       int
    DNSLookup        time.Duration
    TCPConnect       time.Duration
    TLSHandshake     time.Duration
    ServerProcessing time.Duration
    ContentTransfer  time.Duration
    Total            time.Duration
}
```

## 各阶段计算

| 阶段 | 计算方式 |
|------|----------|
| DNS 查询 | `DNSDone - DNSStart` |
| TCP 连接 | `ConnectDone - ConnectStart` |
| TLS 握手 | `TLSHandshakeDone - TLSHandshakeStart`（HTTP 时为 0） |
| 服务端处理 | `GotFirstResponseByte - (TLSHandshakeDone 或 ConnectDone)` |
| 内容传输 | `BodyReadDone - GotFirstResponseByte` |
| 总耗时 | `BodyReadDone - 请求开始` |

## 输出格式

### 文本（默认）

单次：

```
URL: https://github.com
状态码:            200
DNS 查询:          23.45ms
TCP 连接:          45.12ms
TLS 握手:          89.34ms
服务端处理:        120.56ms
内容传输:          15.23ms
总耗时:            293.70ms
```

多次（`-n 3`）：每次输出一段（标注 `#1`、`#2`、`#3`），末尾追加一段**平均值**。

### JSON（`--json`）

Duration 以毫秒浮点数表示。`-n > 1` 时输出数组，额外附带 `"average"` 对象。`-n 1` 时输出单个对象。

## 响应体处理

Body **读完即丢**（等价 curl `-o /dev/null`），只关心耗时，不输出内容。

## 错误处理

- URL 格式不合法：报错退出。
- 网络不通 / DNS 失败 / 超时：返回底层错误信息，非零退出。
- `-n` 小于 1：报错退出。
- 多次请求中**部分失败**：该次标记为失败并输出错误信息，平均值只计算成功次数；若全部失败则非零退出。

## 测试策略

- **单元测试**：`httptest.NewServer` 提供本地 HTTP 端点，验证 `Measure` 返回的各 Duration 字段 ≥ 0、`StatusCode` 正确。
- **输出格式测试**：文本与 JSON 格式化函数各自测。

## 非目标（YAGNI）

- 不支持自定义 HTTP 方法（固定 GET）。
- 不支持自定义 Header / Body。
- 不支持代理配置（首版）。
- 不支持跟随重定向的逐跳拆解（只看最终结果）。
