---
title: "feat: Add jdan macgpu — Apple Silicon GPU TUI Monitor"
type: feat
status: active
date: 2026-03-30
origin: docs/brainstorms/2026-03-30-mac-gpu-monitor-requirements.md
---

# feat: Add jdan macgpu — Apple Silicon GPU TUI Monitor

## Overview

新增 `jdan macgpu` 子命令，在 Apple Silicon Mac 上通过调用 `sudo powermetrics` 实时采集 GPU 指标，以 htop/glances 风格的 TUI 界面展示：顶部三条带颜色的 ASCII 柱状图（GPU 占用率、功耗、频率），底部详情表格，每 2 秒自动刷新。

## Problem Frame

开发者在终端工作流中缺乏直观的 Apple Silicon GPU 实时监控工具。Activity Monitor 无法在终端使用；`powermetrics` 原始输出不可读。本功能将 `jdan macgpu` 定位为终端 GPU 仪表盘，类似 htop 对 CPU 的定位。

（见原始需求文档：docs/brainstorms/2026-03-30-mac-gpu-monitor-requirements.md）

## Requirements Trace

- R1. `jdan macgpu` 扁平子命令，仅支持 arm64；非 Apple Silicon 平台有明确错误提示并以非零退出码退出
- R2. 需要 sudo 权限；以非 root 身份运行时输出提示并退出
- R3. 调用 `sudo powermetrics --samplers gpu_power,thermal --format plist -i <ms> -n 0`，plist 格式以 NUL 字节分隔
- R4. 默认采样间隔 2000ms，`--interval`/`-i` 可自定义（最小 500ms）
- R5. 采集：GPU 活跃占用率（`1 - idle_ratio`）、GPU 频率（`gpu.freq_hz` 单位 MHz）、GPU 功耗（`processor.gpu_power` 单位 mW）、散热压力等级
- R6. 持续刷新 TUI，`q`/`Ctrl+C` 干净退出
- R7. 顶部三条带颜色柱状图（GPU 占用率、功耗、频率）；颜色阈值：绿→黄→红
- R8. 底部表格展示所有指标
- R9. 使用 Bubble Tea v2 + Lip Gloss v2
- R10. 标题栏含命令名、采样间隔、当前时间戳
- R11. powermetrics 调用失败时输出错误并退出，不崩溃
- R12. 日志使用 zerolog，TUI 运行时 zerolog 写入临时文件（stdout 被 Bubble Tea 接管）

## Scope Boundaries

- 不支持 Intel Mac 或 eGPU
- 不支持 JSON 输出（纯 TUI 工具）
- 不监控 CPU/内存等非 GPU 指标
- 不持久化历史数据
- 不进行进程级 GPU 占用分析
- 温度展示为散热压力等级字符串，不展示摄氏度（powermetrics thermal 采样器在 Apple Silicon 上不提供摄氏度）

## Context & Research

### Relevant Code and Patterns

- `internal/cli/pubip.go` — 扁平子命令注册到 rootCmd 的参考模式（`init()` + `rootCmd.AddCommand()`）
- `internal/cli/http_timing.go` — 处理 `--interval`/`-n` 等参数的参考，`cmd.Flags().GetInt()` 读取方式
- `cmd/jdan/main.go` — zerolog `ConsoleWriter` 初始化位置；TUI 模式下需要覆盖 log output 目标
- `internal/httptiming/measure.go` — 业务逻辑与 CLI 层分离的结构参考（纯函数 + 返回 error）
- `internal/filebak/name_test.go` — 表格驱动测试模式参考

### External References

