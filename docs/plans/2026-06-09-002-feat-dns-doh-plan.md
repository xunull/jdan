---
title: "feat: Add --doh flag to jdan dns lookup"
type: feat
status: active
date: 2026-06-09
origin: docs/plans/2026-06-09-001-feat-dns-lookup-plan.md (DoH 此前列入 NOT in scope，本 plan 解锁)
---

# feat: Add --doh flag to jdan dns lookup

## Overview

为 `jdan dns lookup` 增加 `--doh` flag，支持通过 DNS-over-HTTPS (RFC 8484) 查询域名。
核心动机是绕过本地 DNS 劫持（用户实测 `114.114.114.114` 把 NXDOMAIN 替换为 `198.18.0.19`）。

3 个关键决策（已通过 plan-eng-review lock）：
- D1 = 别名 + 完整 URL：`--doh google` / `--doh https://dns.google/dns-query` 都接受
- D2 = 别名内置 IP 绕过本地 DNS：`google → 8.8.8.8`，TLS SNI 仍是 dns.google，证书验证不破
- D3 = 仅 DoH POST + TLS 验证默认开：不做 DoT / HTTP3 / ECS / --insecure-tls

## Problem Frame

DoH 自身有鸡和蛋问题：要 DoH 连接 `dns.google`，得先 DNS 查 `dns.google` 的 IP——而这一步可能被劫持。
方案：别名 → 内置 IP 表。`--doh google` 直接 dial `8.8.8.8:443`，TLS SNI=dns.google，证书验证走 host。
这与 `curl --resolve` / `dig +https` 的实现路径一致。

## Requirements Trace

- R17. 新增 `--doh string` flag，与 `-s/--server` 互斥
- R18. `--doh` 接受三种输入形式：
  - 内置别名：`google` / `cloudflare` / `quad9` / `opendns` / `ali` / `360`
  - 主机名：`dns.google` → 自动补 `https://dns.google/dns-query`
  - 完整 URL：`https://dns.google/dns-query`
- R19. 别名 → URL 解析 + bootstrap IP 表内置，绕过 OS resolver
- R20. 非别名形式（主机名 / 完整 URL）使用 OS resolver 解析 DoH host
- R21. 顶部输出由 `domain — via 8.8.8.8:53` 变为 `domain — via https://dns.google/dns-query` (DoH 模式)
- R22. `--json` 中的 `server` 字段改为 DoH URL（含完整 path）
- R23. 默认验证 TLS 证书；TLS 错误标 `TLS_ERROR`，HTTP 非 200 标 `HTTP_<code>`
- R24. `--timeout` flag 透传到 HTTPS 请求

## Scope Boundaries

- **NOT** DoT (DNS over TLS, port 853)
- **NOT** DoH3 (HTTP/3 transport)
- **NOT** DoH GET 方法（仅 POST）
- **NOT** EDNS Client Subnet (ECS) 传递
- **NOT** `--insecure-tls`（DoH 的初衷就是加密+认证，跳过验证违反产品意图）
- **NOT** 自动 fallback 到 UDP（用户明确选 DoH 就是不要 UDP）

## Context & Research

### Relevant Code and Patterns

- `internal/dnslookup/resolver.go` — Resolver 接口、ensurePort 工具
- `internal/dnslookup/lookup.go` — queryOneType 调用 Resolver.Query
- `internal/cli/dns.go` — newDNSCommand 用 deps injection 注入 lookup func
- `internal/httptiming/measure.go` — http.Transport 自定义 dial 的参照模式

### Technology Stack

- Go 1.25 / net/http 标准库 / github.com/miekg/dns (已有 dns.Msg.Pack / Unpack)
- 零新增依赖

## Key Technical Decisions

- **dohResolver 实现 Resolver 接口**：现有 `queryOneType` 调用代码不变。CLI 层根据 server 形态选 resolver。
- **DialContext 重写**：`http.Transport` 用自定义 DialContext，在表内匹配 host 时连接到 bootstrap IP，TLS SNI 保持原 host。这是 `curl --resolve` 的同款机制。
- **Provider table 单一数据源**：别名表同时承担"短名 → URL"和"host → bootstrap IPs"两个映射，避免维护两份数据。
- **POST 方法**：Content-Type: `application/dns-message`，body 是 dns.Msg.Pack() 输出。RFC 8484 推荐。
- **HTTP 错误码映射**：非 200 转 `HTTP_xxx` Err 字符串；TLS 错误转 `TLS_ERROR`；context 超时仍走 `friendlyErr → TIMEOUT`。

## Data Flow

