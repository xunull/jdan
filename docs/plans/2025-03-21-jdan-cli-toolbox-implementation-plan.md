# jdan CLI toolbox Implementation Plan

> **For Claude:** REQUIRED SUB-SKILL: Use superpowers:executing-plans to implement this plan task-by-task.

**Goal:** 交付单二进制 `jdan`，含 `jdan file bak`：按设计文档将文件备份到同目录，命名含 `.bak`、本地时间戳 `YYYYMMDD-HHMMSS`，可选 `--desc`（仅字母/汉字/数字/空格，空格变 `_`，非法字符则打日志并退出且不复制）；目标已存在则提示「相同时间戳备份已存在」并退出。

**Architecture:** `cmd/jdan` 入口 + `internal/cli` 注册 Cobra；备份与命名逻辑放 `internal/filebak`（纯函数可测）；Viper/zerolog 接线遵循项目技能。

**Tech Stack:** Go 1.22+（或仓库选定版本）、github.com/spf13/cobra、github.com/spf13/viper、github.com/rs/zerolog

**设计依据：** `docs/plans/2025-03-21-jdan-cli-toolbox-design.md`  
**技能：** `@.cursor/skills/go-cli-cobra-viper/SKILL.md`

---

### Task 1: 初始化模块与入口日志

**Files:**
- Create: `go.mod`（module 路径与仓库一致，例如 `github.com/xunull/jdan`，可按远程调整）
- Create: `cmd/jdan/main.go`

**Step 1:** 在 `go.mod` 中声明 module，添加 cobra、viper、zerolog 依赖（`go mod tidy`）。

**Step 2:** 在 `main.go` 中设置：

```go
log.Logger = log.Output(zerolog.ConsoleWriter{Out: os.Stdout, TimeFormat: time.RFC3339})
```

然后调用 `cli.Execute()`（或当前包名）。

**Step 3:** `go build -o jdan ./cmd/jdan` 应能通过（可先返回 `nil` 的根命令占位）。

**Step 4:** Commit：`chore: init go module and jdan entrypoint`

---

### Task 2: Cobra 根命令与 `file bak` 骨架

**Files:**
- Create: `internal/cli/root.go`
- Create: `internal/cli/file_bak.go`（或 `internal/cli/file/bak.go`）

**Step 1:** 实现 `rootCmd`、`fileCmd`、`fileBakCmd`；`fileBakCmd` 接受一个参数「源路径」、flag `--desc`。

**Step 2:** `RunE` 中暂时只 `log.Info` 打印解析结果，验证 `jdan file bak ./x --desc "a"` 可用。

**Step 3:** Commit：`feat(cli): add file bak command skeleton`

---

### Task 3: 描述校验与备份文件名（TDD）

**Files:**
- Create: `internal/filebak/name.go`
- Create: `internal/filebak/name_test.go`

**Step 1: 写失败测试**

- `NormalizeDesc("")` → `""`, `ok`
- `NormalizeDesc("  ")` → `""`, `ok`
- `NormalizeDesc("a b")` → `"a_b"`, `ok`
- `NormalizeDesc("ab")` → `"ab"`, `ok`
- `NormalizeDesc("中文")` → `"中文"`, `ok`
- `NormalizeDesc("a,b")` → `!ok`（含标点）
- `NormalizeDesc("a\tb")` → `!ok`

**Step 2:** `go test ./internal/filebak -run Normalize -v` → 预期 FAIL（函数未实现）。

**Step 3:** 实现：trim → 若空返回空；否则逐 rune 校验（`a-zA-Z0-9`、Han、` `）；失败返回 `ok=false`；成功将空格换 `_`。

**Step 4:** `go test ./internal/filebak -v` → PASS。

**Step 5:** Commit：`feat(filebak): validate and normalize backup description`

---

### Task 4: 目标路径与时间戳

**Files:**
- Modify: `internal/filebak/name.go`
- Modify: `internal/filebak/name_test.go`

**Step 1: 写失败测试**

固定 `time.Time`（用参数注入或包级 `now func()` 便于测试），断言：
- 源 `dir/foo.txt`，无 desc → `dir/foo.txt.bak.20250321-153045`
- 有 desc `a b` → `...153045-a_b`
- desc 非法时函数应返回错误（由调用方打日志并退出）

**Step 2:** 实现 `BackupPath(src string, now time.Time, descNormalized string) (dst string, err error)`；冲突检测留给 Task 5，此处只生成路径字符串。

**Step 3:** `go test ./internal/filebak -v` → PASS。

**Step 4:** Commit：`feat(filebak): build backup destination path with timestamp`

---

### Task 5: 复制与错误分支

**Files:**
- Create: `internal/filebak/copy.go`（或合并入同一包）
- Modify: `internal/cli/file_bak.go`

**Step 1:** 在 `RunE` 中：`os.Stat` 源文件，确保 `Mode().IsRegular()`（或平台等价）；否则返回错误。

**Step 2:** 解析 desc：`NormalizeDesc`；`!ok` 时 **zerolog 提示仅允许英文字母、汉字、数字与空格**，返回错误（非零退出）。

**Step 3:** 计算 `dst`；`os.Stat(dst)` 若存在 → 返回明确错误：**已存在相同时间戳的备份**（与设计一致，不复制）。

**Step 4:** `io.Copy` + `os.Open`/`os.Create`（或 `os.ReadFile`/`os.WriteFile` 小文件），注意权限与 `Close`、临时失败信息。

**Step 5:** 手动或集成测试：临时目录创建源文件，执行 `jdan file bak`；第二次同秒执行应命中「已存在」分支（若 CI 不稳定可只测 Stat 分支 mock）。

**Step 6:** Commit：`feat(filebak): implement copy and conflict handling`

---

### Task 6: Viper 接线（最小）

**Files:**
- Modify: `internal/cli/root.go`（或 `PersistentPreRunE`）

**Step 1:** 按 `go-cli-cobra-viper` 技能：`SetEnvPrefix`（如 `JDAN`）、`AutomaticEnv`、`ReadInConfig` 可选忽略；在解析后 `BindPFlags`（Persistent + Local）。

**Step 2:** 首版可只把全局 flag（如未来 `--config`）挂到 viper；`file bak` 仍以 pflag 为准。

**Step 3:** Commit：`feat(cli): wire viper with flag precedence`

---

### Task 7: README 与验证

**Files:**
- Create或修改: `README.md`（安装、`jdan file bak` 示例、描述字符规则说明）

**Step 1:** `go test ./...`、`go build ./cmd/jdan`

**Step 2:** Commit：`docs: add README for jdan file bak`

---

## 执行交接

实现时按任务顺序执行；每任务结束保持可编译、测试通过后再进入下一任务。完成后可用 `executing-plans` 或常规 PR 流程合并。