- Bubble Tea v2：`charm.land/bubbletea/v2 v2.0.2`（2026-03-06 发布；`View()` 返回 `tea.View` 而非 `string`；键盘消息改为 `tea.KeyPressMsg`；启动方法改为 `p.Run()`）
- Lip Gloss v2：`charm.land/lipgloss/v2 v2.0.2`（2026-03-11 发布；`Blend1D` 生成颜色渐变数组；`LightDark(hasDark)` 处理暗/亮终端适配）
- plist 解析：`howett.net/plist`（推荐库，支持 Apple XML plist 反序列化，stable API）
- powermetrics 格式：仅支持 `text` 和 `plist`，无 `json` 格式；plist 格式 NUL 分隔连续采样
- `gpu.freq_hz` 字段命名陷阱：GPU 的此字段实际单位是 **MHz**（非 Hz），与 CPU cluster 的同名字段（真实 Hz，需 ÷1e6）行为不同；这是 Apple 已知命名不一致
- M1~M4 系列 GPU plist 字段名一致（`gpu.freq_hz`、`gpu.idle_ratio`、`processor.gpu_power`）；差异仅在 dvfm_states 档位数量（M1 约 6 档，M4 Pro 约 15 档）
- 温度：Apple Silicon `thermal` 采样器只返回 5 级枚举 `thermal_pressure` 字符串，不返回摄氏度

## Key Technical Decisions

- **选 plist 格式而非 text 格式**：`powermetrics` 无 JSON 格式，plist 字段结构稳定（已验证 M1~M4），Go 有成熟的 plist 解析库（`howett.net/plist`）；text 格式需正则解析，版本间格式差异大
- **选 Bubble Tea v2（`charm.land/bubbletea/v2`）而非 v1**：v2 为 2026 年 2 月发布的当前稳定版，v1 路径已停止维护；v2 API 中 `View()` 返回 `tea.View`，`KeyPressMsg` 替代 `KeyMsg`，`p.Run()` 替代 `p.Start()`——规划已按 v2 API 设计
- **Collector 用 goroutine + channel 推送**：powermetrics 是流式连续输出（-n 0 无限采样），天然适合 goroutine 逐 NUL 块扫描后推送到 Bubble Tea 的 `p.Send()`；优于 Tick 轮询（Tick 无法流式接收子进程输出）
- **TUI 期间 zerolog 写入临时文件**：Bubble Tea v2 仍默认占用 stdout；zerolog 的 ConsoleWriter 若继续写 stdout 会破坏 TUI 渲染。在 macgpuCmd 的 RunE 中，TUI 启动前将 `log.Logger` 重定向到 `os.CreateTemp`；TUI 退出后 log 恢复（或保持写文件）
- **平台检查放 RunE 最早处**：`runtime.GOARCH != "arm64"` 在 RunE 入口返回 error，不影响任何其他命令的 `PersistentPreRunE` 执行

## Open Questions

### Resolved During Planning

- **powermetrics 格式**：外部研究确认无 JSON 格式，改用 `--format plist`（已更新需求文档 R3）
- **温度字段**：thermal 采样器在 Apple Silicon 上仅提供 5 级枚举字符串（`thermal_pressure`），无摄氏度；展示为散热压力等级（已更新需求文档 R5）
- **Bubble Tea 版本**：v2（`charm.land/bubbletea/v2 v2.0.2`）为当前稳定版，规划按 v2 API
- **`gpu.freq_hz` 单位**：GPU 该字段直接是 MHz，不需除以 1e6（与 CPU cluster 同名字段行为不同）

### Deferred to Implementation

- **powermetrics 功耗单位（mW vs W）**：较新芯片（M4 系列）在部分 macOS 版本下功率可能为 mW，旧芯片可能为 W；实现时通过实机采样确认，并在 `GPUSnapshot` 中统一用 mW（如有必要做转换）
- **柱状图颜色阈值**：占用率/功耗/频率各自的绿→黄→红切换百分比，留给实现阶段根据实测数据调整
- **功耗柱状图最大值**：需以该 SoC 的 TDP 为参考设定满格值；作为可配置常量留给实现阶段
- **`tea.WithContext` vs 手动 kill**：两种子进程退出策略的选择留给实现阶段（均可工作，无产品影响）

## High-Level Technical Design

> *以下示意图为方向性指导，供审阅验证思路，不作为实现规范。*

