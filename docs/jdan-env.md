# jdan env

`.env` 文件检查工具。4 个子命令覆盖 **lint / diff / redact / get**。偏"检查 / 对比 / 脱敏"，不做加载（那是 `direnv` / `dotenv-cli` 的范畴）。0 新依赖（纯 stdlib）。

## 它解决什么问题

`.env` 是开发者每天碰的文件，但出问题很隐蔽：

- prod `.env` 少了一个 key → 服务起不来，部署时才发现
- `KEY=hello world` 没加引号 → shell 加载时被截断成 `hello`
- 重复 key → 后面的悄悄覆盖前面的
- 想把 `.env` 贴到 issue 求助 → 直接泄露所有 secret
- 想从 `.env` 取一个值 → `grep + cut` 处理不了引号 / `export` 前缀 / 行内注释

`jdan env` 把这些变成一行命令。

## 子命令一览

| 子命令 | 用途 |
|--------|------|
| `lint <file>` | 检查问题（重复 key / 未引号空格 / 非法名 / 尾空格 / CRLF / BOM） |
| `diff <a> <b>` | 对比两个 .env 的 key 差异（部署前查漏） |
| `redact <file>` | 脱敏 value 以便安全分享 |
| `get <file> <key>` | 取单个 key 的 value（正确处理引号 / export / 行内注释） |

## `.env` 解析规则

对齐主流 dotenv 实现：

- `KEY=value` 每行一对
- `# 注释行` + 行内注释（`KEY=value # comment`）
- `export KEY=value`（shell 兼容前缀）
- 引号：`KEY="value with spaces"` / `KEY='literal'`
- 空行跳过，空值 `KEY=` 合法
- 重复 key 取最后一个（跟 shell 加载语义一致）

## lint

```bash
$ jdan env lint .env
.env:3   warning  duplicate key DATABASE_URL (first at line 1)
.env:5   warning  unquoted value with spaces: KEY=hello world
.env:6   error    invalid key name "2FOO" (must match [A-Za-z_][A-Za-z0-9_]*)
.env:7   warning  trailing whitespace in value for PORT

4 issues (1 errors, 3 warnings)
```

**检查项**：

| 检查 | 级别 | 为什么 |
|------|------|--------|
| 重复 key | warning | 后面的悄悄覆盖前面的 |
| 未引号但含空格的 value | warning | shell 加载会截断 |
| 非法 key 名（数字开头 / 非法字符） | error | shell 无法用 |
| 缺 `=` 的孤立 token | error | 解析不了 |
| value 尾随空格 | warning | 复制粘贴 bug |
| UTF-8 BOM | warning | Windows 编辑器留的，可能破坏 shell 解析 |
| CRLF 行尾 | warning | 用 LF |

**退出码**：有 error → 1（CI gate）。只有 warning → 0，除非 `--strict`。`--json` 给脚本消费。

## diff（部署前查漏最有用）

```bash
$ jdan env diff .env.example .env
Only in .env.example (3):
  + STRIPE_SECRET_KEY
  + REDIS_URL
  + SMTP_HOST
Only in .env (1):
  - DEBUG_FLAG
Common keys: 12
```

**默认只比 key 不比 value**（value 是 secret，不该对比/泄露）。典型用法：`.env.example` 是模板，部署前查 `.env` 缺了哪些 key。

```bash
# CI gate：确保 prod .env 没缺 key
jdan env diff .env.example .env.prod --exit-code && echo "all keys present"
```

`--values` 才对比公共 key 的 value（明确要求时）：

```bash
$ jdan env diff .env.example .env --values
...
Value differs (1):
  ~ API_KEY
```

`--exit-code` 让有差异时退出码 1（CI gate）。`--json` 给脚本消费。

## redact（安全分享）

```bash
$ jdan env redact .env
DATABASE_URL=po**************************db
export API_KEY=sk***********56
PORT=8**0
DEBUG=tr*e
```

