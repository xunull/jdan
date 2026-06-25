# jdan toc

从 Markdown 标题生成目录（TOC），anchor 跟 **GitHub 渲染规则一致**，可直接贴回 README。0 新依赖（纯 stdlib）。

全新「文档」品类。

## 它能干什么

```bash
$ jdan toc README.md
- [安装](#安装)
  - [方式 1：下载预编译二进制（推荐）](#方式-1下载预编译二进制推荐)
- [命令](#命令)
  - [`jdan qr`](#jdan-qr)
  - [`jdan figlet`](#jdan-figlet)
```

整完一个长 README 后一键生成目录，anchor 不用手算。

## 用法

```bash
jdan toc <file>                    # 输出 TOC 到 stdout
jdan toc                           # 读 stdin
jdan toc README.md --min 2 --max 3 # 只要某几级
jdan toc README.md --inplace       # 回填到文件
```

## anchor 与 GitHub 一致

anchor 算法：lowercase → 保留字母/数字/标记/连字符/下划线 → 空格转连字符 → 删除其余（反引号、标点等）→ 重复标题按全文顺序加 `-1`/`-2`。

| 标题 | anchor |
|------|--------|
| `## 安装` | `#安装`（CJK 保留）|
| `### \`jdan http timing\`` | `#jdan-http-timing`（反引号删除）|
| `## 方式 1：下载（推荐）` | `#方式-1下载推荐`（标点删、空格转连字符）|
| `### C#` | `#c`（GitHub 也删 `#`）|
| 两个 `## Setup` | `#setup` / `#setup-1` |

> 已用 jdan 自己的 README 验证：生成的 anchor 跟 README 里现有的 `#jdan-xxx` 内部链接**逐一吻合**。

## flags

| flag | 默认 | 用途 |
|------|------|------|
| `--min` | 2 | 最小标题级别（默认从 h2 起，**跳过文档大标题** `# xxx`）|
| `--max` | 6 | 最大标题级别 |
| `--inplace` | false | 回填到文件的 `<!-- toc -->` 标记之间 |

缩进 = `(标题级别 − 最小级别) × 2` 空格。链接文字保留原标题（含反引号），anchor 去标点。

## `--inplace` 回填

在文件里 `<!-- toc -->` 和 `<!-- /toc -->` 两个标记**之间**替换为生成的 TOC。先在文件里放好标记：

```markdown
<!-- toc -->
<!-- /toc -->
```

跑 `jdan toc README.md --inplace` 后变成：

```markdown
<!-- toc -->
- [安装](#安装)
- [命令](#命令)
<!-- /toc -->
```

**标记不存在则报错**（不瞎猜插入位置）。可重复运行（**幂等**，每次结果相同）。

## 正确处理的边界

- **代码围栏内的 `#` 不当标题**：``` 和 `~~~` 围栏里的 `# 注释` 会被跳过（否则代码块里的注释会污染目录）。
- `#nospace`（`#` 后无空格）不是标题。
- `## Title ##` 去掉收尾 `#` 序列；但 `### C#` 的 `#` 紧贴字母、不是收尾序列，文字保留为 `C#`。
- 重复标题 anchor 加 `-1`/`-2`，**按全文出现顺序**（跟 GitHub 一致，即使被 `--min`/`--max` 过滤掉的标题也参与编号）。

## 内部架构

```
internal/toc/
  toc.go   ParseHeadings(md) —— 解析全部 ATX 标题（跳过代码围栏），
           每个带 Level/Text/Anchor（全局 slugger 去重）
           Slug(text) —— GitHub anchor 算法
           Render(headings, min, max) —— 按级别过滤 + 缩进 + bullet
           Insert(content, toc) —— 替换 <!-- toc --> 标记之间，缺标记报错
internal/cli/toc.go
```

## 测试

- `internal/toc`：Slug（lowercase/空格转连字符/反引号·标点删/CJK 保留/`C#`→`c`）/ ParseHeadings（基本/**代码围栏 `#` 跳过**/`~~~` 围栏/`#nospace` 非标题/收尾 `#`/重复去重 `-1`/`-2`/7 个 `#` 非标题）/ Render（缩进/级别过滤/空/链接保留反引号）/ Insert（替换/幂等/缺标记报错/空 TOC 清旧内容）
- `internal/cli`：stdout（默认跳 h1）/ `--min`/`--max` / stdin / `--inplace` 写文件 + 标记保留 / 缺标记报错 / stdin+inplace 报错 / 文件不存在 / 级别非法

## 退出码

| 状况 | exit code |
|------|-----------|
| 成功 | 0 |
| `--inplace` 缺标记 / 文件不存在 / 级别非法 | 1 |

## 有意不做

| 候选 | 原因 |
|------|------|
| setext 标题（`===`/`---` 下划线）| 罕见；第一版只做 ATX `#` |
| 自动找插入位置 | 要标记更可控、不破坏文档 |
| 非 GitHub anchor 方言（GitLab 等）| 默认 GitHub；够覆盖主场景 |

## TL;DR

1. `jdan toc <file>` —— 从 Markdown 标题生成目录，anchor 跟 GitHub 一致
2. 默认从 h2 起（跳过文档大标题），`--min`/`--max` 控制级别
3. `--inplace` 回填到 `<!-- toc -->` 标记之间，幂等
4. 代码围栏内 `#` 不误当标题；重复标题加 `-1`/`-2`
5. **0 新依赖**；已用 jdan 自己 README 验证 anchor 逐一吻合