```
┌─ macgpuCmd.RunE ──────────────────────────────────────┐
│  1. arm64 & sudo 检查（失败则 return error）            │
│  2. 将 log.Logger 重定向到临时文件                      │
│  3. 构造 Collector（ctx + interval）                    │
│  4. 构造 tui.Model（含 Collector 引用）                 │
│  5. tea.NewProgram(model).Run()                         │
│  6. 返回后 cancel() 确保 Collector goroutine 退出       │
└───────────────────────────────────────────────────────┘
          │
          ▼ goroutine
┌─ Collector ──────────────────────────────────────────────┐
│  exec.CommandContext → powermetrics --format plist -n 0  │
│  bufio.Scanner（NUL 分隔）                                │
│  for scanner.Scan():                                       │
│      sample = ParseSample(bytes)                           │
│      p.Send(SampleMsg{sample})   ──────────────────────►  │
│  context.Done() → 退出循环，cmd 自动 Kill                  │
└──────────────────────────────────────────────────────────┘
          │                              │
          │   SampleMsg                  ▼
          │                   ┌─ tui.Model.Update ─────────┐
          │                   │  case SampleMsg → m.latest  │
          │                   │  case WindowSizeMsg → 调整宽度│
          │                   │  case KeyPressMsg "q" → Quit │
          │                   └────────────────────────────┘
          │                              │
          │                              ▼
          │                   ┌─ tui.Model.View ───────────┐
          │                   │  标题栏（命令名/时间/间隔）   │
          │                   │  柱状图 × 3（占用率/功耗/频率）│
          │                   │  详情表格                    │
          │                   └────────────────────────────┘
```

## Implementation Units

---

- [ ] **Unit 1: plist 解析层**

**Goal:** 定义 powermetrics plist 输出的 Go 结构体，实现 `ParseSample([]byte) (*GPUSample, error)` 函数，将 NUL 分隔的 plist 块转换为可用的 Go 值对象。

**Requirements:** R3, R5

**Dependencies:** 无

**Files:**
- 新建：`internal/macgpu/metrics.go`
- 新建：`internal/macgpu/metrics_test.go`

**Approach:**
- 定义 `PowerMetricsSample`（顶层）、`ProcessorStats`、`GPUStats`、`DVFMState` 结构体，使用 `plist:"field_name"` 标签
- `GPUStats.ActiveResidency()` 方法返回 `1.0 - IdleRatio`（active 无直接字段，需计算）
- `ParseSample` 使用 `howett.net/plist` 库：`plist.NewDecoder(bytes.NewReader(data)).Decode(&sample)`，API 与 `encoding/json` 风格一致；字段不存在时返回零值（兼容不同芯片/OS 版本）
- 从 `PowerMetricsSample` 提取一个扁平化的 `GPUSnapshot` 供 TUI 层使用（避免 TUI 直接依赖 plist 结构细节）：包含 `ActiveResidency float64`、`FreqMHz float64`、`PowerMW float64`、`ThermalPressure string`、`Timestamp time.Time`

**Patterns to follow:**
- 表格驱动测试：参考 `internal/filebak/name_test.go` 的 `[]struct{...}` 模式
- 错误返回向上传递：不在此层调用 `log`

**Test scenarios:**
- Happy path：给定一段真实 M1 plist fixture bytes，`ParseSample` 返回正确的 `GPUSnapshot`（`ActiveResidency = 1 - idle_ratio`，`FreqMHz` 与 plist 中 `gpu.freq_hz` 值相等，`PowerMW` 与 `processor.gpu_power` 相等）
- Edge case：`gpu.idle_ratio = 1.0`（GPU 完全空闲）→ `ActiveResidency = 0.0`，不返回负值
- Edge case：`gpu.idle_ratio = 0.0`（GPU 满载）→ `ActiveResidency = 1.0`
- Edge case：plist 中 `thermal_pressure` 字段缺失 → `ThermalPressure` 为空字符串，不 panic
- Error path：输入不合法的 bytes（非 plist 格式）→ 返回非 nil 错误
- Edge case：`dvfm_states` 为空数组（极少数情况）→ 不 panic，正常返回

**Verification:**
- `go test ./internal/macgpu/` 全部通过，无需实机

---

- [ ] **Unit 2: Collector（powermetrics 子进程管理）**

**Goal:** 实现 `Collector` 类型，负责启动/管理 powermetrics 子进程，逐 NUL 块扫描 plist 输出，通过 `p.Send()` 向 Bubble Tea 程序推送 `SampleMsg`；context 取消时干净退出并 kill 子进程。

**Requirements:** R2, R3, R4, R11