```
用户:
  jdan dns lookup example.com --doh google

CLI 层 (internal/cli/dns.go):
  parseDoHTarget("google")
    → (url="https://dns.google/dns-query", bootstrapIPs=["8.8.8.8","8.8.4.4"])
  resolver = dnslookup.NewDoHResolver(url, bootstrapIPs, timeout)
  opts.Server = url   // server 字段携带完整 URL

dnslookup.Lookup:
  并发 6 个 type → resolver.Query(ctx, "example.com", qtype, url)

dohResolver.Query (internal/dnslookup/doh.go):
  msg := dns.Msg{Question: {example.com, qtype}}
  body := msg.Pack()
  req := POST url body=application/dns-message
  http.Client.Do(req)
    └─ Transport.DialContext("tcp", "dns.google:443")
       └─ 命中 bootstrap → dial "8.8.8.8:443"
       └─ TLS handshake (SNI=dns.google, verify cert)
  resp.Body → respBytes → dns.Msg.Unpack() → 返回标准 *dns.Msg
```

## Implementation Units

- [ ] **DoH Unit 1: dohResolver + 提供商表**

**Goal:** 实现 DoH 协议层，含别名 + bootstrap IP 表

**Requirements:** R18, R19, R20, R23, R24

**Files:**
- Create: `internal/dnslookup/doh.go`
- Create: `internal/dnslookup/doh_test.go`

**Approach:**

```go
// 数据模型
type DoHTarget struct {
    URL          string   // https://dns.google/dns-query
    BootstrapIPs []string // 用于绕过 OS resolver；空表示走 OS
}

// 别名 / 主机名 / URL → DoHTarget
func ResolveDoHTarget(s string) (DoHTarget, error)

// dohResolver 实现 Resolver 接口
type dohResolver struct {
    client *http.Client
    target DoHTarget
}

func NewDoHResolver(target DoHTarget, timeout time.Duration) Resolver

func (r *dohResolver) Query(ctx, domain, qtype, server string) (*dns.Msg, error) {
    msg := new(dns.Msg)
    msg.SetQuestion(dns.Fqdn(domain), qtype)
    msg.RecursionDesired = true
    body, _ := msg.Pack()
    req, _ := http.NewRequestWithContext(ctx, "POST", r.target.URL, bytes.NewReader(body))
    req.Header.Set("Content-Type", "application/dns-message")
    req.Header.Set("Accept", "application/dns-message")
    resp, err := r.client.Do(req)
    if err != nil { return nil, err }
    defer resp.Body.Close()
    if resp.StatusCode != 200 {
        return nil, fmt.Errorf("HTTP_%d", resp.StatusCode)
    }
    respBytes, _ := io.ReadAll(resp.Body)
    out := new(dns.Msg)
    if err := out.Unpack(respBytes); err != nil { return nil, err }
    return out, nil
}
```

**Provider 别名表（6 个）：**

| 别名 | URL | Bootstrap IPs |
|------|-----|----------------|
| google | https://dns.google/dns-query | 8.8.8.8, 8.8.4.4 |
| cloudflare | https://cloudflare-dns.com/dns-query | 1.1.1.1, 1.0.0.1 |
| quad9 | https://dns.quad9.net/dns-query | 9.9.9.9, 149.112.112.112 |
| opendns | https://doh.opendns.com/dns-query | 208.67.222.222, 208.67.220.220 |
| ali | https://dns.alidns.com/dns-query | 223.5.5.5, 223.6.6.6 |
| 360 | https://doh.360.cn/dns-query | 101.226.4.6, 218.30.118.6 |

**Custom DialContext (TLS SNI override)：**

```go
transport := &http.Transport{
    DialContext: func(ctx context.Context, network, addr string) (net.Conn, error) {
        host, port, _ := net.SplitHostPort(addr)
        for _, ip := range r.target.BootstrapIPs {
            // 命中 bootstrap：直接 dial IP，TLS SNI 在 ClientHello 中由 net/http 自动设为 url host
            if hostMatchesProvider(host, r.target.URL) {
                addr = net.JoinHostPort(ip, port)
                break
            }
        }
        var d net.Dialer
        return d.DialContext(ctx, network, addr)
    },
}
```

**Test scenarios (使用 httptest.NewTLSServer):**
- happy path: POST 收到 valid dns wire format，返回构造的 dns.Msg
- HTTP 500 → Err == "HTTP_500"
- HTTP 200 但 body 不是合法 dns wire format → Err 含 unpack 错误
- context 超时 → Err == "TIMEOUT"
- ResolveDoHTarget("google") → 正确 URL + 2 个 bootstrap IP
- ResolveDoHTarget("dns.google") → URL 自动补 /dns-query，无 bootstrap
- ResolveDoHTarget("https://example.com/dns-query") → 原样 URL，无 bootstrap
- ResolveDoHTarget("INVALID(URL") → error

**Verification:**
- `go test ./internal/dnslookup/...` 通过

---

- [ ] **DoH Unit 2: CLI --doh flag + 路由**

**Goal:** CLI 层加 flag、解析、resolver 路由

**Requirements:** R17, R21, R22

