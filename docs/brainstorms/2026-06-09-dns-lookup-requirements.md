---
date: 2026-06-09
topic: dns-lookup
---

# DNS 多记录类型查询

## Problem Frame

开发者排查 DNS 问题时常用 `dig`，但 `dig` 默认只查 A 记录，要看 MX、TXT、NS 必须分多次执行，输出格式以 `;; ANSWER SECTION:` 形式呈现，信息密度高但不易扫视，更难被脚本消费。在 DNS 被劫持、CDN 切换、邮件 SPF 配置验证等典型诊断场景里，用户希望「一发命令把这个域名的所有关键记录都看一眼」。

`jdan dns lookup <domain>` 默认并发查询 6 个最常用 record type（A、AAAA、MX、TXT、CNAME、NS），输出为可扫视的三列表格（type、TTL、value），同时支持 `--json` 供脚本消费、`--short` 模拟 `dig +short`、`--verbose` 输出 server 和 query time 等元数据。

## Requirements

**命令与入口**
- R1. 新增子命令 `jdan dns lookup <domain>`，两级结构为 `dns` 名空间下未来的 reverse / trace / sec 等子功能留扩展位。
- R2. 默认人类可读三列表格输出；支持 `--json`（`-j`）输出 JSON、`--short` 仅输出值（脚本友好）、`--verbose`（`-v`）展示 server 与 query time 等元数据。

**Record type 行为**
- R3. 默认并发查询 A + AAAA + MX + TXT + CNAME + NS 共 6 个 record type。
- R4. 支持 `-t` / `--type` 指定单 type 或逗号分隔多 type（如 `-t A,MX,TXT`）。
- R5. 支持 `-t all` 关键字查询 9 个 type（默认 6 个 + SOA + CAA + SRV）。

**Resolver**
- R6. 默认从 `/etc/resolv.conf` 读取系统 DNS server（miekg/dns 提供 `dns.ClientConfigFromFile`），失败则 fallback `8.8.8.8`。
- R7. 支持 `--server` / `-s` 指定单个或多个 DNS server，格式接受 `8.8.8.8`、`8.8.8.8:53`、`dns.google`。
- R8. 默认顶部输出一行 `domain — via <server>`，让用户明确查询源。

**输出元数据**
- R9. 默认 text 三列：TYPE、TTL（秒）、VALUE。多值（如 NS 有多个）缩进续行。
- R10. 无记录的 type 显示 `(no records)` 或 `—`，**不算错误**。
- R11. `--verbose` 在顶部追加 server、query time（ms）、flags、rcode。
- R12. `--json` 输出完整 metadata（domain、server、query_time_ms、results 数组含 type、ttl、values、rcode）。

**错误处理与 exit code**
- R13. 默认宽容策略：任一 type 有结果（含 NOERROR 空结果）即 `exit 0`；全部 type 都是 NXDOMAIN/SERVFAIL/TIMEOUT 才 `exit 1`。
- R14. 部分失败的 type 在 text 输出中标注 `⚠  TIMEOUT` 或 `⚠  SERVFAIL`，其他 type 正常显示。
- R15. 支持 `--strict` 切换严格模式：任一 type 失败即 `exit 1`。
- R16. 默认查询整体超时 5 秒，可通过 `--timeout` 调整。

## Success Criteria

- `jdan dns lookup example.com` 输出 6 个 type 的并发查询结果，总耗时 < 200ms（本地缓存命中场景）。
- `jdan dns lookup example.com -t A` 仅查询 A 记录。
- `jdan dns lookup example.com -t A,MX,TXT` 查询指定 3 个 type。
- `jdan dns lookup example.com --server 8.8.8.8` 使用指定 DNS server。
- `jdan dns lookup example.com --json | jq .` 合法 JSON 输出。
- `jdan dns lookup example.com --short -t A` 仅输出 IP 值，可用于 `IP=$(jdan dns lookup example.com --short -t A)`。
- `jdan dns lookup nonexistent-12345.invalid` 返回 `exit 1`，错误信息明确指出 NXDOMAIN。
- `go test ./internal/dnslookup/...` 全部通过（不含 integration build tag 的部分）。
- `go test -tags integration ./internal/dnslookup/...` 在网络可用时通过真实 DNS 烟雾测试。

## Scope Boundaries

- **NOT 支持 Windows（first release）**：first release 仅 macOS + Linux，Windows 的 resolver 自动检测（无 `/etc/resolv.conf`）留待未来需求。
- **NOT 支持 DNSSEC** 验证（首版无 `+dnssec`、无 RRSIG 检查）。
- **NOT 支持 DoH / DoT** 加密 DNS（首版仅明文 UDP/TCP）。
- **NOT 支持 reverse lookup**（PTR 查询）—— 留给后续 `jdan dns reverse <ip>` 子命令。
- **NOT 支持 trace** / `dig +trace` 风格的迭代查询 —— 留给后续 `jdan dns trace`。
- **NOT 支持 ANY 查询** —— RFC 8482 已建议废弃，多数权威服务器返回空。
- **NOT 支持配置文件**（无 `~/.jdanrc`、无 `--config`）—— 与 jdan 现有命令的"全部走 flag"风格一致。

## Key Decisions

- **解析器：纯 miekg/dns 路线**。`net.LookupX` 系列不支持 `--server`、无 TTL、缺少 CAA/SOA API；混合策略会让 `--json` 输出字段不稳定。miekg/dns 是 Go DNS 业界标准（CoreDNS、Prometheus blackbox_exporter 都在用），单一代码路径，metadata 完整。
- **命令空间：两级 `jdan dns lookup`**。与 `jdan http timing` / `jdan file bak` / `jdan obsidian install-claudian` 的两级动词族一致；DNS 是语义丰富的领域，未来 reverse / trace / sec 子功能无须重命名。
- **默认 6 type 并发查询**。一发命令完成日常诊断 99% 场景（IP、邮件路由、SPF/DKIM/DMARC、CNAME 链路、权威 NS），跟 `dig` 默认单 type 拉开差距。并发后总耗时 ≈ 最慢单 type。
- **默认 resolver 跟随 OS 配置**。诊断场景用户希望 jdan 结果与本地 curl / 浏览器一致；`--server 8.8.8.8` 零成本切换到"看真相"模式。
- **宽容 exit code + `--strict` opt-in**。DNS 查询本质是"取任一可用路径"，部分成功是常态而非错误；NXDOMAIN 全失败才是真错；严格脚本可 opt-in。
- **解析器接口化以支持低成本测试**。`Resolver` 接口抽象，生产用 miekg.Client，测试用 mock；与 `internal/httptiming/measure_test.go` 用 `http.RoundTripper` 接口 mock 的模式一致。
- **integration build tag 隔离真实 DNS 测试**。CI 默认仅跑 mock 单测；本地 `go test -tags integration` 验证真实贯通。

## Dependencies / Assumptions

- 新增依赖：`github.com/miekg/dns`（业界标准，单一依赖）。
- macOS / Linux 上 `/etc/resolv.conf` 存在且可读（绝大多数情况成立；不可读时 fallback 8.8.8.8）。
- 用户机器到查询 DNS server 的 UDP/TCP 53 端口可达（公司内网若封禁 53 端口，用户可用 `--server` 切到内网 DNS）。

## Outstanding Questions

### Resolve Before Planning

（无阻塞性问题，9 轮决策点已全部 lock；可直接进入规划）

## Next Steps

→ `docs/plans/2026-06-09-001-feat-dns-lookup-plan.md` 进行结构化实现规划
