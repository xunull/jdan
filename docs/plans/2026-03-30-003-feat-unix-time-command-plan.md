---
title: "feat: Add unix-time command for timestamp conversion"
type: feat
status: active
date: 2026-03-30
origin: docs/brainstorms/2026-03-30-unix-time-requirements.md
---

# feat: Add unix-time command for timestamp conversion

## Overview

新增 `jdan unix-time` 子命令，将 Unix 时间戳（秒或毫秒）转换为本地时区可读时间字符串。命令支持参数输入和 stdin 单值输入，满足命令行直用和管道场景。

## Problem Frame

日志与排障场景中经常遇到时间戳数值，当前仓库缺少一个“即时可读化”命令。该功能是低复杂度、高频价值的工具增强，目标是减少切换工具与手工换算成本（see origin: `docs/brainstorms/2026-03-30-unix-time-requirements.md`）。

## Requirements Trace

- R1. 新增子命令 `jdan unix-time`
- R2. 参数优先，无参数时从 stdin 读取单值
- R3. 非数字输入报错并非零退出
- R4. 10 位按秒、13 位按毫秒
- R5. 越界/非法格式返回可读错误
- R6. 默认本地时区输出
- R7. 简洁单行输出，无标签
- R8. 成功时仅输出结果

## Scope Boundaries

- 不做批量多值转换
- 不做反向转换
- 不做自定义输出模板

## Context & Research

### Relevant Code and Patterns

- `internal/cli/file_bak.go`：Cobra 命令定义与 `RunE` 风格
- `internal/cli/pubip.go`：扁平子命令注册到 `rootCmd` 的模式
- `internal/filebak/name_test.go`：表格驱动测试写法

### Institutional Learnings

- 当前仓库无 `docs/solutions/` 可复用知识条目

### External References

- 本需求仅依赖 Go 标准库 `time`、`strconv`、`strings`，无需外部框架研究

## Key Technical Decisions

- **参数+stdin 双输入**：参数优先，保持 CLI 直觉并兼容管道。
- **10/13 位识别策略**：覆盖最常见秒/毫秒来源，避免额外 `--unit` 心智负担。
- **解析逻辑放在 `internal/unixtime`**：保持 CLI 层薄，延续仓库“命令层 + 业务层”分离模式。

## Open Questions

### Resolved During Planning

- **格式输出布局**：采用固定布局 `2006-01-02 15:04:05 -07:00`，满足“简洁单行”的需求。
- **stdin 处理**：读取并 `TrimSpace` 后解析；空输入返回明确错误。

### Deferred to Implementation

- 无

## Implementation Units

- [ ] **Unit 1: 时间戳解析与格式化业务层**

**Goal:** 在独立包中实现“输入校验 + 秒/毫秒识别 + 本地时区格式化”核心逻辑。

**Requirements:** R3, R4, R5, R6, R7, R8

**Dependencies:** None

**Files:**
- Create: `internal/unixtime/convert.go`
- Test: `internal/unixtime/convert_test.go`

**Approach:**
- 提供函数（命名可在实现时定）：输入 `string`，输出格式化时间 `string` 或 `error`
- 仅接受纯数字；长度 10 视为秒，长度 13 视为毫秒
- 使用 `time.Unix(...).In(time.Local)` 进行本地时区转换
- 使用固定 layout 输出：`2006-01-02 15:04:05 -07:00`

**Patterns to follow:**
- `internal/filebak/name_test.go` 的表格驱动测试风格

**Test scenarios:**
- Happy path: 10 位秒时间戳正确输出本地时区时间
- Happy path: 13 位毫秒时间戳正确输出本地时区时间
- Edge case: 输入前后空白被 trim 后仍可正确解析
- Error path: 非数字输入返回错误
- Error path: 非 10/13 位输入返回错误
- Error path: 空字符串输入返回错误

**Verification:**
- `internal/unixtime` 单测全部通过，转换结果与预期一致

- [ ] **Unit 2: CLI 子命令接入**

**Goal:** 新增 `jdan unix-time` 命令并接入根命令。

**Requirements:** R1, R2, R8

**Dependencies:** Unit 1

**Files:**
- Create: `internal/cli/unix_time.go`
- Modify: `README.md`

**Approach:**
- 定义扁平命令 `unix-time [timestamp]`
- `Args` 允许 0 或 1 个参数；>1 报错
- 有参数时直接用参数；无参数时读取 stdin 一次
- 调用 `internal/unixtime` 业务函数，成功仅 `fmt.Print(result)`，失败返回 error

**Patterns to follow:**
- `internal/cli/pubip.go` 的 root 注册模式
- `internal/cli/file_bak.go` 的 `RunE` 错误传播模式

**Test scenarios:**
- Happy path: 参数输入成功输出一行结果
- Happy path: stdin 输入成功输出一行结果
- Error path: 提供 2 个参数时报错
- Error path: 无参数且 stdin 为空时报错

**Verification:**
- `jdan --help` 中出现 `unix-time`
- `jdan unix-time --help` 用法清晰

- [ ] **Unit 3: 文档与回归验证**

**Goal:** 完成文档更新并验证全仓测试与构建通过。

**Requirements:** R1, R2, R7

**Dependencies:** Unit 1, Unit 2

**Files:**
- Modify: `README.md`

**Approach:**
- README 增加 `jdan unix-time` 章节：示例（参数与管道）
- 执行全量测试与构建，确保无回归

**Patterns to follow:**
- README 现有命令章节风格（如 `http timing`、`pubip4/pubip6`）

**Test scenarios:**
- Integration: `echo 1711843200000 | jdan unix-time` 输出单行可读时间
- Integration: `jdan unix-time 1711843200` 输出单行可读时间
- Error path: `jdan unix-time abc` 返回错误与非零退出

**Verification:**
- `go test ./...` 通过
- `go build -o jdan ./cmd/jdan` 成功

## System-Wide Impact

- 仅新增一个独立命令，不影响既有命令行为
- 无新增第三方依赖，构建与发布风险低
- 保持 CLI 层薄、业务层可测的既有架构约定

## Risks & Dependencies

| Risk | Mitigation |
|------|------------|
| 秒/毫秒识别误判导致时间错误 | 严格限制只接受 10/13 位，其他长度直接报错 |
| stdin 空输入或换行处理不一致 | 统一 `TrimSpace`，空值显式报错并加测试覆盖 |
| 本地时区导致跨机器输出差异 | 文档明确“按本地时区输出”，测试中避免写死带时区的绝对字符串（或固定 `time.Local`） |

## Documentation / Operational Notes

- README 增加命令说明与示例
- 该功能无运行时外部依赖、无部署迁移要求

## Sources & References

- **Origin document:** [`docs/brainstorms/2026-03-30-unix-time-requirements.md`](../brainstorms/2026-03-30-unix-time-requirements.md)
- Related code: `internal/cli/file_bak.go`, `internal/cli/pubip.go`, `internal/filebak/name_test.go`