**Dependencies:** Unit 1（`ParseSample`）

**Files:**
- 新建：`internal/macgpu/collector.go`
- 新建：`internal/macgpu/collector_test.go`

**Approach:**
- `Collector` 持有 `ctx context.Context`、`interval int`（ms）、`program *tea.Program`
- `Collector.Start()` 在新 goroutine 中运行：`exec.CommandContext(ctx, "powermetrics", "--samplers", "gpu_power,thermal", "--format", "plist", "-i", strconv.Itoa(interval), "-n", "0")`
- 自定义 `bufio.ScanFunc` 按 NUL 字节（`0x00`）分割输出流
- 每个完整 plist 块：调用 `ParseSample` → 成功则 `p.Send(SampleMsg{...})`；失败则 `p.Send(ErrMsg{...})`
- `context.Done()` 时退出循环，`exec.CommandContext` 自动向子进程发送 kill 信号
- `powermetrics` 启动失败（命令不存在、权限不足）在 `Start()` 内捕获，通过 `p.Send(ErrMsg{...})` 通知 TUI

**Patterns to follow:**
- `exec.CommandContext` 的生命周期管理参考 Go 标准库文档
- goroutine + channel 推送：参考研究报告中的"方式 B"

**Test scenarios:**
- Happy path：用一个 fake powermetrics 脚本（输出已知 plist + `\x00`），Collector 能正确解析并推送 `SampleMsg`
- Error path：fake 脚本退出码非 0 → 推送 `ErrMsg`，不 panic
- Error path：fake 脚本输出不合法 plist → 推送 `ErrMsg`，Collector 继续运行（不因单次解析失败而退出）
- Edge case：context 在第一条数据到达前被取消 → goroutine 干净退出，不 deadlock

**Verification:**
- `go test ./internal/macgpu/` 通过（fake 子进程通过测试 fixture 模拟，无需 sudo）

---

- [ ] **Unit 3: TUI Model（Bubble Tea v2）**

**Goal:** 实现 Bubble Tea v2 的 `Model`、`Init`、`Update`、`View`，展示 GPU 仪表盘：顶部标题栏 + 三条 lipgloss 柱状图（占用率/功耗/频率）+ 底部详情表格；响应终端窗口大小变化动态调整布局。

**Requirements:** R6, R7, R8, R9, R10

**Dependencies:** Unit 1（`GPUSnapshot`）、Unit 2（`SampleMsg`, `ErrMsg`）

**Files:**
- 新建：`internal/macgpu/tui.go`

**Approach:**
- 使用 `charm.land/bubbletea/v2` 和 `charm.land/lipgloss/v2`（新域名，v2 API）
- `Model` 字段：`width int`、`height int`、`ready bool`、`latest *GPUSnapshot`、`err error`、`interval int`、`cancel context.CancelFunc`
- `Init()`：返回 `nil`（Collector 已由 CLI 层在 `p.Send` goroutine 中独立运行）
- `Update()` 处理：
  - `tea.WindowSizeMsg` → 更新 `m.width`/`m.height`/`m.ready`，动态计算 `barWidth = m.width - 28`（最小 10）
  - `SampleMsg` → 更新 `m.latest`
  - `ErrMsg` → 更新 `m.err`，TUI 继续运行（底部显示错误，不强制退出）
  - `tea.KeyPressMsg "q"` / `"ctrl+c"` → 调用 `m.cancel()`，返回 `tea.Quit`
- `View()` 返回 `tea.NewView(...)` 渲染结果（v2 API）：
  - 标题栏：`"jdan macgpu  interval: Xms  <timestamp>"`，lipgloss Bold + 颜色
  - 3 条水平柱状图（`renderBar` 函数），颜色阈值：`< 0.6 → 绿`、`0.6~0.85 → 黄`、`> 0.85 → 红`，使用 `lipgloss.Blend1D(100, ...)` 生成渐变色板
  - 底部：详情表格（占用率%/频率 MHz/功耗 mW/散热压力等级/采样时间戳）
  - 如 `m.err != nil`：底部显示红色错误行
  - 如 `!m.ready`：显示 "Initializing..."
- 柱状图宽度随终端宽度自适应（`tea.WindowSizeMsg` 时更新）

