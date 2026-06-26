# jdan uuid

检视一个 UUID：版本、variant、v1/v7 内嵌时间戳、字节、URN 形式、nil/max。0 新依赖（纯 stdlib）。生成走 `jdan uuid new`（复用 `jdan rand uuid` 的实现，不重复造）。

## 它能干什么

```bash
$ jdan uuid 0190a1b2-c3d4-7e5f-8a9b-1c2d3e4f5a6b
canonical: 0190a1b2-c3d4-7e5f-8a9b-1c2d3e4f5a6b
version:   7 (时间排序)
variant:   RFC 4122
time:      2026-06-26 14:00:00.000 UTC
bytes:     01 90 a1 b2 c3 d4 7e 5f 8a 9b 1c 2d 3e 4f 5a 6b
urn:       urn:uuid:0190a1b2-c3d4-7e5f-8a9b-1c2d3e4f5a6b

$ jdan uuid 3f2504e0-4f89-41d3-9a0c-0305e82c3301   # v4 无时间戳
version: 4 (随机)  variant: RFC 4122
```

## 与现有的关系

| 能力 | 归属 |
|------|------|
| **生成** v4/v7 | `jdan rand uuid`（`internal/randgen`，CSPRNG） |
| **检视/解析** | `jdan uuid <uuid>`（`internal/uuidx`，本命令） |
| 便捷生成入口 | `jdan uuid new`（薄封装，直接调 `randgen`，零逻辑重复） |

## 解析细节

- **输入容错**：canonical `8-4-4-4-12`、`urn:uuid:` 前缀、`{花括号}`、无连字符 32 hex、大小写——都规整成 16 字节。
- **version** = byte[6] 高 4 位（1–8；全 0 为 nil）。
- **variant**（byte[8] 高位，RFC 9562 §4.1）：`NCS(0xxx)` / `RFC 4122(10xx)` / `Microsoft(110x)` / `Reserved(111x)`。
- **时间戳**：
  - **v7** → 前 48 bit = unix 毫秒。
  - **v1** → 60-bit 100ns 自 1582-10-15（Gregorian 偏移 `122192928000000000`）。
  - 其余版本（3/4/5/8）无时间戳。
- **nil**（全 0）/ **max**（全 0xFF，RFC 9562）特殊标注。

## 用法

```bash
jdan uuid <uuid>            # 检视
echo "$U" | jdan uuid       # stdin
jdan uuid "$U" --json       # 结构化
jdan uuid new [--v7] [-n N] # 生成（复用 rand）
```

## flags

| flag | 作用 |
|------|------|
| `--json` | 结构化输出（canonical/version/variant/time/nil/max） |
| `new --v7` | 生成 v7（默认 v4） |
| `new -n N` | 批量生成 N 个 |

## 内部架构 & 可测性

```
internal/uuidx/uuidx.go
  Parse(s) (Info, error)            —— 规整 + 版本/variant/时间戳解析（纯函数）
  (Info).FormatText / .FormatJSON
internal/cli/uuid.go
  jdan uuid <uuid>                  —— 检视
  jdan uuid new                     —— 复用 randgen.GenerateUUIDv4/v7
```

`Parse` 纯函数；v1/v7 时间戳用**程序化构造的已知向量**断言（不依赖当前时间，可复现）。

## 测试

- `internal/uuidx`：v4/v1/v3/v5/v7/v8 版本识别；variant（RFC/NCS/MS/Reserved）；nil / max；v7 时间戳（已知毫秒）；v1 时间戳（已知时间往返，容 100ns 量化）；输入容错（urn/花括号/无连字符/大小写/空格）；长度错·非 hex 报错；FormatText/FormatJSON
- `internal/cli`：检视 text / stdin / `--json` / 非法报错 / 无输入报错 / `new`（v4 可被自身解析、`--v7` + `-n` 数量）

## 退出码

| 状况 | exit code |
|------|-----------|
| 成功 | 0 |
| 非法 UUID / 无输入 | 1 |

## 有意不做

| 候选 | 原因 |
|------|------|
| v1 的 MAC 节点解析 | 隐私 + 价值低；只给时间戳够用 |
| 重新实现生成 | 复用 `jdan rand uuid` 的 `randgen`，绝不重写 |

## TL;DR

1. `jdan uuid <uuid>` —— 拆开看版本/variant/时间戳/字节/URN
2. v7/v1 自动解出内嵌时间戳；nil/max 识别
3. 输入容错：urn 前缀 / 花括号 / 无连字符 / 大小写
4. `jdan uuid new [--v7] [-n N]` 生成（复用 `jdan rand uuid`）
5. **0 新依赖**，检视是全新能力，生成不重复造
