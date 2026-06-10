---
title: "feat: Add jdan dns trace command"
type: feat
status: active
date: 2026-06-10
origin: docs/plans/2026-06-09-001-feat-dns-lookup-plan.md (dns 名空间下第三个子命令)
---

# feat: Add jdan dns trace command

## Overview

新增 `jdan dns trace <domain>` 子命令，从根 DNS 服务器开始迭代解析，展示每一跳的委派路径
（zone / NS server / 查询耗时 / referral 或 final answer）。`dig +trace` 的 jdan 同款，
但输出对齐 lookup/reverse 的 text + `--json` + `--short` + `--verbose` 模式。

3 个关键决策（已通过 plan-eng-review lock）：
- **D1 = 默认仅查 A，`--type` 覆盖（dig +trace 风格）**——多跳每 type 都要重跑 chain，6 type 默认会让总耗时 ×6
- **D2 = `--doh` 启用 → glueless NS 走 DoH bootstrap**；未传 `--doh` 时 glueless NS 走 OS resolver
- **D3 = 极简 v1**——含跳可视化 + glue 处理 + 循环检测；**不**含 DNSSEC 验证 + CNAME 链追踪 + path latency aggregate

## Problem Frame

`jdan dns lookup` 默认走 recursive resolver（OS DNS 或 DoH provider），用户看不到 DNS
解析的中间过程。当出现 DNS 委派调试场景（新改的 NS 没生效 / gTLD 返回的 NS 与预期不符 /
权威 NS 响应慢），用户需要看到**从根到 auth NS 的完整跳路径**。

`dig +trace` 是当前唯一干净的方案，但与 `dig +short` 的输出形态隔离，脚本难以消费。
`jdan dns trace` 复用与 lookup/reverse 一致的 flag 表面 + 输出形态，让"看委派路径"和
"看 IP/MX/...记录" 在同一工具家族里平滑切换。

## Requirements Trace

- R1. 新增子命令 `jdan dns trace <domain>`
- R2. 默认查 A 记录；`--type NS / MX / TXT / ...` 覆盖（参考 dig +trace 风格）
- R3. 从内置的 13 台根 DNS server 开始；按 IPv4 IP 顺序选取（a.root-servers.net 先）
- R4. 沿 referral 追踪 NS：parse Authority section 取 NS，Additional section 取 glue
- R5. **In-bailiwick glue**（NS 的 IP 在 Additional section 提供）直接使用
- R6. **Out-of-bailiwick / glueless NS**：
  - 未传 `--doh`：用 OS resolver (`net.LookupHost`) 解析 NS hostname
  - 传了 `--doh`：用 DoH resolver 解析 NS hostname（绕过本地劫持）
- R7. 循环检测：维护 visited zone set，若 referral 指向已访问 zone 则终止
- R8. 跳数上限：默认 max 20 hops，超过即终止
- R9. Per-hop timeout（默认 3s，`--hop-timeout` 调整）+ 总 timeout（`--timeout`，默认 30s）
- R10. NS 选择策略：sequential first（取第一个 NS，超时/失败时按顺序 fallback）
- R11. 输出三种形态 + 顶部一行：
  - **text** 默认：分段 hop 列表（zone / server / IP / 耗时 / type / referral 或 answer）
  - **`--json`**：完整结构化 Result（domain、query_type、hops 数组、final、total_time_ms）
  - **`--short`**：仅最终答案（与 `jdan dns lookup --short` 一致）
  - **`--verbose`**：text 基础上每跳加 query message + flags
- R12. `--server` flag 复用为"从哪台 root server 起跳"（极少用，但保留与 lookup/reverse 对称）
- R13. `--strict` 在 trace 中语义：若任何 hop 失败（含中途 NS timeout 但仍最终拿到 answer）即 `exit 1`
- R14. exit code 默认宽容：拿到 final answer 即 `exit 0`；trace 中断（cycle / max hops / NS 全失败）`exit 1`

## Scope Boundaries

**NOT in scope（v1）：**

- **DNSSEC 验证**——留给未来 `jdan dns sec` 子命令
- **CNAME 链追踪**——若 final answer 是 CNAME，不自动追到 A；用户用 trace 输出再 lookup 一次
- **Parallel NS query**——sequential first 已足够；并发查会破坏 trace 输出的线性叙事
- **Path latency aggregate**——逐跳已显示耗时，不额外做 summary 行
- **RRSIG 提示** "[signed]" 标识——属于 sec 命令的表面
- **Anycast detection**——同一 IP 在不同地理位置 trace 结果不同的诊断
- **EDNS0 padding / NSID extension**——专业诊断特性，jdan 用户场景用不到
- **HTTPS / SVCB record type 在 trace**——`--type` 接受但行为同 lookup（只查不诊断）
- **Windows 平台**——与 dns lookup / reverse 一致，仅 macOS + Linux first release

