# jdan json

JSON 工具集。10 个子命令覆盖**美化 / 取值 / diff / JSONL / YAML 互转 / CSV 互转**。设计目标：常见操作 0 学习曲线，不替代 jq。

## 它解决什么问题

开发者日常处理 JSON 的窘境：
- `python -m json.tool` 美化但参数难记
- `jq '.users[0].name'` 强大但语法要学
- YAML / CSV 想转 JSON 要找另外的工具（`yq`、`csvjson`、Excel）
- JSONL（一行一个 JSON 的日志格式）没有趁手的命令

**`jdan json` 是一组 "做最常见的事 + 0 语法门槛" 的子命令。** 复杂操作仍然推荐 jq。

## 子命令一览

| 子命令 | 用途 |
|--------|------|
| `pretty [file]` | 美化（默认 2 空格） |
| `minify [file]` | 压缩成单行 |
| `path <expr> [file]` | 按 path 取值（dot-path / bracket / RFC 6901） |
| `keys [file]` | 列出 key（顶层或递归所有路径） |
| `diff <a> <b>` | 语义 diff（输出 RFC 6902 JSON Patch） |
| `lines [file]` | JSONL 工具：count / get / head |
| `from-yaml [file]` | YAML → JSON |
| `to-yaml [file]` | JSON → YAML |
| `from-csv [file]` | CSV → JSON array |
| `to-csv [file]` | JSON array → CSV |

**输入约定**（所有命令统一）：file 参数 > stdin。不需要 `-i` flag。

## pretty / minify

```bash
$ jdan json pretty data.json
{
  "name": "alice",
  "age": 30
}

$ jdan json minify data.json
{"name":"alice","age":30}

$ jdan json pretty data.json --in-place    # 原地修改
$ jdan json pretty data.json --indent 4    # 4 空格缩进
```

**数字精度保留**：内部用 `json.Decoder.UseNumber()`，大整数（`2^53 + 1`）和高精度浮点不会被 float64 损失。这跟 `python -m json.tool` 不一样。

## path

按路径取值。三种语法可以**自由混用**：

```bash
# Dot-path（最简单，数组用 .N）
$ jdan json path "users.0.name" data.json
"alice"

# Bracket 表示数组（更直观）
$ jdan json path "users[0].name" data.json
"alice"

# 混用
$ jdan json path "servers[0].ports.2" data.json
8080

# 负 index（从末尾倒数）
$ jdan json path "xs[-1]" data.json

# Key 含 dot：用 backslash 转义
$ jdan json path "labels.foo\\.bar" data.json

# 字符串结果想去掉引号：-r / --raw
$ jdan json path "users[0].name" data.json -r
alice
```

切到严格的 **RFC 6901 JSON Pointer** 语法：

```bash
$ jdan json path "/users/0/name" data.json --pointer
"alice"
```

RFC 6901 转义：`~0` → `~`，`~1` → `/`。用于 path 段含 `/` 或 `~` 的字段。

## keys

```bash
# 默认：顶层 key（按字母序）
$ jdan json keys data.json
age
name
tags
users

# --all：递归所有叶子路径（dot-path 风格）
$ jdan json keys data.json --all
age
name
tags[0]
tags[1]
users[0].email
users[0].name
users[1].email

# 截断深度
$ jdan json keys data.json --all --depth 2
```

含 dot 的 key 自动 backslash 转义（跟 `path` 语义对齐，输出可以直接喂回 `path`）。

## diff

语义 diff，**不是文本 diff**：字段顺序、空格、缩进无关。

```bash
$ jdan json diff a.json b.json
~ /age: 30 -> 31
+ /new = true
~ /tags/1: "b" -> "c"
+ /tags/2 = "d"
```

`~` = replace，`+` = add，`-` = remove。Path 用 JSON Pointer。

输出 RFC 6902 JSON Patch（CI / 自动化用）：

```bash
$ jdan json diff a.json b.json --json
[
  { "op": "replace", "path": "/age", "old": 30, "value": 31 },
  { "op": "add", "path": "/new", "value": true },
  ...
]
```

CI gate（有差异时非零退出码）：

```bash
$ jdan json diff schema.json schema-prod.json --exit-code && echo "schema unchanged"
```

## lines (JSONL)

JSONL = newline-delimited JSON，一行一个 JSON 对象。**typical 用途**：结构化日志（zerolog、winston、structlog）、数据 pipeline 中间格式。

```bash
# 计数 + 校验每行 valid
$ jdan json lines --count < logs.jsonl
12847

# 取第 N 行（0-based，跳过空行）
$ jdan json lines --get 0 < logs.jsonl
{"level":"info","msg":"server start","ts":"2026-..."}

# 前 N 行
$ jdan json lines --head 10 < logs.jsonl

# 默认（不带 flag）：校验所有行 + 显示总数
$ jdan json lines logs.jsonl
12847 valid JSONL records
```

**遇到 invalid JSON 行立即报错并打行号**，不静默跳过。

## YAML 互转 (from-yaml / to-yaml)

```bash
# YAML → JSON（默认 pretty）
$ jdan json from-yaml config.yaml > config.json

# 紧凑 JSON
$ jdan json from-yaml config.yaml --pretty=false > config.json

# JSON → YAML
$ jdan json to-yaml config.json > config.yaml

# 管道串：JSON 文件 → YAML 文件
$ jdan json to-yaml < src.json > dst.yaml
```