value 打码成 `头***尾`（保留首尾各 1-2 字符帮识别），贴到 issue / 截图安全。`export` 前缀和 key 名保留。

```bash
jdan env redact .env | pbcopy   # 脱敏后直接进剪贴板
```

- `--full`：完全打码（`****`），不保留首尾
- `--keep-short`：短值（≤4 字符）/ 布尔类（true/false/yes/no/0/1）不打码

## get（脚本用）

```bash
$ jdan env get .env DATABASE_URL
postgres://localhost:5432/mydb
```

比 `grep + cut` 可靠：正确处理引号剥离、`export` 前缀、行内注释。key 不存在 → 退出码 1。

```bash
# 脚本里取值
DB=$(jdan env get .env DATABASE_URL)
```

## flags

| 子命令 | flag | 用途 |
|--------|------|------|
| `lint` | `--strict` | warning 也算失败（退出码 1） |
| `lint` | `--json` | 结构化输出 |
| `diff` | `--values` | 也对比公共 key 的 value（默认只比 key） |
| `diff` | `--exit-code` | 有差异时退出码 1（CI gate） |
| `diff` | `--json` | 结构化输出 |
| `redact` | `--full` | 完全打码 |
| `redact` | `--keep-short` | 短值 / 布尔不打码 |

## 内部架构

```
internal/dotenv/
  parse.go    Parse（引号 / export / 行内注释 / CRLF / BOM 检测）+ Entry / File
  lint.go     Lint（6 类检查）+ Issue / Severity / CountBySeverity
  diff.go     Diff（key 集合 + 可选 value）+ DiffResult / HasDifferences
  redact.go   RedactValue / RedactLine（头***尾 / full / keep-short）
  get.go      Get（取值，重复 key 取最后）

internal/cli/env.go    父命令 + 4 子命令
```

**0 新依赖**，全 stdlib。

## 测试

- 31 unit tests on `internal/dotenv`：
  - Parse 引号 / export / 行内注释 / 空值 / CRLF / BOM / 注释跳过 + 行号
  - Lint 每类 issue + clean file + BOM/CRLF + severity 计数
  - Diff key 集合 / 不带 --values 不比 value / 带 --values / 重复 key 取最后
  - Redact 长值首尾保留 / --full / --keep-short / 空值 / export 前缀保留
  - Get 取值（引号剥离）/ 不存在报错 / 重复取最后
- 14 CLI tests：lint（issue/clean/strict/json/file-not-found）/
  diff（key/values/exit-code/json）/ redact（mask/full/keep-short）/ get（found/not-found）

## 退出码

| 状况 | exit code |
|------|-----------|
| 成功 | 0 |
| `lint` 有 error（或 `--strict` 有 warning） | 1 |
| `diff --exit-code` 有差异 | 1 |
| `get` key 不存在 | 1 |
| 文件不存在 | 1 |

## 有意不做的事

| 候选 | 不做原因 |
|------|---------|
| 加载 .env 到 shell | `direnv` / `dotenv-cli` 已做；jdan 偏"检查/对比"不偏"加载" |
| 写入/修改 .env（set key） | 有状态写操作；第一版聚焦只读检查 |
| .env 加密（SOPS / dotenv-vault） | key 管理是单独大 scope |
| 变量插值（`${OTHER_VAR}`） | 加载时才需要求值；lint/diff 不需要 |
| 多环境合并优先级 | 加载语义，出 jdan scope |

## TL;DR

1. `jdan env lint .env` —— 6 类检查，error → 退出码 1（CI gate），`--strict` 连 warning 也拦
2. `jdan env diff .env.example .env` —— 部署前查漏 key，默认只比 key 不泄露 value
3. `jdan env redact .env` —— 脱敏后安全贴 issue（`--full` / `--keep-short`）
4. `jdan env get .env KEY` —— 比 grep+cut 可靠（处理引号 / export / 行内注释）
5. **0 新依赖**，纯 stdlib；偏"检查/对比/脱敏"，加载交给 direnv
