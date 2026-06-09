---
title: "feat: Add jdan dns lookup command"
type: feat
status: active
date: 2026-06-09
origin: docs/brainstorms/2026-06-09-dns-lookup-requirements.md
---

# feat: Add jdan dns lookup command

## Overview

新增 `jdan dns lookup <domain>` 子命令，通过 `github.com/miekg/dns` 库并发查询多个 DNS record type（默认 A + AAAA + MX + TXT + CNAME + NS 共 6 个），输出三列表格（type、TTL、value），支持 `--json` / `--short` / `--verbose` 三种输出形态，宽容错误处理 + `--strict` opt-in，默认从 `/etc/resolv.conf` 读取 DNS server 并支持 `--server` 覆盖。first release 仅 macOS + Linux。

## Problem Frame

`dig` 默认只查 A 记录，看 MX/TXT 需多次执行，输出难扫视、难脚本消费。`jdan dns lookup` 一发命令并发查 6 个常用 type，输出简洁三列 + JSON 双模式，是 DNS 诊断场景的"人体工学"工具。

## Requirements Trace

- R1. 新增子命令 `jdan dns lookup <domain>`（两级结构）
- R2. 默认 text 输出 + `--json` / `--short` / `--verbose` 切换
- R3. 默认并发查询 6 个 type（A + AAAA + MX + TXT + CNAME + NS）
- R4. `-t` / `--type` 单 type 或逗号分隔多 type
- R5. `-t all` 查询 9 个 type
- R6. 默认从 `/etc/resolv.conf` 读取 DNS server
- R7. `--server` / `-s` 指定 server
- R8. 顶部输出 `domain — via <server>`
- R9. 默认三列：TYPE、TTL、VALUE
- R10. 无记录显示 `(no records)`，不算错误
- R11. `--verbose` 加 server、query time、flags、rcode
- R12. `--json` 输出完整 metadata
- R13. 宽容 exit code（任一成功即 0）
- R14. 部分失败标注 `⚠`
- R15. `--strict` 切换严格模式
- R16. 默认超时 5s，`--timeout` 可调

## Scope Boundaries

- 仅 macOS + Linux（Windows 留待后续，see brainstorm "NOT in scope" 段）
- 无 DNSSEC、DoH / DoT
- 无 reverse / trace / ANY 查询
- 无配置文件

## Context & Research

### Relevant Code and Patterns

- `internal/cli/http_timing.go` — 两级动词（`http timing`）与 JSON/text 双模式输出参照
- `internal/cli/ports.go` — `--json` flag 与表格 + JSON 双输出参照
- `internal/cli/obsidian.go` — 两级子命令（`obsidian install-claudian`）结构参照
- `internal/httptiming/measure.go` — 接口注入（`http.RoundTripper` 通过参数传入）以支持 mock 测试的参照
- `internal/httptiming/measure_test.go` — 用 mock transport 测网络代码的参照
- `internal/cli/root.go` — cobra 命令注册和 flag 绑定模式

### Technology Stack

- Go 1.25 / cobra v1.10.2 / viper v1.21.0
- 新增依赖：`github.com/miekg/dns`（业界 DNS 标准库）

## Key Technical Decisions

- **miekg/dns 单一路径**：纯走 miekg/dns，不混用 `net.LookupX`，避免 `--json` 输出字段不稳定（D2）。
- **两级命令空间**：`jdan dns lookup`，为未来 `dns reverse` / `trace` 留扩展（D3）。
- **默认 6 type 并发**：通过 `errgroup` 或 `sync.WaitGroup` + buffered channel 并发查询，总耗时 ≈ 最慢单 type（D4）。
- **三列默认输出**：TYPE、TTL、VALUE，使用 `text/tabwriter` 对齐（D5）。
- **resolver fallback 三层**：`/etc/resolv.conf` → 失败时尝试空 fallback 列表 → 最终 `8.8.8.8`（D6）。
- **宽容 exit code**：任一 type NOERROR（含空结果）即 `exit 0`；`--strict` 切换为任一失败即 `exit 1`（D7）。
- **Resolver 接口化**：`type Resolver interface { Query(ctx, domain, qtype, server) (*dns.Msg, error) }`，生产用 miekg.Client，测试用 fakeResolver（D8）。
- **integration build tag**：真实 DNS 测试 `//go:build integration`，CI 默认不跑（D8）。

