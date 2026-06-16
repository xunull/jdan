# jdan fake

生成像真实数据的结构化假值，供造测试 fixture、填库、写示例。0 新依赖（内置词库 + stdlib）。

跟 `jdan rand`（密码/随机字节/随机串）不同：`rand` 给的是**无意义随机串**，`fake` 给的是**像真实数据的结构化假值**（姓名、邮箱、日期……）。

## 它能干什么

```bash
$ jdan fake name
Alice Chen

$ jdan fake email -n 3
bob.patel@example.net
amy.wong@test.org
leo.kim@demo.net

$ jdan fake --json -n 2
[
  {"name":"Bob Patel","email":"bob.patel@example.net","age":74,"ip":"198.51.100.134"},
  {"name":"Zack Thomas","email":"zack.thomas@example.org","age":33,"ip":"203.0.113.175"}
]
```

## 支持的类型（8 种）

| type | 输出例 | 说明 |
|------|--------|------|
| `name` | `Alice Chen` | 名 + 姓（内置词库） |
| `email` | `alice.chen@example.com` | 由姓名派生 + 示例域名 |
| `uuid` | `f47ac10b-58cc-4372-a567-0e02b2c3d479` | v4 格式 |
| `sentence` | `Lorem ipsum dolor sit.` | lorem 词库，`--words N` 控制词数 |
| `word` | `lorem` | 单个 lorem 词 |
| `int` | `4271` | `--min/--max` 控制范围（默认 0–9999） |
| `date` | `2016-04-30` | 固定窗口内随机日期，`--format` 可改 |
| `ip` | `192.0.2.45` | RFC 5737 文档保留段内随机 IPv4 |

## 用法

```bash
jdan fake <type> [-n K]        # 生成 K 个某类型值（默认 1，每行一个）
jdan fake --json [-n K]        # 无 type：生成 K 条复合记录
jdan fake <type> --json -n K   # 某类型的 JSON 数组
jdan fake --list               # 列出类型
```

```bash
jdan fake name                 # 一个姓名
jdan fake email -n 3           # 3 个邮箱
jdan fake int --min 1 --max 6  # 骰子
jdan fake sentence --words 10  # 10 词的句子
jdan fake date --format 2006/01/02
jdan fake uuid --json -n 5     # JSON 数组
```

## flags

| flag | 默认 | 用途 |
|------|------|------|
| `-n, --count` | 1 | 生成几个 |
| `--seed` | 不设（=真随机熵） | **可复现**：给定 seed 输出固定 |
| `--json` | false | JSON 数组（有 type）或复合记录（无 type） |
| `--min / --max` | 0 / 9999 | int 范围（闭区间） |
| `--words` | 6 | sentence 词数 |
| `--format` | `2006-01-02` | date 格式（Go 参考时间布局） |
| `--list` | false | 列出类型 |

## 可复现性（关键设计）

默认从 `crypto/rand` 取熵 → 每次不同（真随机）。给 `--seed N` 则切到 `math/rand` 确定性序列 → **同 seed 同输出**。造稳定测试 fixture 时很有用。

```bash
$ jdan fake name --seed 42 -n 2
Zack Walker
Cleo King
$ jdan fake name --seed 42 -n 2   # 完全一样
Zack Walker
Cleo King
```

`date` 类型用一个固定窗口 `[2000-01-01, 2025-01-01)`，**不依赖 wall clock**，所以 `--seed` 完全可复现（不会因为今天日期不同而漂移）。

## 复合记录（--json 无 type）

无 type 时配 `--json` 生成完整的人物记录，`email` 与 `name` 保持一致：

```bash
$ jdan fake --json -n 2 --seed 1
[
  {"name":"Bob Patel","email":"bob.patel@example.net","age":74,"ip":"198.51.100.134"},
  {"name":"Zack Thomas","email":"zack.thomas@example.org","age":33,"ip":"203.0.113.175"}
]
```

字段：`name` / `email` / `age`（18–80）/ `ip`。

## 安全 / 边界

- **IP** 只生成 RFC 5737 文档保留段（`192.0.2.0/24`、`198.51.100.0/24`、`203.0.113.0/24`），不可路由，不会撞真实主机。
- **email 域名** 用 RFC 2606/6761 保留示例域名（`example.com` / `test.org` / `demo.net` 等），不撞真实邮箱。
- 假数据**仅供测试**，不要当真实身份/联系方式使用。

## 内部架构

```
internal/fake/
  data.go    内置词库：firstNames / lastNames / loremWords / domains / docIPBlocks
  fake.go    Generator{rng}：Name/Email/UUID/Sentence/Word/Int/Date/IP/Person
             New(seed) 确定性；NewRandom() 从 crypto/rand 取熵种子
internal/cli/fake.go
```

`--seed` 显式设置 → `fake.New(seed)`（确定性）；未设置 → `fake.NewRandom()`（真随机）。`uuid` 也走 Generator 的 rng，所以同样可复现。

## 测试

- `internal/fake`：可复现性（同 seed byte-equal，核心）/ 各类型格式（email 含 @、uuid v4 正则、date 可 `time.Parse` 且落在窗口、ip 是文档段合法 IPv4）/ int 范围 + 边界互换 / sentence 词数 / Value 分发 + 未知类型 / Person 复合记录字段 + 与 name 一致
- `internal/cli`：基本 / 可复现 / `-n` 行数 / JSON 数组合法可 Unmarshal / 复合记录 / int 范围 / sentence 词数 / `--list` / 未知类型 / 无 type 无 --json 报错 / count<1 报错

## 退出码

| 状况 | exit code |
|------|-----------|
| 成功 | 0 |
| 未知类型 / 无 type 又无 --json / count<1 | 1 |

## 有意不做的事

| 候选 | 不做原因 |
|------|---------|
| 本地化（中文名等）/ `--locale` | 第一版精选英文词库；多语言词库工作量大 |
| 几十种类型（faker 那种） | 8 种覆盖常用场景；保持精简 |
| 自定义模板 / schema | 复合记录够用；模板化是另一个工具 |
| 真随机域名/可路由 IP | 故意只用保留段，避免误用撞真实资源 |

## TL;DR

1. `jdan fake <type>` —— 生成假数据：name / email / uuid / sentence / word / int / date / ip
2. `--seed N` 可复现（造稳定 fixture）；不设则真随机
3. `--json` 出数组，无 type + `--json` 出复合记录
4. IP/邮箱用 RFC 保留段，安全不撞真实资源
5. **0 新依赖**，内置词库；跟 `jdan rand`（无意义随机串）互补
