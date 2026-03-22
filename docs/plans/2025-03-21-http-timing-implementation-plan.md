# jdan http timing Implementation Plan

> **For Claude:** REQUIRED SUB-SKILL: Use superpowers:executing-plans to implement this plan task-by-task.

**Goal:** 新增 `jdan http timing <url> [-n N] [--json]`，用 `net/http/httptrace` 拆解 HTTP 请求各阶段耗时，支持多次请求取平均值和 JSON 输出。

**Architecture:** `internal/httptiming/` 纯逻辑包（measure + format），`internal/cli/http_timing.go` 注册 Cobra 子命令。

**Tech Stack:** Go 标准库 `net/http`、`net/http/httptrace`、`encoding/json`；已有 cobra/viper/zerolog。

**设计依据：** `docs/plans/2025-03-21-http-timing-design.md`

---

### Task 1: `Result` 结构体与 `Measure` 函数（TDD）

**Files:**
- Create: `internal/httptiming/measure.go`
- Create: `internal/httptiming/measure_test.go`

**Step 1: 写失败测试**

用 `httptest.NewServer` 创建本地 HTTP 服务，调用 `Measure(ctx, serverURL)`，断言：
- `err == nil`
- `StatusCode == 200`
- `DNSLookup >= 0`（本地回环可能为 0）
- `TCPConnect > 0`
- `TLSHandshake == 0`（httptest 默认非 TLS）
- `ServerProcessing > 0`
- `ContentTransfer >= 0`
- `Total > 0`

**Step 2:** `go test ./internal/httptiming -run TestMeasure -v` → FAIL（函数未实现）。

**Step 3:** 实现 `Measure`：
- 构造 `http.Request`
- 用 `httptrace.ClientTrace` 注册回调：`DNSStart`/`DNSDone`、`ConnectStart`/`ConnectDone`、`TLSHandshakeStart`/`TLSHandshakeDone`、`GotFirstResponseByte`
- 发起请求，`io.Copy(io.Discard, resp.Body)`，记录 `bodyDone` 时间
- 计算各阶段差值填入 `Result`

**Step 4:** `go test ./internal/httptiming -v` → PASS。

**Step 5:** Commit：`feat(httptiming): Measure function with httptrace`

---

### Task 2: HTTPS 测试（TLS 阶段验证）

**Files:**
- Modify: `internal/httptiming/measure_test.go`

**Step 1:** 新增测试 `TestMeasure_HTTPS`，用 `httptest.NewTLSServer`，断言 `TLSHandshake > 0`。注意需要 `InsecureSkipVerify` 或用 server 的 `Client()`。

**Step 2:** `go test ./internal/httptiming -v` → PASS。

**Step 3:** Commit：`test(httptiming): verify TLS handshake timing`

---

### Task 3: 文本与 JSON 格式化（TDD）

**Files:**
- Create: `internal/httptiming/format.go`
- Create: `internal/httptiming/format_test.go`

**Step 1: 写失败测试**

构造固定的 `Result`（或 `[]Result`），断言：
- `FormatText([]Result)` 输出包含 `DNS 查询:`、`总耗时:` 等关键行
- `FormatText` 多结果时包含 `#1`、`#2`、`平均值`
- `FormatJSON([]Result)` 是合法 JSON，反序列化后字段值正确

**Step 2:** 实现 `FormatText` 和 `FormatJSON`。Duration 在 JSON 中以毫秒浮点数表示。单结果文本不带序号；多结果带序号并追加平均值段。

**Step 3:** `go test ./internal/httptiming -v` → PASS。

**Step 4:** Commit：`feat(httptiming): text and JSON formatters`

---

### Task 4: Cobra 子命令 `http timing`

**Files:**
- Create: `internal/cli/http_timing.go`

**Step 1:** 创建 `httpCmd`（`Use: "http"`）和 `httpTimingCmd`（`Use: "timing [url]"`）。

**Step 2:** 注册 flags：`-n`（int，默认 1）、`--json`（bool，默认 false）。

**Step 3:** `RunE` 逻辑：
- 校验 `-n >= 1`
- 循环调用 `httptiming.Measure`，收集 `[]Result`
- 部分失败：zerolog 打出该次错误，继续后续；全失败则返回错误
- 根据 `--json` 选择 `FormatText` 或 `FormatJSON`，写到 stdout

**Step 4:** `go build ./cmd/jdan`，手动验证 `jdan http timing https://httpbin.org/get`。

**Step 5:** Commit：`feat(cli): add http timing command`

---

### Task 5: README 更新与全量验证

**Files:**
- Modify: `README.md`

**Step 1:** 在 README 中添加 `jdan http timing` 的用法说明（参数、示例、JSON 输出）。

**Step 2:** `go test ./... -count=1` 全量通过。

**Step 3:** Commit：`docs: add http timing to README`

---

## 执行交接

按任务顺序执行；每任务结束保持可编译、测试通过后再进入下一任务。
