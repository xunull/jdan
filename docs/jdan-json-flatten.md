# jdan json flatten / unflatten

嵌套 JSON ↔ 扁平点分键。`flatten` 把 `{a:{b:1}}` 压成 `{"a.b":1}`，`unflatten` 反向还原。方便 grep / diff / 转 env / 填表格 / 看结构。0 新依赖（纯 stdlib）。`jdan json` 下的两个子命令。

## 它能干什么

```bash
$ echo '{"a":{"b":1,"c":[10,20]}}' | jdan json flatten
{"a.b":1,"a.c[0]":10,"a.c[1]":20}

$ echo '{"a.b":1,"a.c":2}' | jdan json unflatten
{"a":{"b":1,"c":2}}
```

## 键格式 = `jdan json path` 表达式

flatten 的键用**对象 `sep` 连接 + 数组 `[i]` 下标**：`a.b`、`a.c[0]`。这正好是现有 `jdan json path` 能吃的表达式——flatten 出来的键可以**直接喂回** `jdan json path`，整个 json 工具链一致。

```bash
$ echo '{"a":{"c":[10,20]}}' | jdan json flatten
{"a.c[0]":10,"a.c[1]":20}
$ echo '{"a":{"c":[10,20]}}' | jdan json path 'a.c[1]'
20
```

## round-trip 是核心正确性

**`flatten` 再 `unflatten` 还原原结构**（语义相等，key 顺序可不同）。为此：
- **空对象 `{}` / 空数组 `[]` 当叶子保留**（`{"a":{}}` → `{"a":{}}`），否则会丢失、还原不回来。
- 用 `json.Number`（`UseNumber`）解码，**保大整数精度**（跟其他 json 子命令一致）。

```bash
$ echo '{"n":12345678901234567890,"a":{"b":[]}}' | jdan json flatten | jdan json unflatten
{"a":{"b":[]},"n":12345678901234567890}   # 大整数精度 + 空数组都保住
```

## 用法

```bash
jdan json flatten [file]        # 嵌套 → 扁平
jdan json unflatten [file]      # 扁平 → 嵌套
echo '{...}' | jdan json flatten
jdan json flatten data.json --sep / -p
```

## flags

| flag | 默认 | 用途 |
|------|------|------|
| `--sep` | `.` | 对象键连接符（数组始终用 `[i]`，不受 sep 影响） |
| `-p, --pretty` | false | 缩进输出（默认 compact 一行，可管道给 `jdan json pretty`） |

输入：文件参数或 stdin（跟其他 json 子命令一致）。

## 行为细节 / 边界

- **数组**：`[i]` 下标；顶层数组 → `[0]`/`[1]`。
- **顶层标量 / 空容器**：flatten 成单键（`5` → `{"":5}`，`{}` → `{"":{}}`），unflatten 还原。
- `unflatten` 输入必须是**扁平对象**（非对象报错）。
- **冲突检测**：unflatten 时若同一前缀既当对象又当数组（`a.b` 和 `a[0]` 并存）→ 清晰报错。
- **稀疏数组**：`{"x[2]":9}` → `[null,null,9]`（中间补 null）。
- **已知局限**：键里**本身含 `sep`**（如键名就叫 `a.b`）→ round-trip 歧义；换 `--sep` 避开。
- 无效 JSON / 非法下标（`a[x]`）→ 报错。

## 内部架构

```
internal/jsonx/flatten.go
  Flatten(node, sep) map[string]any        —— 递归压扁，空容器当叶子
  Unflatten(flat, sep) (any, error)        —— splitFlatKey 拆段 + setPath 递归建嵌套
  FlattenBytes / UnflattenBytes            —— 解码(UseNumber)→转换→编码
internal/cli/json_flatten.go               —— flatten/unflatten 子命令挂到 jsonCmd
```

复用 `runJSONFormat`（现有 json 子命令的 file/stdin 输入 + 输出 helper）。

## 测试

- `internal/jsonx`：Flatten（基本 / 空容器保留 / 顶层数组 / 顶层标量 / `--sep` / 大整数精度）；Unflatten（还原对象 / 还原数组 / 稀疏补 null / 对象-数组冲突报错 / 非法下标报错）；**round-trip**（一组多样输入 `Unflatten(Flatten(x)) == x`，含空容器/大整数/深嵌套/顶层数组与标量，核心）；Bytes 包装（compact/pretty/非对象报错/无效 JSON）
- `internal/cli`：flatten stdin / `-p` / `--sep` / unflatten / 非对象报错 / round-trip（CLI 链）/ 无效 JSON 报错

## 退出码

| 状况 | exit code |
|------|-----------|
| 成功 | 0 |
| 无效 JSON / unflatten 非对象 / 路径冲突 / 非法下标 | 1 |

## 有意不做

| 候选 | 原因 |
|------|------|
| 全 `.` 方言（数组也用 `a.0`） | 用 `[i]` 跟 `json path` 一致更有价值 |
| 键含 sep 的转义 | 第一版用 `--sep` 规避；转义复杂度高 |

## TL;DR

1. `jdan json flatten` —— 嵌套 → 扁平点分键（`a.b` / `a.c[0]`）
2. `jdan json unflatten` —— 反向还原
3. 键格式 = `jdan json path` 表达式，工具链一致
4. round-trip 还原（空容器保留 + 大整数精度），`--sep` 改分隔符，`-p` 缩进
5. **0 新依赖**，复用现有 json 输入/解码约定