## Implementation Units

- [ ] **Unit 1: 创建 `internal/dnslookup/` 包骨架与 Resolver 接口**

**Goal:** 建立内部包结构、定义 Resolver 接口、实现生产端 miekgResolver

**Requirements:** 为 R6, R7 提供查询能力基础

**Dependencies:** 新增 `github.com/miekg/dns` 到 go.mod

**Files:**
- Create: `internal/dnslookup/resolver.go`
- Create: `internal/dnslookup/resolver_test.go`

**Approach:**
```go
// resolver.go
package dnslookup

import (
    "context"
    "github.com/miekg/dns"
)

type Resolver interface {
    Query(ctx context.Context, domain string, qtype uint16, server string) (*dns.Msg, error)
}

type miekgResolver struct {
    client *dns.Client
}

func NewResolver() Resolver {
    return &miekgResolver{client: &dns.Client{Net: "udp", Timeout: 5 * time.Second}}
}

func (r *miekgResolver) Query(ctx context.Context, domain string, qtype uint16, server string) (*dns.Msg, error) {
    msg := new(dns.Msg)
    msg.SetQuestion(dns.Fqdn(domain), qtype)
    // 注意：context 取消需通过 client.ExchangeContext 传入
    resp, _, err := r.client.ExchangeContext(ctx, msg, ensurePort(server))
    return resp, err
}
```

**Patterns to follow:**
- `internal/httptiming/measure.go` 把 `http.RoundTripper` 作为参数注入的解耦风格

**Test scenarios:**
- 无需测 miekgResolver 本身（薄包装，集成测试覆盖）
- 单测 `ensurePort` 工具函数：`"8.8.8.8"` → `"8.8.8.8:53"`，`"8.8.8.8:5353"` 保持

**Verification:**
- `go build ./...` 通过，`go mod tidy` 引入 miekg/dns

---

- [ ] **Unit 2: 实现多 type 并发查询和结果合并**

**Goal:** 实现核心 `Lookup` 函数，并发查询多个 type 并合并为统一结果结构

**Requirements:** R3, R4, R5, R13, R14, R16

**Dependencies:** Unit 1

**Files:**
- Create: `internal/dnslookup/lookup.go`
- Create: `internal/dnslookup/lookup_test.go`

**Approach:**
```go
// 数据结构
type Options struct {
    Domain   string
    Types    []uint16     // 查询的 record type 列表（dns.TypeA 等）
    Server   string       // DNS server (host:port)
    Timeout  time.Duration
    Strict   bool
}

type TypeResult struct {
    Type     string    // "A", "MX" ...
    Rcode    string    // "NOERROR", "NXDOMAIN", ...
    TTL      uint32    // 取最小 TTL；空结果为 0
    Values   []string  // 渲染后的值列表（A: IP；MX: "10 mx.example.com"；TXT: 带引号字符串）
    Err      string    // 非 rcode 错误（timeout / network），空表示无错误
}

type Result struct {
    Domain       string
    Server       string
    QueryTimeMs  int64
    Results      []TypeResult  // 按 Options.Types 顺序
}

func Lookup(ctx context.Context, r Resolver, opts Options) (*Result, error) {
    // 1. 应用 timeout: ctx, cancel := context.WithTimeout(ctx, opts.Timeout)
    // 2. errgroup 并发查询每个 type
    // 3. 每个 type 单独 catch error 写入 TypeResult.Err，不影响其他 type
    // 4. 合并结果，记录 query 总耗时
}

// helper: 把 *dns.Msg 的 Answer section 渲染为 []string
func renderAnswers(qtype uint16, ans []dns.RR) []string { ... }

// helper: ResolveStrategy 决定 exit code 语义
func (res *Result) HasAnySuccess() bool { ... }
func (res *Result) AllFailed() bool { ... }
```

