# jdan json merge

深度合并多个 JSON 文档（后者覆盖前者）。配置分层（base + override + local）、合 fixture 高频。0 新依赖（纯 stdlib）。`jdan json` 下的子命令。

## 它能干什么

```bash
$ jdan json merge base.json override.json
# base:     {"a":1,"nest":{"x":1}}
# override: {"b":2,"nest":{"y":2}}
{"a":1,"b":2,"nest":{"x":1,"y":2}}
```

`nest` 是**递归合并**（`{x:1,y:2}`），不是整体替换——这是和 `cat` 拼接或浅覆盖的关键区别。

## 合并语义

- **对象**：递归合并，键取并集；同一键两边都是对象 → 继续递归，否则**后者覆盖**。
- **数组**：`--arrays` 策略：
  - `replace`（默认）：后者整段替换前者。
  - `append`：两段拼接。
- **标量 / 类型不一致**：后者覆盖（`{"a":{"x":1}}` ⊕ `{"a":5}` → `{"a":5}`）。
- **null**：当普通值，**后者的 null 覆盖**前者（不做「null = 删键」，更可预测）。
- 多个输入**从左到右**依次合：`merge a b c` = `((a⊕b)⊕c)`。

## 用法

```bash
jdan json merge <file> <file> [file...]      # ≥2 个输入
jdan json merge base.json override.json
jdan json merge a.json b.json --arrays append
jdan json merge *.json -p                    # 多个依次合 + 缩进
cat base.json | jdan json merge - override.json   # - = stdin（最多一次）
```

## flags

| flag | 默认 | 用途 |
|------|------|------|
| `--arrays` | `replace` | 数组策略：`replace`（后者替换）/ `append`（拼接） |
| `-p, --pretty` | false | 缩进输出（默认 compact 一行） |

输入：≥2 个文件参数；`-` 代表 stdin（最多用一次），方便管道。

## 例：配置分层

```bash
# 默认配置 + 环境 override + 本地 override，后者优先
$ jdan json merge defaults.json prod.json local.json -p
```

## 边界 / 错误

- **保大整数精度**（`UseNumber`，跟其他 json 子命令一致）。
- 输入 < 2 个 → 报错（合并至少要两个）。
- 任一输入无效 JSON → **指明是第几个输入** + 报错；文件不存在 → 报错。
- 顶层可以是对象/数组/标量（数组按 `--arrays`，标量后者覆盖）。
- `--arrays` 非法值 / 两次 `-` → 报错。
- **不改输入**：`Merge` 返回新结构，不就地改两边（有测试守护）。

## 内部架构

```
internal/jsonx/merge.go
  Merge(a, b, strat) any                    —— 递归深合并（纯函数，不改输入）
  MergeAll(docs, strat, indent) ([]byte, error)  —— 多文档左→右合并
  ParseArrayStrategy(s) (ArrayStrategy, error)
internal/cli/json_merge.go                  —— merge 子命令（- = stdin）挂到 jsonCmd
```

复用 flatten/unflatten 引入的 `decodeJSON(UseNumber)` / `marshalJSON`。

## 测试

- `internal/jsonx`：Merge（对象并集 / 嵌套递归合 / 标量覆盖 / 类型不一致后者赢双向 / 数组 replace / 数组 append / 顶层数组 append / null 覆盖 / 只在 a 的键保留 + 只在 b 的键加入 / 深嵌套 / **不改输入**）；MergeAll（3 文档左→右 / 大整数精度 / 第 n 个无效 JSON 报错且指明序号）；ParseArrayStrategy（replace/empty/append/非法）
- `internal/cli`：基本合并 / `--arrays append` / `-p` / `-`=stdin / 双 stdin 报错 / <2 输入报错 / 无效 JSON 报错 / 文件不存在报错 / 非法 `--arrays` 报错

## 退出码

| 状况 | exit code |
|------|-----------|
| 成功 | 0 |
| <2 输入 / 无效 JSON / 文件不存在 / 非法 flag | 1 |

## 有意不做

| 候选 | 原因 |
|------|------|
| `--arrays union`（去重） | 元素相等判定模糊；replace/append 够用 |
| null = 删键 | 不直观、易踩坑；后者 null 当普通覆盖更可预测 |
| JSON Merge Patch (RFC 7386) | 另一套语义；deep-merge 覆盖日常场景 |

## TL;DR

1. `jdan json merge a.json b.json` —— 深度合并，后者覆盖前者
2. 对象递归合并（不是整体替换），数组 `--arrays replace/append`
3. 多文档左→右、`-`=stdin、`-p` 缩进、大整数精度
4. null 当普通覆盖、类型不一致后者赢、不改输入
5. **0 新依赖**，复用现有 json 解码约定