**Technical design:** *(方向性指导)*
```
// View() 结构（伪代码）
titleBar := lipgloss.NewStyle().Bold(true).Render(...)
bar1 := renderBar("GPU Util  ", m.latest.ActiveResidency, m.barWidth)
bar2 := renderBar("GPU Power ", normalizePower(m.latest.PowerMW), m.barWidth)
bar3 := renderBar("GPU Freq  ", normalizeFreq(m.latest.FreqMHz), m.barWidth)
table := renderDetailTable(m.latest)
return tea.NewView(strings.Join([]string{titleBar, bar1, bar2, bar3, "", table}, "\n"))
```
`normalizePower` / `normalizeFreq` 将绝对值映射到 0.0~1.0（基于预设最大值常量）

**Patterns to follow:**
- `charm.land/bubbletea/v2` v2 API：`View()` 返回 `tea.View`，`KeyPressMsg` 而非 `KeyMsg`
- `charm.land/lipgloss/v2`：`lipgloss.Blend1D` 颜色渐变，`lipgloss.LightDark` 暗/亮终端适配

**Test scenarios:**
- 无 TUI 的单元测试不适合此层；主要通过实机运行验证渲染结果
- 如需自动化：可对 `renderBar(label, pct, width)` 纯函数单独测试：`pct=0` 全空格、`pct=1.0` 全填充、`pct=0.5` 半填充（字符数正确）

**Verification:**
- `sudo jdan macgpu` 启动后进入 TUI，柱状图随 GPU 负载更新，按 `q` 干净退出（无残留 ANSI）
- 调整终端窗口大小，柱状图宽度自适应

---

- [ ] **Unit 4: CLI 命令注册**

**Goal:** 在 `internal/cli/mac_gpu.go` 中定义并注册 `macgpuCmd`，处理平台/权限检查、`--interval` flag，设置 zerolog 临时文件重定向，组装 Collector + TUI 并启动。

**Requirements:** R1, R2, R4, R11, R12

**Dependencies:** Unit 2（Collector）、Unit 3（tui.Model）

**Files:**
- 新建：`internal/cli/mac_gpu.go`

**Approach:**
- `init()` 中注册：`rootCmd.AddCommand(macgpuCmd)`；`macgpuCmd.Flags().IntVarP(&interval, "interval", "i", 2000, "采样间隔（ms，最小 500）")`
- `RunE` 顺序：
  1. `runtime.GOARCH != "arm64"` → `return fmt.Errorf("jdan macgpu 仅支持 Apple Silicon (arm64) Mac")`
  2. `os.Getuid() != 0` → `return fmt.Errorf("请使用 sudo 运行此命令")`（不用 `log.Fatal`，让 cobra 打印 error 后以非零码退出）
  3. 校验 `interval >= 500`，否则 `return error`
  4. 将 `log.Logger` 重定向到 `os.CreateTemp("", "jdan-macgpu-*.log")` （TUI 期间）
  5. `ctx, cancel := context.WithCancel(context.Background())`
  6. 构造 TUI model（含 cancel），创建 `tea.NewProgram(model, tea.WithAltScreen())`
  7. 构造 `Collector{ctx, interval, program}`，调用 `collector.Start()`
  8. `program.Run()`，等待退出
  9. `cancel()`（确保 Collector 退出），返回可能的 error
- 不修改 `root.go`

**Patterns to follow:**
- `internal/cli/pubip.go` — 扁平命令注册方式（`init()` + `rootCmd.AddCommand()`）
- `internal/cli/http_timing.go` — `cmd.Flags().GetInt()` + 参数校验模式

**Test scenarios:**
- 此层无业务逻辑可单元测试；通过实机运行验证：
  - 非 arm64 平台（或构建为 amd64）→ 打印错误，非零退出
  - 非 root 执行 → 打印"请使用 sudo 运行此命令"，非零退出
  - `sudo jdan macgpu -i 300` → 打印"间隔不得小于 500ms"，非零退出
  - `sudo jdan macgpu` → 正常进入 TUI

**Verification:**
- `jdan macgpu --help` 展示正确的用法说明和 `--interval`/`-i` 参数
- 上述实机测试用例均符合预期