**Patterns to follow:**
- `internal/httptiming/measure.go` 的 timeout 处理风格

**Test scenarios（用 fakeResolver mock）:**
- Happy path: 6 type 全成功，Result.Results 长度等于 6
- 部分失败: 4 个 NOERROR + 1 个 TIMEOUT + 1 个 SERVFAIL，HasAnySuccess() = true, AllFailed() = false
- 全部 NXDOMAIN: AllFailed() = true
- NOERROR 但空记录（如 MX 不存在）: 不算错误，Values 为空，Err 为空
- 单 type 查询: Options.Types = [dns.TypeA]
- `-t all` 9 type
- TXT 多字符串拼接（一条 TXT 可能有多 string）：渲染为带引号的拼接字符串
- MX 渲染为 `"<preference> <host>"`
- NS / CNAME 渲染为 trailing-dot hostname
- AAAA 渲染为压缩 IPv6

**Verification:**
- `go test ./internal/dnslookup/...` 100% 通过
- 表格驱动测试覆盖每个 type 的渲染

---

- [ ] **Unit 3: 实现输出格式化（text / short / verbose / json）**

**Goal:** 把 Result 渲染为 4 种输出格式

**Requirements:** R2, R8, R9, R10, R11, R12

**Dependencies:** Unit 2

**Files:**
- Create: `internal/dnslookup/format.go`
- Create: `internal/dnslookup/format_test.go`

**Approach:**

```go
// format.go
func FormatText(res *Result) string     // 默认三列 + 顶部 "domain — via server"
func FormatShort(res *Result) string    // 仅值，一行一个（dig +short 风格）
func FormatVerbose(res *Result) string  // 三列 + 顶部追加 query time、flags、每 type rcode
func FormatJSON(res *Result) (string, error)  // 完整 metadata
```

**Text 输出示例（默认）：**
```
example.com — via 8.8.8.8

TYPE   TTL    VALUE
A      3600   93.184.216.34
AAAA   3600   2606:2800:220:1:248:1893:25c8:1946
MX     —      (no records)
TXT    3600   "v=spf1 -all"
CNAME  —      (no records)
NS     86400  a.iana-servers.net.
                b.iana-servers.net.
```

**部分失败示例：**
```
TYPE   TTL    VALUE
A      3600   93.184.216.34
MX     ⚠      TIMEOUT
TXT    3600   "v=spf1 -all"
```

**JSON 结构：**
```json
{
  "domain": "example.com",
  "server": "8.8.8.8:53",
  "query_time_ms": 23,
  "results": [
    {"type": "A", "rcode": "NOERROR", "ttl": 3600, "values": ["93.184.216.34"]},
    {"type": "MX", "rcode": "NOERROR", "ttl": 0, "values": []},
    {"type": "TXT", "rcode": "NOERROR", "ttl": 3600, "values": ["\"v=spf1 -all\""]},
    {"type": "AAAA", "error": "i/o timeout"}
  ]
}
```

**Patterns to follow:**
- `internal/httptiming/format.go` 的双模式格式化分离

**Test scenarios:**
- 快照测试: 固定输入 → 固定输出字符串（go test -update 模式可用 testdata/）
- 多值 NS 缩进续行
- 部分失败标注 `⚠`
- 空 results 数组 JSON 仍合法
- TXT 值中包含引号正确转义

**Verification:**
- `go test ./internal/dnslookup/...` 通过

---

- [ ] **Unit 4: 实现 resolver 自动检测**

**Goal:** 从 `/etc/resolv.conf` 读取系统 DNS server，失败 fallback `8.8.8.8`

**Requirements:** R6

**Dependencies:** Unit 1

