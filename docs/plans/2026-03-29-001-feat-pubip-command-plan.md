---
title: feat: Add pubip4 and pubip6 subcommands
type: feat
status: active
date: 2026-03-29
origin: docs/brainstorms/2026-03-29-pubip-command-requirements.md
---

# Add pubip4 and pubip6 Subcommands

## Overview

新增两个独立子命令 `jdan pubip4` 和 `jdan pubip6`，分别查询并输出本机公网 IPv4 和 IPv6 地址。内部重试至多 3 次，全部失败后输出提示信息并报错退出。

## Problem Frame

用户需要快速确认自己的公网出口 IP，IPv4 和 IPv6 需分别查询。

## Requirements Trace

- R1. `jdan pubip4` 输出本机当前公网 IPv4 地址，纯文本，无其他修饰
- R2. `jdan pubip4` 若所有重试均失败，输出一行提示信息并以非零退出码退出
- R3. `jdan pubip6` 输出本机当前公网 IPv6 地址，纯文本，无其他修饰
- R4. `jdan pubip6` 若所有重试均失败，输出一行提示信息并以非零退出码退出
- R5. 内部重试至多 3 次
- R6. 每次失败时重新发起请求，不等待重试
- R7. 使用 ipify 服务：IPv4 → https://api.ipify.org，IPv6 → https://api6.ipify.org

## Scope Boundaries

- 无 JSON 输出选项
- 不做 IP 格式校验
- 不做历史记录或缓存
- 两命令相互独立，无共享状态

## Context & Research

### Relevant Code and Patterns

- `internal/cli/root.go` — root 命令定义，Viper 环境变量绑定模式
- `internal/cli/http_timing.go` — HTTP GET 请求模式（失败 warn + continue，最后统一处理）
- `internal/cli/zip.go` — 独立子命令文件结构，每个命令单独 `init()` 注册

### Patterns to Follow

- 子命令结构参考 `http_timing.go`：父命令 `httpCmd` + 子命令 `httpTimingCmd`，在子命令 `init()` 中 `httpCmd.AddCommand(httpTimingCmd)` 后 `rootCmd.AddCommand(httpCmd)`
- 失败处理参考 `http_timing.go`：循环内失败打 warn log，最终失败 `return error`
- zerolog 日志格式参考 `http_timing.go`：使用 `log.Warn().Err(err).Msg("...")`

## Key Technical Decisions

- **单文件实现**：所有 pubip 相关代码放在 `internal/cli/pubip.go`，包含父命令 `pubipCmd`、两个子命令及共享的重试逻辑
- **重试实现**：在命令 RunE 内循环至多 3 次，每次失败 log warn 后继续，不等待
- **退出码**：失败时 `fmt.Errorf(...)` 返回，非零退出由 cobra 默认处理

## Open Questions

### Resolved During Planning

- 使用 ipify 作为查询服务（见 origin doc R7）

## Implementation Units

- [ ] **Unit 1: Create `internal/cli/pubip.go` with pubip4 and pubip6 commands**

**Goal:** 实现 `jdan pubip4` 和 `jdan pubip6` 两个子命令，含 3 次重试逻辑

**Requirements:** R1, R2, R3, R4, R5, R6, R7

**Dependencies:** None

**Files:**
- Create: `internal/cli/pubip.go`
- Tests: `internal/cli/pubip_test.go`

**Approach:**
- 定义 `pubipCmd`（父命令，`Use: "pubip"`），定义 `pubip4Cmd` 和 `pubip6Cmd` 两个子命令
- `pubip4Cmd.RunE`：循环 3 次 GET https://api.ipify.org，成功则 `fmt.Print(resp)` 并返回 nil，失败 log warn 后继续循环
- `pubip6Cmd.RunE`：同上，但 URL 为 https://api6.ipify.org
- 3 次全部失败后 `return fmt.Errorf("无法获取 IPv4/IPv6 地址")`
- `init()` 中依次：`pubipCmd.AddCommand(pubip4Cmd)`、`pubipCmd.AddCommand(pubip6Cmd)`、`rootCmd.AddCommand(pubipCmd)`

**Patterns to follow:**
- `internal/cli/http_timing.go` — RunE 内循环 + 失败处理模式
- `internal/cli/zip.go` — 父子命令注册结构

**Test scenarios:**
- Happy path: `pubip4Cmd` 返回有效 IPv4 地址（mock http 服务或用真实服务）
- Happy path: `pubip6Cmd` 返回有效 IPv6 地址（mock http 服务或用真实服务）
- Error path: 网络完全不可达时 3 次重试后返回 error，退出码非零
- Error path: 服务返回非 200 时触发重试

**Verification:**
- `go build -o jdan ./cmd/jdan && ./jdan pubip4` 输出一个 IPv4 地址
- `go build -o jdan ./cmd/jdan && ./jdan pubip6` 输出一个 IPv6 地址
- `go test ./internal/cli/...` 通过

## System-Wide Impact

- **New entry points:** `jdan pubip4`，`jdan pubip6`，`jdan pubip`（帮助信息）
- **No other surfaces affected** — purely additive, no existing interfaces changed

## Risks & Dependencies

| Risk | Mitigation |
|------|------------|
| ipify 服务不可用 | 3 次重试后优雅失败，提示用户而非 panic |
| Go 标准库 net/http 超时 | 使用默认 client，不设超时；失败时重试 |

## Documentation / Operational Notes

- README.md 需新增 `jdan pubip4` 和 `jdan pubip6` 命令说明（参考 `jdan http timing` 格式）

## Sources & References

- **Origin document:** [docs/brainstorms/2026-03-29-pubip-command-requirements.md](../brainstorms/2026-03-29-pubip-command-requirements.md)
- ipify: https://api.ipify.org, https://api6.ipify.org