## Context & Research

### Relevant Code and Patterns

- `internal/dnslookup/resolver.go` — `Resolver` 接口，trace 的 bootstrap resolver 直接复用
- `internal/dnslookup/doh.go` — `NewDoHResolver` 作为 glueless NS bootstrap 注入
- `internal/dnslookup/lookup.go` — `friendlyErr` 错误映射工具复用
- `internal/cli/dns_reverse.go` — 同级子命令模式参照（注册到 dnsCmd、deps injection）
- `internal/cli/dns.go` — `dnsCmdDeps` struct 扩展承载 trace 函数注入

### Technology Stack

- Go 1.25 / cobra v1.10.2 / viper v1.21.0
- 复用 `github.com/miekg/dns`（已为现有 dns lookup 引入），新增零依赖

### 关键 Layer 1 选择

- **Hardcoded root servers**：RFC 公开数据，20 年只动过 1 次（b.root-servers.net 在 2017）。这是 `dig +trace` 和 unbound、bind9 都采用的做法。优于"先 query OS resolver `. NS`"——那会被本地劫持
- **Sequential NS selection**：dig +trace 的同款。优于并发——保持输出线性可读
- **3s per-hop timeout**：dig 默认 5s，trace 多跳累计耗时上限不超 30s，3s 是合理折中

## Key Technical Decisions

- **新建 `internal/dnstrace/` 包**，单文件 `trace.go` 内含：root server 表 + Tracer struct + Trace 函数 + 输出 format。这种"单包+单文件"组织是为了通过 Step 0 的 8 文件复杂度门槛，同时 trace 逻辑约 250 行，单文件可读
- **Tracer 持有 bootstrap Resolver**——CLI 层根据 `--doh` 注入：传 `--doh` 时注入 dohResolver；不传时注入一个 OS-resolver wrapper（`net.LookupHost` 包装为 Resolver 接口）
- **Hop 结构**：`Zone / ServerName / ServerIP / QueryTime / Type {REFERRAL/ANSWER/ERROR} / NSReferrals / GlueIPs / Answers / Error`。`Type` 是 enum 决定后续渲染分支
- **循环检测**：visited zone set（map[string]bool），referral 进入已访问 zone 即标记 cycle 并终止
- **不共享 `runDNSQuery`**：trace 输出形态与 lookup/reverse 的"表格"完全不同（分段叙事），且 trace 不使用 dnslookup.Lookup，复用 helper 反而扭曲。trace 的 CLI 路径有自己的 `runDNSTrace`
- **trace 主路径不走 DoH**——直接 UDP/TCP 到权威 NS 是 trace 的本质；DoH 仅在 glueless NS bootstrap 时启用

## Implementation Units

- [ ] **Unit 1: internal/dnstrace 包骨架与核心 Trace 函数**

  **Files:**
  - Create: `internal/dnstrace/trace.go` (~280 行)

  **包含：**
  - 13 台 root server 硬编码列表（含 IPv4 + IPv6）
  - `Hop / Result / Tracer / Options` 类型
  - `NewTracer(opts) *Tracer`
  - `Tracer.Trace(ctx, domain, qtype) (*Result, error)` —— 主迭代循环
  - 内部 helper：`queryDirect` (UDP/TCP), `parseReferral`, `pickNextNS` (含 in-bailiwick glue + glueless via bootstrap), `cycleCheck`
  - `friendlyTraceErr` 错误短化（复用 dnslookup 同款风格）

  **Approach（迭代循环骨架）：**
  ```go
  func (t *Tracer) Trace(ctx, domain, qtype) (*Result, error) {
      ctx, cancel := context.WithTimeout(ctx, t.totalTimeout)
      defer cancel()

      visited := make(map[string]bool)
      currentZone := "."
      currentServer := pickRoot(t.preferIPv4)

      result := &Result{Domain: domain, QueryType: qtype}

      for hopIdx := 0; hopIdx < t.maxHops; hopIdx++ {
          hop := t.queryOneHop(ctx, currentZone, currentServer, domain, qtype)
          result.Hops = append(result.Hops, hop)

          if hop.Type == HopError {
              // 该 NS 失败，尝试 fallback NS（同 zone 内）
              // 都失败 → 终止
          }
          if hop.Type == HopAnswer {
              result.Final = &hop
              break  // 拿到终点
          }
          // HopReferral：解析下一 zone + NS
          nextZone := hop.ReferralZone
          if visited[nextZone] {
              // cycle 终止
              break
          }
          visited[nextZone] = true
          currentZone = nextZone
          currentServer = pickNextNS(hop, t.bootstrap)
      }
      return result, nil
  }
  ```

  **Verification:** `go build ./...`