**Files:**
- Create: `internal/dnslookup/sysresolver.go`
- Create: `internal/dnslookup/sysresolver_test.go`

**Approach:**

```go
// sysresolver.go
//go:build !windows

func DetectSystemServer() string {
    cfg, err := dns.ClientConfigFromFile("/etc/resolv.conf")
    if err == nil && len(cfg.Servers) > 0 {
        return cfg.Servers[0] + ":" + cfg.Port  // miekg 返回不带端口
    }
    return "8.8.8.8:53"
}
```

**Test scenarios:**
- 用 `t.TempDir()` 模拟 resolv.conf（传入路径参数化版本以便测试）
- 文件不存在 → fallback 8.8.8.8:53
- 文件无 nameserver 行 → fallback
- 多个 nameserver → 取第一个

**Verification:**
- `go test ./internal/dnslookup/...` 通过

---

- [ ] **Unit 5: 实现 CLI 命令层**

**Goal:** 注册 `jdan dns lookup`，绑定 flag，调用 dnslookup 包

**Requirements:** R1, R2, R4, R7, R8, R15

**Dependencies:** Unit 1-4

**Files:**
- Create: `internal/cli/dns.go`
- Create: `internal/cli/dns_test.go`（轻量级，仅测 flag 解析）

**Approach:**

```go
// dns.go
var dnsCmd = &cobra.Command{
    Use:   "dns",
    Short: "DNS 相关子命令",
}

var dnsLookupCmd = &cobra.Command{
    Use:   "lookup <domain>",
    Short: "查询域名的多个 DNS 记录类型",
    Args:  cobra.ExactArgs(1),
    RunE: func(cmd *cobra.Command, args []string) error {
        domain := args[0]
        // 1. 读 flag: --type, --server, --json, --short, --verbose, --strict, --timeout
        // 2. 解析 type 列表（"all" → 9 个；逗号分隔 → 多个；空 → 默认 6 个）
        // 3. 解析 server（空 → DetectSystemServer()）
        // 4. dnslookup.Lookup(ctx, dnslookup.NewResolver(), opts)
        // 5. 按 flag 选择 Format*
        // 6. exit code: !strict → AllFailed() ? 1 : 0；strict → AnyFailed() ? 1 : 0
        return nil
    },
}

func init() {
    dnsLookupCmd.Flags().StringP("type", "t", "", "查询的 record type，逗号分隔，'all' 表示 9 种")
    dnsLookupCmd.Flags().StringP("server", "s", "", "DNS server (e.g. 8.8.8.8)")
    dnsLookupCmd.Flags().BoolP("json", "j", false, "JSON 输出")
    dnsLookupCmd.Flags().Bool("short", false, "仅输出值，dig +short 风格")
    dnsLookupCmd.Flags().BoolP("verbose", "v", false, "输出 server、query time、flags 等元数据")
    dnsLookupCmd.Flags().Bool("strict", false, "任一 type 失败即 exit 1")
    dnsLookupCmd.Flags().Duration("timeout", 5*time.Second, "整体查询超时")
    dnsCmd.AddCommand(dnsLookupCmd)
    rootCmd.AddCommand(dnsCmd)
}
```

**Patterns to follow:**
- `internal/cli/obsidian.go` 的两级子命令注册（`dnsCmd` 作为伞型 + `dnsLookupCmd` 作为子命令）
- `internal/cli/http_timing.go` 的 flag 绑定 + 双输出分支

**Test scenarios:**
- `jdan dns lookup -h` 列出所有 flag
- `jdan dns lookup` 无参数报错（cobra ExactArgs 自动处理）
- type 解析单测：`""` → 6 默认 type；`"A"` → [A]；`"A,MX,TXT"` → [A, MX, TXT]；`"all"` → 9 type；`"INVALID"` → 错误

**Verification:**
- `go build -o jdan . && ./jdan dns lookup -h` 输出包含所有 flag
- `./jdan dns lookup example.com` 输出 6 个 type（需要网络）

