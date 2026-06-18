# jdan git summary

仓库一眼看：总 commit 数、分支、tag、年龄、贡献者榜、改动最多的文件（hotspots）。纯只读。

jdan **第一个 git 命令**——底层 shell out 到 `git`（os/exec），**0 新 Go 依赖**，只要运行环境里有 git。做成 `jdan git` 父命令 + `summary` 子命令，给后续 git 子命令（changelog / clean / standup…）留好位置。

## 它能干什么

```bash
$ jdan git summary
仓库: jdan
commit: 77   分支: 5   tag: 2
年龄: 2026-03-21 起 (约 2 个月)

贡献者 Top 5:
  xunull  77 (100.0%)

改动最多的文件 (hotspots) Top 5:
  README.md            40 次
  go.mod                9 次
  .gitignore            8 次
  go.sum                7 次
  internal/cli/dns.go   4 次
```

接手一个新仓库、想快速摸清「谁在写、写了多久、哪些文件最常动（风险/热点）」时一条命令搞定。

## 用法

```bash
jdan git summary             # 当前目录的仓库
jdan git summary /path/repo  # 指定仓库
jdan git summary --top 10    # 贡献者 / hotspots 各显示 10 条（默认 5）
jdan git summary --json      # 结构化输出（供脚本）
```

## 收集什么 & 怎么拿

| 字段 | git 命令 |
|------|---------|
| 仓库名 | `git rev-parse --show-toplevel` 取 basename |
| commit 数 | `git rev-list --count HEAD` |
| 分支数 | `git for-each-ref refs/heads` 计数 |
| tag 数 | `git tag` 计数 |
| 年龄 | 第一条 commit 日期（`git log --reverse --max-parents=0`）→ 末条 commit 日期的跨度 |
| 贡献者 | `git log --format=%an` 计数 + 百分比，降序 |
| hotspots | `git log --pretty=format: --name-only` 统计文件出现次数，降序 |

> **年龄用「首 commit → 末 commit」跨度**（两端都来自 git），不依赖系统当前时间，所以测试里可复现。
>
> 贡献者按 `%an`（author name）统计；用 `Co-Authored-By:` trailer 署名的协作者不计入 author，这是 git 的语义。

## flags

| flag | 默认 | 用途 |
|------|------|------|
| `--top` | 5 | 贡献者 / hotspots 各显示几条 |
| `--json` | false | JSON 输出 |

可选位置参数 `[path]`：在指定目录的仓库上跑（默认当前目录）。

## 边界 / 错误处理

- **不是 git 仓库** → 报错并提示 `git init`。
- **空仓库（0 commit）** → 友好提示「仓库还没有任何提交」，不 panic。
- 环境里**没有 git 可执行文件** → 报错指明缺 git。

## 内部架构

```
internal/gitx/
  gitx.go      Runner 类型 + ExecRunner（os/exec 跑 git）+ IsRepo
  summary.go   Summarize(run, dir, top) → Summary；Contributor/Hotspot 结构；
               humanizeSpan(first, last)
internal/cli/git.go          jdan git 父命令（grouping）
internal/cli/git_summary.go  summary 子命令（注入 Runner，便于测试）
```

**Runner 注入式设计**：CLI 注入真实 `ExecRunner`；测试可注入返回固定输出的 fake runner（不依赖真 git），或直接对临时真仓库跑。

## 测试

- `internal/gitx`：建临时真仓库（`git init` + 用 `GIT_AUTHOR_DATE`/`GIT_COMMITTER_DATE` 造多作者多日期 commit + 打 tag + 反复改某文件造 hotspots）→ 断言 commit/tag/分支数、贡献者降序 + 百分比、hotspots 降序、年龄跨度；非 git 仓库报错；空仓库报错；`humanizeSpan` 天/月/年边界 + 对称性；`IsRepo`。（无 git 环境自动 skip）
- `internal/cli`：fake runner 驱动 → 文本输出含关键行 / `--json` 合法可 Unmarshal / `--top` 限制条数 / 非 repo 报错 / `git` 父命令挂了 `summary` 子命令

## 退出码

| 状况 | exit code |
|------|-----------|
| 成功 | 0 |
| 非 git 仓库 / 空仓库 / 缺 git | 1 |

## 有意不做（留给后续 git 子命令）

| 候选 | 去向 |
|------|------|
| `changelog`（按 Conventional Commits 生成发布说明） | 后续独立子命令 |
| `clean`（清理已合并/gone 分支） | 后续独立子命令 |
| `standup`（我最近的 commit） | 后续独立子命令 |
| 代码行数统计（cloc 那种） | 需逐文件读，超范围 |
| 远端 / PR 统计 | 需联网或 gh，另说 |

## TL;DR

1. `jdan git summary` —— 仓库一眼看：commit / 分支 / tag / 年龄 / 贡献者 / hotspots
2. jdan 第一个 git 命令，底层调 git，**0 新 Go 依赖**
3. `--top N` 控制榜单条数，`--json` 供脚本
4. 年龄用首末 commit 跨度（可复现），贡献者按 author name 统计
5. `jdan git` 父命令已留好位置给 changelog / clean / standup 等后续子命令
