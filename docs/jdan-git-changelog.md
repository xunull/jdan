# jdan git changelog

从上个 tag 到 HEAD 的提交生成 changelog，按 Conventional Commits 分组（feat/fix/…）。跟 `feat()/fix()` 提交风格契合，发版前一键出发布说明。0 新依赖（shell out git，复用 `internal/gitx`）。`jdan git` 下的子命令。

## 它能干什么

```bash
$ jdan git changelog
## 未发布 (自 v0.5.2)

### ⚠ Breaking Changes
- (api) drop the v1 endpoint

### Features
- (json) add json merge for deep-merging
- (ping) add jdan ping with --dns

### Bug Fixes
- (figlet) block font UTF-8 panic

### Other
- bump deps
```

## 流程

1. **范围**：`--from`..`--to`。默认 `--to`=HEAD；默认 `--from`=最近的 tag（`git describe --tags --abbrev=0`），没 tag 则取全部历史。
2. **取 commit**：`git log <range> --no-merges`（merge commit 默认跳过去噪）。
3. **解析 subject** 为 Conventional Commit：`type(scope)!: desc`。breaking 判定 = subject 带 `!` 或 body 含 `BREAKING CHANGE`/`BREAKING-CHANGE`（大小写不敏感）。不符合规范的 subject → 归到 **Other**（不丢）。
4. **分组**（固定顺序）：⚠ Breaking Changes → Features(feat) → Bug Fixes(fix) → Performance(perf) → Refactoring(refactor) → Documentation(docs) → Other（chore/test/ci/build/style/revert/非规范）。

> breaking 的 commit **只**进 Breaking Changes 段，不重复进类型段。

## 用法

```bash
jdan git changelog                       # 最近 tag → HEAD
jdan git changelog --from v0.4.0 --to v0.5.0
jdan git changelog > RELEASE.md          # 默认就是 markdown，直接重定向
jdan git changelog --json                # 结构化输出
```

## flags

| flag | 默认 | 用途 |
|------|------|------|
| `--from` | 最近 tag（无则全历史） | 起点 ref（tag/commit/分支） |
| `--to` | HEAD | 终点 ref |
| `--json` | false | 结构化输出 `{from,to,commits:[{hash,type,scope,subject,breaking}]}` |

## 标题规则

- `--to`=HEAD（默认）→ `## 未发布 (自 <from>)`
- `--to` 是 tag → `## <to> (自 <from>)`
- 无 tag → `(全部历史)`
- 范围内无 commit → 标题 + `_(无提交)_`

## 内部架构

```
internal/gitx/changelog.go
  BuildChangelog(run, dir, from, to) (Changelog, error)  —— 范围解析 + git log + 分类
  parseCommit(hash, subject, body) Commit                —— Conventional Commit 解析（纯函数）
  (Changelog).Markdown() / .JSON()                       —— 渲染（纯函数）
internal/cli/git_changelog.go                            —— changelog 子命令挂到 jdan git
```

跟 `git summary` 一样**注入式 Runner**：单测注入假 Runner（喂 canned `git log` 输出）断言解析/分组/渲染，不依赖真 git；外加一个**临时真仓库**集成测试（`git init` + conventional commit + 打 tag，跑 `ExecRunner` 断言范围与分组）。

## 边界 / 错误

- 不是 git 仓库 → 报错（提示 `git init`）。
- 无 tag → 默认取全部历史。
- `--from`/`--to` 是非法 ref → git 报错透出。
- merge commit 默认 `--no-merges` 跳过。

## 测试（~30）

- `parseCommit`：feat 带/不带 scope、fix、`!` breaking、body `BREAKING CHANGE` breaking、`BREAKING-CHANGE` 连字符、非规范 subject（type 空）、type 转小写
- `BuildChangelog`（假 Runner）：范围解析 + 解析、非 repo 报错、无 tag 全历史
- `Markdown`：分组 + breaking 单独拎出不重复、段落顺序、Other 兜底、空范围、tag 标题、全历史标题
- `JSON`：可 Unmarshal、空 commits 为 `[]` 非 null
- **集成**（真仓库）：tag 前的 commit 不在范围、tag 后的进范围、`from` 解析为最近 tag
- CLI：text / `--json` / `--from`/`--to` 标题 / 非 repo 报错

## 退出码

| 状况 | exit code |
|------|-----------|
| 成功 | 0 |
| 非 git 仓库 / 非法 ref | 1 |

## 有意不做

| 候选 | 原因 |
|------|------|
| 自动写进 CHANGELOG.md 顶部 | 第一版输出到 stdout，重定向够用；该交给 `/ship` 那类 |
| commit hash 链接到 GitHub | 需 remote URL + 格式；第一版纯文本（`--json` 里带 hash） |
| 自定义 type→段落映射 | 内置一套覆盖主流；可后续加 |

## TL;DR

1. `jdan git changelog` —— 最近 tag → HEAD，Conventional Commits 分组
2. feat→Features / fix→Bug Fixes / …，breaking 单独拎出不重复
3. 默认 markdown（直接 `> RELEASE.md`），`--json` 结构化
4. `--from`/`--to` 指定范围，无 tag 取全历史
5. **0 新依赖**，复用 `gitx` 注入式 Runner