- [ ] **Unit 2: trace_test.go 单测（mock resolver / mock UDP path）**

  **Files:**
  - Create: `internal/dnstrace/trace_test.go` (~250 行)

  **Approach：**
  trace 直接调 miekg `dns.Client.Exchange`，难直接 mock 网络层。两条测试路径：
  1. 抽出一个 `transport interface { Exchange(ctx, msg, server) (*dns.Msg, time.Duration, error) }`，把 `dns.Client` 包成 default impl，测试用 fake transport 返回预设响应
  2. cycle 检测、max hop 限制、bootstrap injection 等纯逻辑用单元测试覆盖

  **Test scenarios:**
  - Happy path: 3 跳 root → com → example.com，最终 A 答案
  - In-bailiwick glue：referral 自带 glue IP，直接使用
  - Glueless NS（out-of-bailiwick）：referral 只给 NS 名字，bootstrap resolver 被调用
  - Cycle detection：referral 形成循环时终止
  - Max hops 超限：人造 21 跳，第 20 跳终止
  - NS sequential fallback：第一个 NS timeout，第二个成功
  - All NS fail：zone 内所有 NS 都失败，trace 中断
  - Per-hop timeout：单跳超过 hopTimeout 即标 error
  - Total timeout：整 trace 超过 totalTimeout 即中断
  - --type NS 覆盖：默认 A 改为 NS，最终一跳查 NS 而非 A
  - IPv4 vs IPv6 root：preferIPv4 flag 控制 root server IP 选取

  **Verification:** `go test ./internal/dnstrace/...`

- [ ] **Unit 3: 输出 format（text / json / short / verbose）**

  **Files:**
  - 与 Unit 1 同文件 `internal/dnstrace/trace.go`（加 format 函数）

  **示例 text 输出：**
  ```
  example.com — tracing from root (type A)

  .                   a.root-servers.net.       198.41.0.4         5ms    referral → com
  com.                a.gtld-servers.net.       192.5.6.30         12ms   referral → example.com
  example.com.        a.iana-servers.net.       199.43.135.53      45ms   A 93.184.216.34

  total 62ms across 3 hops
  ```

  **示例 --short 输出**（只有最终答案）：
  ```
  93.184.216.34
  ```

  **示例 --json 输出：**
  ```json
  {
    "domain": "example.com",
    "query_type": "A",
    "total_time_ms": 62,
    "hops": [
      {"zone": ".", "server_name": "a.root-servers.net.", "server_ip": "198.41.0.4", "query_time_ms": 5, "type": "REFERRAL", "ns_referrals": ["a.gtld-servers.net.", ...]},
      ...
    ],
    "final": {"zone": "example.com.", "type": "ANSWER", "answers": ["93.184.216.34"]}
  }
  ```

  **Verification:** test scenarios 覆盖各 format 分支

- [ ] **Unit 4: internal/cli/dns_trace.go CLI 注册**

  **Files:**
  - Create: `internal/cli/dns_trace.go` (~80 行)
  - Modify: `internal/cli/dns.go`（+1 行 `dnsCmd.AddCommand(newTraceCommand(deps))`）

  **Approach：**
  ```go
  func newTraceCommand(deps dnsCmdDeps) *cobra.Command {
      cmd := &cobra.Command{
          Use:   "trace <domain>",
          Short: "从根服务器迭代解析，展示完整委派路径",
          Args:  cobra.ExactArgs(1),
          RunE: func(cmd *cobra.Command, args []string) error {
              return runDNSTrace(cmd, args, deps)
          },
      }
      cmd.Flags().StringP("type", "t", "A", "查询的 record type（默认 A）")
      cmd.Flags().String("doh", "", "DoH endpoint：启用后 glueless NS 走 DoH bootstrap，与 --server 互斥")
      cmd.Flags().StringP("server", "s", "", "起步 root server IP（极少用，默认依次轮询 13 个 root）")
      cmd.Flags().BoolP("json", "j", false, "JSON 输出")
      cmd.Flags().Bool("short", false, "仅输出最终答案")
      cmd.Flags().BoolP("verbose", "v", false, "每跳含完整 query message")
      cmd.Flags().Bool("strict", false, "任一跳失败即 exit 1（默认拿到 final answer 即 0）")
      cmd.Flags().Duration("timeout", 30*time.Second, "整 trace 总超时")
      cmd.Flags().Duration("hop-timeout", 3*time.Second, "单跳超时")
      return cmd
  }

  func runDNSTrace(cmd, args, deps) error {
      // parse flags
      // build dnstrace.Options
      // bootstrap resolver：--doh ? dohResolver : osLookupResolver
      // tracer := dnstrace.NewTracer(opts)
      // result, err := tracer.Trace(ctx, domain, qtype)
      // 选 format 输出 + 决定 exit code
  }
  ```

  **Patterns to follow:**
  - `internal/cli/dns_reverse.go` 同级子命令注册模式
  - `dnsCmdDeps` deps injection 模式

