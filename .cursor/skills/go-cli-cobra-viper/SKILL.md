---
name: go-cli-cobra-viper
description: Use when building or refactoring Go CLIs that need nested commands, layered settings from flags/env/files, and zerolog output as plain text on stdout.
---

# Go CLI：Cobra + Viper + zerolog

## Overview

用 **Cobra** 组织命令与子命令，用 **Viper** 统一读取配置，并固定优先级：**命令行标志 > 环境变量 > 配置文件 > 代码默认值**。日志统一用 **zerolog** 的 `ConsoleWriter` 写到 stdout，便于人类阅读。

## When to Use

- 新建或扩展带 `root` / 多级 `subcommand` 的 Go 工具
- 同一配置项需同时支持 `--flag`、`MYAPP_*` 环境变量与 `config.yaml`（或同类文件）
- 需要结构化日志但希望终端里是带时间的普通文本行，而非纯 JSON

**When NOT to Use**

- 无子命令的极简工具（可直接 `flag` 包）
- 必须只输出 JSON 日志给采集端（可改用默认 JSON `zerolog` 或单独 sink）
- 配置来源单一、无需合并优先级

## Core Pattern

1. **Cobra**：`cmd/root.go` 定义 `rootCmd`；每个子域一个包或文件，在 `init()` 里 `rootCmd.AddCommand(...)`。
2. **Viper**：单例即可；在 **Cobra 已解析完当前命令行** 之后，对**正在执行的** `*cobra.Command` 依次 `BindPFlags(cmd.PersistentFlags())` 与 `BindPFlags(cmd.Flags())`，使 **flag 覆盖 env 与文件**（Viper 默认优先级：显式 `Set` > **Flag** > **Env** > **Config** > **Default**）。
3. **zerolog**：进程入口尽早设置全局 `log.Logger` 为 `ConsoleWriter`，其余包使用 `github.com/rs/zerolog/log`。

## Quick Reference

| 关注点 | 做法 |
|--------|------|
| 子命令 | `cobra.Command{Use: "name", RunE: ...}`，`rootCmd.AddCommand` |
| 配置键 | flag 名与 viper 键一致；env 用 `SetEnvPrefix` + `AutomaticEnv`，必要时 `SetEnvKeyReplacer`（如 `.` → `_`） |
| 读配置顺序 | `SetDefault` → `ReadInConfig`（可忽略错误）→ `SetEnvPrefix` + `AutomaticEnv` → 当前 `cmd` 上 `BindPFlags`（Persistent + Local） |
| 日志 | `log.Output(zerolog.ConsoleWriter{Out: os.Stdout, TimeFormat: time.RFC3339})` |

## Implementation

**入口（`main.go`）**：先配日志，再执行 `cmd.Execute()`。

```go
package main

import (
	"os"
	"time"

	"github.com/rs/zerolog"
	"github.com/rs/zerolog/log"

	"your/module/cmd"
)

func main() {
	log.Logger = log.Output(zerolog.ConsoleWriter{
		Out:        os.Stdout,
		TimeFormat: time.RFC3339,
	})
	if err := cmd.Execute(); err != nil {
		log.Fatal().Err(err).Msg("command failed")
	}
}
```

**Root 与 Viper 绑定（示意）**：在 `PersistentPreRunE` 或子命令的 `PreRunE` 里，对**当前** `cmd` 绑定 flags，保证子命令自己的 flag 也参与覆盖。

```go
var cfgFile string

rootCmd.PersistentFlags().StringVar(&cfgFile, "config", "", "config file path")
// 子命令再定义自己的 Flags()

rootCmd.PersistentPreRunE = func(cmd *cobra.Command, args []string) error {
	if cfgFile != "" {
		viper.SetConfigFile(cfgFile)
	} else {
		viper.SetConfigName("config")
		viper.AddConfigPath(".")
	}
	_ = viper.ReadInConfig() // 可选：无文件时继续用 env/default

	viper.SetEnvPrefix("MYAPP")
	viper.AutomaticEnv()
	viper.SetEnvKeyReplacer(strings.NewReplacer(".", "_"))

	if err := viper.BindPFlags(cmd.PersistentFlags()); err != nil {
		return err
	}
	return viper.BindPFlags(cmd.Flags())
}
```

业务代码用 `viper.GetString("key")` 等读取；默认值用 `viper.SetDefault` 在 `init` 或 root 初始化阶段设好。

## Common Mistakes

- **BindPFlags 过早或漏绑**：在 Cobra 解析前绑定、只绑了 `Flags()` 未绑 `PersistentFlags()`，或子命令在独立 `PreRunE` 里又定义 flag 却未再绑定，导致优先级或取值不符合预期。
- **未设 `AutomaticEnv`**：环境变量无法参与合并。
- **日志重复初始化**：在多个 `init()` 里改 `log.Logger`，顺序混乱；应只在 `main` 或单一 `setupLogging()` 调用一次。
- **在库代码里 `log.Fatal`**：可执行文件顶层可以，库应返回 `error` 由命令层记录。

## Real-World Impact

统一的命令结构、配置来源优先级与终端可读日志，减少「本地能跑、CI 不行」类问题，并便于后来者按同一套路加子命令与配置项。