---

- [ ] **Unit 6: Integration 测试（build tag 隔离）**

**Goal:** 真实 DNS 烟雾测试，CI 默认不跑

**Requirements:** 覆盖 success criteria 中的真实场景

**Dependencies:** Unit 1-5

**Files:**
- Create: `internal/dnslookup/integration_test.go`

**Approach:**

```go
//go:build integration

package dnslookup

import "testing"

func TestIntegration_ExampleCom(t *testing.T) {
    r := NewResolver()
    res, err := Lookup(context.Background(), r, Options{
        Domain: "example.com",
        Types: []uint16{dns.TypeA, dns.TypeAAAA, dns.TypeNS},
        Server: "8.8.8.8:53",
        Timeout: 5 * time.Second,
    })
    if err != nil { t.Fatal(err) }
    if !res.HasAnySuccess() { t.Fatal("expected success") }
}

func TestIntegration_NXDOMAIN(t *testing.T) {
    // nonexistent-dns-test-12345-jdan.invalid
    // 期望 AllFailed() = true
}
```

**Verification:**
- `go test -tags integration ./internal/dnslookup/` 通过（网络可用时）
- 默认 `go test ./...` 不跑这些（CI 友好）

---

- [ ] **Unit 7: 文档 + README 更新**

**Goal:** 用户能找到并理解新命令

**Requirements:** R1-R16 documentation

**Dependencies:** Unit 5

**Files:**
- Modify: `README.md` — 新增 `### jdan dns lookup` 章节

**Approach:**
- 参照 `### jdan ports` / `### jdan http timing` 的 README 风格
- 示例覆盖：基础用法、`-t` 限定 type、`-t all`、`--server`、`--json`、`--short`、`--verbose`、`--strict`、`--timeout`
- 在示例中说明默认 6 type 与 dig 的差异

**Verification:**
- README 示例命令本地手验通过

## System-Wide Impact

- **新增依赖**：`github.com/miekg/dns`（直接依赖，约 + 200KB 二进制 size，转量经过 stat 验证）
- **对其他命令的影响**：无
- **CLI 表面**：新增 `jdan dns` 名空间 + `jdan dns lookup` 子命令 + 7 个 flag

## Risks & Dependencies

| Risk | Mitigation |
|------|------------|
| miekg/dns 的 ExchangeContext 在某些边缘 server 上有 EDNS 兼容问题 | 默认不开 EDNS0；用户报告问题后加 `--edns` flag |
| `/etc/resolv.conf` 在某些 Linux 容器/WSL 环境下指向 systemd-resolved 的 stub resolver（127.0.0.53），可能行为异常 | 测试覆盖；遇到时 fallback 8.8.8.8 |
| TTL 表示：DNS 多记录的 TTL 可能不同（A 记录有多个 IP 各自 TTL）| 取该 type 所有 RR 的 min TTL；JSON 输出中可单独暴露每条 TTL（v2 改进） |
| 6 type 并发可能被某些权威 DNS server rate limit | timeout 后正常标注，不影响其他 type；`--strict` 用户可发现问题 |

## Documentation / Operational Notes

- README.md 新增 `jdan dns lookup` 章节
- 二进制 size 增加约 200KB（miekg/dns + transitives）—— 可接受，对比 jdan 现有 ~15MB 二进制
- `go install github.com/xunull/jdan@latest` 自动包含新命令

## Sources & References

- **Origin document:** [docs/brainstorms/2026-06-09-dns-lookup-requirements.md](../brainstorms/2026-06-09-dns-lookup-requirements.md)
- `github.com/miekg/dns` README & examples
- `internal/cli/http_timing.go` — 两级命令 + JSON/text 双输出参照
- `internal/cli/ports.go` — `--json` flag + 表格输出参照
- `internal/cli/obsidian.go` — 两级动词族命令注册参照
- `internal/httptiming/measure.go` — 接口注入测试参照