**Files:**
- Modify: `internal/cli/dns.go` (+~30 行)
- Modify: `internal/cli/dns_test.go` (+~50 行)

**Approach:**

```go
// 新增 flag
lookupCmd.Flags().String("doh", "", "DoH endpoint（别名 google/cloudflare/...、主机名或完整 URL）；与 -s 互斥")

// runDNSLookup 中
doh, _ := cmd.Flags().GetString("doh")
if doh != "" && server != "" {
    return fmt.Errorf("--doh 与 --server 不能同时使用")
}
if doh != "" {
    target, err := dnslookup.ResolveDoHTarget(doh)
    if err != nil { return err }
    server = target.URL  // 用于 opts.Server 显示
    // 用 doh resolver 替代默认 miekg resolver
    doLookup = func(ctx, opts) (*Result, error) {
        return dnslookup.Lookup(ctx, dnslookup.NewDoHResolver(target, timeout), opts)
    }
}
// else: 现有 miekgResolver 路径不变
```

**Test scenarios:**
- `--doh google` 触发 DoH resolver；opts.Server 是 https URL
- `--doh https://custom.example/path` 接受任意 URL
- `--doh google -s 8.8.8.8` 报错（互斥）
- `--doh INVALID(URL` 报错
- 不传 --doh 仍走原来的 miekg path

**Verification:**
- `go test ./internal/cli/...` 通过

---

- [ ] **DoH Unit 3: Integration test + README + 验证**

**Goal:** 真实 DoH 服务器烟雾测试 + 文档 + 全量验证

**Requirements:** 用户能找到 + 烟雾测试覆盖真实贯通

**Files:**
- Modify: `internal/dnslookup/integration_test.go`（加 TestIntegration_DoH_*）
- Modify: `README.md`（jdan dns lookup 章节加 --doh 子节）

**Test scenarios (integration tag):**
- `TestIntegration_DoH_Google` 用 `--doh google` 真实查 example.com，期 HasAnySuccess
- `TestIntegration_DoH_Cloudflare` 用 cloudflare 同样测
- `TestIntegration_DoH_BypassHijack`：在劫持环境下，DoH 应该返回真实结果（NXDOMAIN 域名返回 NXDOMAIN 而非 fake IP）

README 加：

```markdown
**通过 DoH 绕过本地 DNS 劫持：**

```bash
jdan dns lookup example.com --doh google         # 使用 Google DoH (8.8.8.8)
jdan dns lookup example.com --doh cloudflare     # 使用 Cloudflare DoH (1.1.1.1)
jdan dns lookup example.com --doh https://dns.alidns.com/dns-query  # 自定义 URL
```

支持的别名：`google` / `cloudflare` / `quad9` / `opendns` / `ali` / `360`。别名形式会用内置的提供商 IP 直连，**绕过本地 resolver**——这是 jdan dns lookup 在 DNS 被劫持环境下的"看真相"模式。
```

**Verification:**
- `go build -o jdan .`
- `go test ./...` (全量单测)
- `go test -tags integration ./internal/dnslookup/...` (真实 DoH)
- 手动: `./jdan dns lookup example.com --doh cloudflare` 看到真实 IP（不是 198.18.0.19）

## System-Wide Impact

- **新增依赖**：无
- **对其他命令的影响**：无
- **CLI 表面**：新增 `--doh` flag
- **二进制 size 影响**：~ +20KB（net/http 已被 jdan http timing 引入）

## Risks & Dependencies

| Risk | Mitigation |
|------|------------|
| 内置 bootstrap IP 长期漂移 | 8.8.8.8 / 1.1.1.1 / 9.9.9.9 多年未变，已列入 RFC；变了 patch 即可 |
| Cloudflare / Google 临时改 endpoint path | 历史 5+ 年未变；用户随时可退回完整 URL 形式 |
| `--doh https://NextDNS UUID path` 这种自定义 path 用户场景 | 完整 URL 形式直接支持，不需要别名 |
| HTTP/2 keep-alive 在 jdan 这种单次 CLI 调用下浪费连接 | http.Transport 默认行为，进程退出连接自然释放，无内存泄漏 |
| TLS 证书过期 / 提供商证书链变更 | 走 Go 默认 root CA，与 curl/Chrome 同步 |

## Documentation / Operational Notes

- README 加 `--doh` 子节
- 之前 design doc 中 "NOT 支持 DoH" 被本 plan 取代

## Sources & References

- **Origin document:** [docs/plans/2026-06-09-001-feat-dns-lookup-plan.md](2026-06-09-001-feat-dns-lookup-plan.md)
- RFC 8484 — DNS Queries over HTTPS (DoH)
- curl `--resolve` flag 文档（同款 SNI override 技术）
- `internal/dnslookup/resolver.go` — Resolver 接口扩展点
- `internal/cli/dns.go` — deps injection 模式