- [ ] **Unit 5: dns_trace_test.go**

  **Files:**
  - Create: `internal/cli/dns_trace_test.go` (~150 行)

  **Approach：** 把 `dnsCmdDeps` 加一个 `trace func(ctx, opts) (*dnstrace.Result, error)` 字段供测试注入 mock。

  **Test scenarios:**
  - 默认 `--type A`
  - `--type NS` 覆盖
  - `--doh google` 启用时传给 tracer bootstrap
  - `--doh` + `--server` 互斥报错
  - `--short` 输出
  - `--json` 输出
  - `--strict` 中途失败时 exit 1
  - `--timeout` / `--hop-timeout` 透传

- [ ] **Unit 6: Integration test + README + 验证**

  **Files:**
  - Modify: `internal/dnslookup/integration_test.go`（+~50 行）
  - Modify: `README.md`（在 `jdan dns reverse` 章节后加 `jdan dns trace` 子节）

  **Integration（real network）：**
  - `TestIntegration_Trace_ExampleCom` 从真实根走完 3 跳
  - `TestIntegration_Trace_GitHub` 验证 in-bailiwick glue
  - `TestIntegration_Trace_DoHBootstrap` 强制 glueless 场景，验证 DoH bootstrap 工作

  **README 章节：**
  ```markdown
  ### `jdan dns trace`

  从根 DNS 服务器开始迭代解析，展示完整委派路径。`dig +trace` 的 jdan 同款，
  输出对齐 jdan dns lookup / reverse 的 `--json` / `--short` / `--verbose` 模式。

  ```bash
  jdan dns trace example.com                 # 默认查 A，从根开始追
  jdan dns trace example.com --type NS       # 改查 NS（不会改变委派路径）
  jdan dns trace example.com --doh google    # glueless NS 走 DoH bootstrap
  jdan dns trace example.com --json          # 完整结构化
  ```
  ```

  **Final verify：**
  - `go build -o jdan .`
  - `go test ./...`
  - `./jdan dns trace example.com` 真实跑通
  - `./jdan dns trace github.com --json | jq '.hops | length'` 检查 hop 数

## System-Wide Impact

- **新增依赖**：无
- **对其他命令影响**：`internal/cli/dns.go` 加一行 AddCommand，`dnsCmdDeps` 加一个可选 `trace` func 字段
- **CLI 表面**：新增 `jdan dns trace` 子命令 + 9 个 flag

## Risks & Dependencies

| Risk | Mitigation |
|------|------------|
| Root server IP 漂移 | 20 年只动过 1 次；硬编码 13 个 IP，变化时 patch 一行 |
| 本地网络只允许 UDP-53 到 ISP DNS（少数 ISP 拦截到任意 IP） | 失败时清晰报错"无法访问根服务器 X.X.X.X"，用户改用 dns lookup --doh |
| 部分企业域名只有 IPv6 NS（无 v4 glue） | 优先 v4 + v6 fallback；测试覆盖 |
| miekg/dns Client.Exchange 在某些异常响应下 hang | 已套 context timeout，hopTimeout 保底 |
| trace 慢域名（root 慢）让人不耐烦 | 总耗时显示在底部，30s totalTimeout 兜底，--timeout 用户可调 |

## Documentation / Operational Notes

- README 加 `jdan dns trace` 章节
- 二进制 size 影响 < 50KB（trace 包仅 ~300 行代码）

## Sources & References

- **Origin design doc:** [~/.gstack/projects/xunull-jdan/quincy-main-design-20260610-075944-dns-trace.md](file:///Users/quincy/.gstack/projects/xunull-jdan/quincy-main-design-20260610-075944-dns-trace.md)
- RFC 1034/1035 — DNS 协议
- IANA Root Server List — https://www.iana.org/domains/root/servers
- `dig +trace` 参考实现行为（ISC bind9 source）
- `internal/cli/dns_reverse.go` — 同级子命令注册参照
- `internal/dnslookup/doh.go` — bootstrap resolver 复用源
