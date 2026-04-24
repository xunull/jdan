---
title: "feat: Add jdan ports command"
type: feat
status: active
date: 2026-04-09
origin: docs/brainstorms/2026-04-09-listening-ports-requirements.md
---

# feat: Add jdan ports command

## Overview

新增 `jdan ports` 子命令，通过调用 `lsof` 获取本机所有处于 LISTEN 状态的 TCP/UDP 端口，支持表格和 JSON 两种输出格式。

## Problem Frame

开发者排查网络问题时需要快速确认本机监听端口和占用进程。现有 `lsof`/`netstat` 输出格式不一，部分需要 sudo，且不易于脚本处理。

## Requirements Trace

- R1. 新增子命令 `jdan ports`
- R2. 默认表格输出，支持 `--json`（`-j`）切换 JSON
- R3. 表格按协议分块（TCP 在前，UDP 在后），JSON 为单一数组
- R4. 每条记录包含：协议、监听地址、进程名
- R5. 进程名通过 `lsof -i -P -n` 获取，权限不足时显示 `-`
- R6. 按端口号升序排列
- R7. 支持 `--tcp`（`-t`）和 `--udp`（`-u`）过滤

## Scope Boundaries

- 仅 macOS（Linux 扩展不在本次范围）
- 仅 LISTEN 状态
- 无 PID 列（进程名已足够）
- 不扫描 Docker 容器内部（仅宿主机已映射端口）

## Context & Research

### Relevant Code and Patterns

- `internal/cli/pubip.go` — 最简子命令模式参照
- `internal/cli/http_timing.go` — JSON/文本双模式输出参照
- `internal/cli/root.go` — cobra 命令注册和 flag 绑定模式

### Technology Stack

- Go 1.25 / cobra v1.10.2 / viper v1.21.0

## Key Technical Decisions

- **`lsof -i -P -n`**：macOS 内置工具，`-i` 查网络文件，`-P` 抑制端口号转换，`-n` 抑制 hostname 解析，`-sTCP:LISTEN` 过滤 LISTEN 状态；无需引入外部库
- **表格分块输出**：先 TCP 后 UDP，每块内按端口号排序，使用 `tabwriter` 格式化
- **JSON 结构**：`[{protocol, address, port, process}, ...]`，统一数组格式

## Implementation Units

- [ ] **Unit 1: Create `internal/cli/ports.go` with command scaffold**

**Goal:** 建立子命令框架和 flag 定义

**Requirements:** R1, R2, R7

**Dependencies:** None

**Files:**
- Create: `internal/cli/ports.go`
- Test: `internal/cli/ports_test.go`（可选，轻量级命令不强制）

**Approach:**
- 参照 `pubip.go` 的文件结构
- 定义 `--json`（`-j`）、`--tcp`（`-t`）、`--udp`（`-u`）三个 flag
- 表格输出使用 `text/tabwriter`

**Patterns to follow:**
- `internal/cli/pubip.go` — 简单子命令结构
- `internal/cli/http_timing.go` — JSON flag 和双模式输出分支

**Test scenarios:**
- Happy path: 运行 `jdan ports -h` 显示正确的帮助信息，3 个 flag 均存在

**Verification:**
- `go build -o jdan . && ./jdan ports -h` 输出包含 `--json`, `--tcp`, `--udp` flag 说明

---

- [ ] **Unit 2: Implement `lsof` 调用和结果解析**

**Goal:** 通过 `lsof` 获取端口数据并解析为结构化数据

**Requirements:** R4, R5, R6

**Dependencies:** Unit 1

**Files:**
- Create: `internal/ports/collector.go`（lsof 调用和解析逻辑）
- Modify: `internal/cli/ports.go`（集成 collector）

**Approach:**
- `lsof -i -P -n -sTCP:LISTEN` 获取 TCP 监听端口
- `lsof -i -P -n -sUDP:LISTEN` 获取 UDP 监听端口
- 解析 `lsof` 输出行：`进程名|协议|地址|端口|状态`（lsof 输出格式以空格分列，按位置取值）
- 合并结果后按端口号排序
- 权限不足时进程名记为 `-`