**数字保留为 numeric**：YAML 里的 `port: 8080` 转 JSON 是 `"port": 8080`（不会被 quote 成 `"8080"`）；反向也一样。大整数（`2^53 + 1`）也不丢精度。

依赖：`go.yaml.in/yaml/v3`（之前就是 viper 的间接依赖，没引入新 dep）。

## CSV 互转 (from-csv / to-csv)

```bash
# CSV → JSON（默认带 header → array of objects）
$ jdan json from-csv users.csv --pretty=false
[{"age":"30","name":"alice"},{"age":"25","name":"bob"}]

# 无 header → array of arrays
$ jdan json from-csv data.csv --no-header

# Tab 分隔
$ jdan json from-csv data.tsv --delim '\t'

# JSON → CSV（默认按字母序 header）
$ jdan json to-csv users.json
age,name
30,alice
25,bob

# 指定列序
$ jdan json to-csv users.json --header "name,age"
```

**重要约定**：
- 所有 cell 保持 **string 类型**（CSV-as-strings）。类型推断留给下游（jq / 用户脚本）。
- **UTF-8 BOM 自动剥除**：Excel/Numbers 导出的 CSV 常带 BOM，不剥会让第一个 key 多出 3 个不可见字节。
- **Quoted field 标准 CSV 行为**：comma 和 embedded newline 都正确处理。
- **Ragged 短行**：缺失字段填空字符串（跟 pandas 一致）。
- **缺失 key**：写空 cell（不写 "null"）。
- **嵌套 object/array** → JSON-encode 成 sub-document 写入单元格。

## 跟其他工具的关系

| 工具 | 它做什么 | jdan json 的边界 |
|------|---------|------------------|
| **`jq`** | DSL 风格的过滤 / 转换 / 聚合 | jdan 不实现 jq DSL；复杂操作仍然推荐 jq |
| **`yq`** | jq 风格的 YAML 查询 | jdan from-yaml / to-yaml 只做转换；查询请用 jq + from-yaml 串管道 |
| **`dasel`** | 多格式（json/yaml/toml/xml）统一 path | dasel 是 jdan json 的 superset；jdan 更轻、专注 |
| **`python -m json.tool`** | 简单美化 | jdan pretty 保留数字精度 + 0 启动开销 |
| **`csvjson` / pandas** | CSV ↔ JSON 转换 | jdan from-csv / to-csv 覆盖 80% 场景，0 依赖 |

**推荐组合**：

```bash
# jq + jdan 互补
jdan json from-yaml config.yaml | jq '.servers[].port'

# jdan + jdan 链
jdan json from-csv users.csv --pretty=false | jdan json path '0.name' -r
```

## 内部架构

```
internal/jsonx/
  pretty.go     Pretty / Minify
  path.go       ParsePath / Get / DecodeValue（dot-path + bracket）
  pointer.go    ParsePointer / pointerEscape（RFC 6901）
  keys.go       Keys（顶层 / --all 递归）
  diff.go       Diff / DiffEntry（RFC 6902 兼容）
  lines.go      LinesCount / LinesGet / LinesHead（JSONL）
  yaml.go       YAMLToJSON / JSONToYAML（go.yaml.in/yaml/v3）
  csv.go        CSVToJSON / JSONToCSV（stdlib encoding/csv）

internal/cli/json.go    10 个 cobra 子命令
```

**测试**：
- jsonx 包 ≥ 50 unit test（边界、precision、RFC 兼容、round-trip）
- cli 包 ≥ 25 子命令测试（stdin/file/flag 组合）

## 退出码

| 状况 | exit code |
|------|-----------|
| 成功 | 0 |
| `diff` 默认（有差异不报错） | 0 |
| `diff --exit-code` 有差异 | 1 |
| 解析失败 / 文件不存在 / path 不匹配 / 互斥 flag 冲突 | 1 |

## 有意不做的事

| 候选 | 不做原因 |
|------|---------|
| **jq DSL** | jq 已经做得很好；jdan json 不重做 |
| **TOML 互转** | 暂无强烈需求（jq + viper 配套不缺）；如果用户要，3 行加 |
| **JSON Patch apply / merge / set 修改** | scope 大，需要专门设计 in-place vs stdout 策略；留给未来 |
| **JSON Schema validate / infer** | scope 大，schema 是单独的领域 |
| **CSV 类型推断（"123" → 数字）** | 不可靠（"0123" 是数字还是 zip code？），交给下游 |

## TL;DR

1. **`jdan json pretty/minify`** —— `python -m json.tool` 的替代，保留数字精度
2. **`jdan json path "users[0].name"`** —— dot-path / bracket / RFC 6901 三选一
3. **`jdan json diff a.json b.json`** —— 语义 diff，输出 RFC 6902 patch
4. **`jdan json lines --count`** —— JSONL（日志、数据 pipeline）
5. **`jdan json from-yaml/to-yaml`** —— 数字、嵌套、大 int 都不丢
6. **`jdan json from-csv/to-csv`** —— UTF-8 BOM 自动剥除、ragged rows 处理
7. 复杂操作 → jq；jdan json 只做 80% 高频场景