---

- [ ] **Unit 5: 依赖与文档更新**

**Goal:** 添加 Bubble Tea v2、Lip Gloss v2、howett.net/plist 依赖至 go.mod；在 README.md 补充 `jdan macgpu` 命令文档。

**Requirements:** R9

**Dependencies:** Unit 3（确认依赖后添加）

**Files:**
- 修改：`go.mod`、`go.sum`（通过 `go get` 命令更新）
- 修改：`README.md`

**Approach:**
- 运行：
  ```
  go get charm.land/bubbletea/v2@latest
  go get charm.land/lipgloss/v2@latest
  go get howett.net/plist@latest
  go mod tidy
  ```
- README 新增 `### jdan macgpu` 章节，格式与现有 `### jdan http timing` 保持一致：功能描述 + 用法示例 + 参数表格；注明需要 sudo

**Verification:**
- `go build ./...` 成功，无编译错误
- `GOWORK=off go build -o jdan ./cmd/jdan` 成功

---

## System-Wide Impact

- **Cobra 注册**：`mac_gpu.go` 的 `init()` 自动将 `macgpuCmd` 注册到 `rootCmd`，不影响其他命令；`jdan --help` 输出将增加 `macgpu` 条目
- **zerolog 全局 logger**：TUI 期间重定向 `log.Logger` 为文件输出；若其他命令的 `PersistentPreRunE` 副作用使全局 logger 在 TUI 期间被重置，需确保重定向在 TUI 启动后发生——当前 `PersistentPreRunE` 不涉及 logger，安全
- **go.mod 新依赖**：Bubble Tea v2 + Lip Gloss v2 + howett.net/plist 为新引入；与现有依赖（zerolog、cobra、viper）无冲突
- **平台限制**：`runtime.GOARCH` 检查在 RunE 中进行（运行时），而非构建时；二进制可在 amd64 上编译，运行时报错——这与现有命令行为一致
- **Unchanged invariants**：现有命令（file bak、http timing、pubip、zip）不受影响，root.go 不修改

## Risks & Dependencies

| Risk | Mitigation |
|------|------------|
| `powermetrics` plist 字段名在未来 macOS 版本变化 | `ParseSample` 对缺失字段返回零值而非错误；`GPUSnapshot` 的关键字段（ActiveResidency/FreqMHz/PowerMW）均有明确来源，变化时只需更新结构体标签 |
| Bubble Tea v2 API 还较新，文档可能不完整 | v2 已于 2026-02 发布稳定版（v2.0.2），官方仓库有完整示例；参考研究报告中的 v1→v2 迁移速查表 |
| TUI + zerolog 日志 stdout 冲突 | 在 RunE 中 TUI 启动前将 log.Logger 重定向到临时文件，已有完整方案 |
| `sudo powermetrics` 需要用户显式加 sudo 前缀 | README 和命令 help 文本明确注明；`os.Getuid() != 0` 检查会给出友好提示 |
| 不同 SoC 的功耗量纲（mW vs W 不确定性） | 在 `GPUSnapshot` 中统一使用 mW；`ParseSample` 加 TODO 注释说明需实机验证量纲 |

## Documentation / Operational Notes

- README 新增 `### jdan macgpu` 章节，注明 sudo 要求及 arm64 限制
- 该命令无持久化数据、无网络请求、无配置文件副作用
- 调试日志写入临时文件（`/tmp/jdan-macgpu-*.log`），TUI 退出后自动保留（不删除，用户可检查）

## Sources & References

- **Origin document:** [docs/brainstorms/2026-03-30-mac-gpu-monitor-requirements.md](../brainstorms/2026-03-30-mac-gpu-monitor-requirements.md)
- Bubble Tea v2 官方仓库：charm.land/bubbletea/v2（v2.0.2, 2026-03-06）
- Lip Gloss v2：charm.land/lipgloss/v2（v2.0.2, 2026-03-11）
- plist 解析库：howett.net/plist
- powermetrics 字段研究：M1/M4 Pro 实机 plist 采样数据（见研究报告）
- 相关本地文件：`internal/cli/pubip.go`、`internal/cli/http_timing.go`、`internal/filebak/name_test.go`