**Technical design**（directional guidance）:
```
// 数据结构
type PortEntry struct {
    Protocol string // "TCP" or "UDP"
    Address  string // e.g. "127.0.0.1", "*", "[::1]"
    Port     int
    Process  string // 进程名，权限不足时为 "-"
}

// lsof 输出行示例
// COMMAND  PID   USER   FD   TYPE   DEVICE  SIZE/OFF   NODE   NAME
// nginx    123   root   6u   IPv4   0x...   0t0       TCP    *:80 (LISTEN)

// 解析策略：
// - NAME 列格式: "*:80 (LISTEN)" 或 "127.0.0.1:8080 (LISTEN)" 或 "[::1]:22 (LISTEN)"
// - 用正则提取地址和端口，忽略 (LISTEN) 后缀
```

**Patterns to follow:**
- `internal/macgpu/collector.go` — 子进程调用模式（但 ports 更简单，无需持续采样）

**Test scenarios:**
- Happy path: 在有监听端口的机器上运行，返回非空切片
- Edge case: 无任何监听端口时返回空切片，不报错
- Edge case: `lsof` 命令不存在时返回有意义的错误

**Verification:**
- 运行 `go build -o jdan . && ./jdan ports` 输出包含正在监听的端口（如 22、80 等系统常见端口）

---

- [ ] **Unit 3: 实现表格和 JSON 双模式输出**

**Goal:** 完成 `jdan ports` 的完整用户体验

**Requirements:** R2, R3

**Dependencies:** Unit 1 + Unit 2

**Files:**
- Modify: `internal/cli/ports.go`

**Approach:**
- 表格模式：使用 `text/tabwriter` 对齐列，格式 `PROTOCOL  ADDRESS              PROCESS`
- JSON 模式：`encoding/json` 序列化 `PortEntry` 数组
- `--tcp` / `--udp` flag 控制调用哪组 lsof（都显示则串行调用两次）

**Patterns to follow:**
- `internal/cli/http_timing.go` — JSON/text 分支输出模式

**Test scenarios:**
- Happy path: `jdan ports` 输出格式化为对齐的表格
- Happy path: `jdan ports --json` 输出合法 JSON 数组
- Happy path: `jdan ports --tcp` 仅显示 TCP 端口
- Happy path: `jdan ports --udp` 仅显示 UDP 端口

**Verification:**
- `jdan ports | grep -E "TCP|UDP"` 可见协议列
- `jdan ports --json | jq .` 无报错退出

---

- [ ] **Unit 4: 命令注册**

**Goal:** 将 `ports` 子命令注册到 root 命令

**Requirements:** R1

**Dependencies:** Unit 1

**Files:**
- Modify: `internal/cli/ports.go`（在 `init()` 中执行 `rootCmd.AddCommand(portsCmd)`）

**Approach:**
- 在 `init()` 中添加 `rootCmd.AddCommand(portsCmd)`，参照 `mac_gpu.go` 的 `rootCmd.AddCommand(macgpuCmd)` 模式

**Verification:**
- `go build -o jdan . && ./jdan ports -h` 正常输出帮助
- `./jdan help` 列表中包含 `ports` 命令

## System-Wide Impact

- **新增依赖**：无（纯用 `os/exec` 调用 `lsof`）
- **对其他命令的影响**：无
- **CLI 表面**：新增 `jdan ports` 命令和 `--json`/`--tcp`/`--udp` 三个 flag

## Risks & Dependencies

| Risk | Mitigation |
|------|------------|
| `lsof` 输出格式在 macOS 版本间可能略有差异 | 使用位置解析 + 正则兼容 `(LISTEN)` 后缀 |
| `lsof` 执行慢（大规模服务器） | 仅获取 LISTEN 状态，已经过 `-sTCP:LISTEN` 过滤，实践中足够快 |

## Documentation / Operational Notes

- README.md 如有命令列表，需新增 `jdan ports` 条目

## Sources & References

- **Origin document:** [docs/brainstorms/2026-04-09-listening-ports-requirements.md](../brainstorms/2026-04-09-listening-ports-requirements.md)
- `lsof -i` man page（macOS 内置）
- `internal/cli/pubip.go` — 简单子命令模式
- `internal/cli/http_timing.go` — JSON 双模式输出
