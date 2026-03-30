---
date: 2026-03-30
topic: mac-gpu-monitor
---

# Mac GPU 监控（Apple Silicon）

## Problem Frame

开发者和重度用户在使用 Apple Silicon Mac 时，缺乏一个快速、直观的命令行工具来实时查看 GPU 的使用状态。
Activity Monitor 虽然有 GPU 面板，但不适合终端工作流；`powermetrics` 原始输出为 JSON 格式，不便阅读。
该功能将 `jdan mac gpu` 定位为终端环境下的 GPU 实时监控仪表盘，类似 htop 对 CPU 的定位。

## Requirements

**命令与入口**
- R1. 新增子命令 `jdan macgpu`（扁平结构，单个子命令），仅支持 Apple Silicon Mac（arm64 架构）；在非 Apple Silicon 平台运行时，输出明确提示并以非零退出码退出。
- R2. 命令需要 `sudo` 权限；若以非 root 身份运行，输出提示信息（"请使用 sudo 运行此命令"）并退出。

**数据采集**
- R3. 通过子进程调用 `sudo powermetrics --samplers gpu_power,thermal --format plist -i <interval> -n 0` 采集数据；`powermetrics` 为 macOS 内置工具，无需额外安装。注：`powermetrics` 只支持 `text` 和 `plist` 两种输出格式，不存在 JSON 格式；使用 plist 格式便于程序解析，每条采样以 `\x00`（NUL 字节）分隔。
- R4. 采样间隔默认 2000ms，支持通过 `--interval`（`-i`）参数自定义（单位 ms，最小 500ms）。
- R5. 采集以下指标（以 `powermetrics` 实际输出为准）：
  - GPU 活跃占用率（active residency %）
  - GPU 频率（MHz）
  - GPU 功耗（W）
  - 散热压力等级（Thermal Pressure：Nominal/Light/Moderate/Heavy/Critical）——注：Apple Silicon 的 `powermetrics thermal` 采样器不输出摄氏度，仅提供此 5 级枚举字符串

**TUI 显示**
- R6. 进入持续刷新的 TUI 界面（类 htop/glances 风格）；按 `q` 或 `Ctrl+C` 退出。
- R7. TUI 顶部区域：为 GPU 占用率、功耗、频率三项指标各自渲染一条带颜色的 ASCII 水平柱状图，颜色随数值高低变化（绿→黄→红）。
- R8. TUI 底部区域：以表格形式展示所有采集到的指标数值及单位。
- R9. 使用 `charmbracelet/bubbletea` 作为 TUI 框架，`charmbracelet/lipgloss` 负责样式渲染。
- R10. 界面顶部显示标题栏，包含命令名称、采样间隔及当前时间戳。

**兼容性与错误处理**
- R11. 若 `powermetrics` 未找到或调用失败，输出错误信息并退出，不崩溃。
- R12. 与现有命令保持一致：日志使用 zerolog，遵循 `go-cli-cobra-viper` 技能中的约定。

## Success Criteria

- 在 Apple Silicon Mac 上执行 `sudo jdan mac gpu`，能在 2 秒内进入 TUI 并展示实时 GPU 指标。
- 柱状图随 GPU 负载变化而动态更新，颜色符合阈值预期。
- 按 `q` 能干净退出，无残留 ANSI 字符。
- 非 Apple Silicon 平台或非 sudo 环境下有明确的错误提示。

## Scope Boundaries

- 不支持 Intel Mac 或外接 GPU（eGPU）。
- 不支持输出 JSON 格式（纯 TUI 工具，非数据管道场景）。
- 不监控 CPU、内存等非 GPU 指标（保持单一职责）。
- 不持久化历史数据或导出报告。
- 不支持进程级别的 GPU 占用分析（只显示全局 GPU 状态）。

## Key Decisions

- **数据来源选 `powermetrics` 而非 IOKit CGo**：powermetrics 是 Apple 官方工具，输出稳定，无需 CGo 跨语言调用，维护成本低；代价是需要 sudo，用户明确接受。
- **TUI 框架选 Bubble Tea + Lip Gloss**：渲染效果最接近 htop/glances 描述的质感，Go 生态中最成熟的 TUI 方案；引入 2 个新依赖，合理。
- **默认持续刷新，间隔 2s**：GPU 监控的核心价值在于实时观察；2s 间隔在可读性和功耗之间取得平衡。
- **命令结构选 `jdan macgpu`**：扁平结构，单命令，简洁直观；若未来需要 `macmem`、`maccpu` 等，亦可平行扩展。
- **柱状图覆盖全部三项指标**：占用率、功耗、频率各有独立柱状图，信息最完整，与 htop/glances 风格一致。

## Dependencies / Assumptions

- 目标平台为 macOS 14+（Sonoma）及以上，`powermetrics` 输出格式以此为准。
- 用户已知晓运行此命令需要 `sudo`，README 将注明。
- `powermetrics` 的 GPU 相关字段在不同 Apple Silicon 芯片（M1/M2/M3/M4 系列）上字段名可能略有不同，实现时需做字段缺失的兼容处理。

## Outstanding Questions

### Resolve Before Planning

（无阻塞性问题，可直接进入规划）

### Deferred to Planning

- [Affects R5][Needs research] `powermetrics` JSON 输出中，M1/M2/M3/M4 系列 GPU 相关字段的确切名称和层级结构需在实现阶段通过实机采样确认。
- [Affects R7][Technical] 柱状图的具体阈值配色（绿/黄/红的百分比切换点）留给实现阶段决定。
- [Affects R10][Technical] Bubble Tea 的布局是否需要响应终端窗口大小变化（`tea.WindowSizeMsg`），留给实现阶段评估。

## Next Steps

→ `/ce:plan` 进行结构化实现规划
