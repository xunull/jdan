# jdan morse

文本 ↔ 国际摩斯电码（ITU）互转。**自动判方向**。学习/解谜/玩。0 新依赖（一张查表）。

## 它能干什么

```bash
$ jdan morse "SOS"
... --- ...

$ jdan morse "... --- ..."          # 自动认出是摩斯码 → 解码
SOS

$ jdan morse "Hello World"
.... . .-.. .-.. --- / .-- --- .-. .-.. -..
```

## 自动判方向

输入**只含** `.`/`-`/`/`/空格 → **解码**；否则 → **编码**。`--encode` / `-d, --decode` 可强制（极短/有歧义输入用，如 `.` 既是句号字符又是字母 E 的码）。

```bash
$ jdan morse "." --encode    # 当句号字符编码
.-.-.-
$ jdan morse "." -d          # 当摩斯码解码
E
```

## 规则

- 字母间**单空格**，单词间 **` / `**（解码兼容 `/`、多空格折叠）。
- 大小写无关（摩斯无大小写），解码输出统一**大写**。
- 字符集：A–Z、0–9 + 标准标点（`. , ? ' ! / ( ) & : ; = + - _ " $ @`，ITU 规定）。

## 未知字符

| 方向 | 处理 |
|------|------|
| 编码遇到无法映射的字符（中文/emoji） | **跳过**，stderr 提示「跳过 N 个无法编码的字符」 |
| 解码遇到无法识别的码 | 输出 `#` 占位 + 计数（stderr） |

计数走 **stderr**，stdout 保持干净，可安全管道。

## round-trip

可映射字符 + 空格 **encode→decode 还原**（大小写丢失，统一大写）。有测试守护。

## 命令形态 & flags

```bash
jdan morse "<text or morse>"   # 自动判方向
echo "SOS" | jdan morse        # stdin
jdan morse "E" --encode        # 强制编码
jdan morse "...." -d           # 强制解码
jdan morse "SOS" --json        # {direction, output}
```

| flag | 用途 |
|------|------|
| `--encode` / `-d, --decode` | 强制方向（默认自动判；二者互斥） |
| `--json` | 结构化 `{direction, output}` |

## 内部架构 & 可测性

```
internal/morsex/morsex.go
  Encode(s) (string, skipped int)
  Decode(s) (string, unknown int)
  LooksLikeMorse(s) bool
  （forward 字符→码 / reverse 码→字符 两张表）
internal/cli/morse.go
```

纯函数，固定向量断言，完全可复现。

## 测试

- `internal/morsex`：Encode（SOS / 大小写归一 / 数字+标点 / 单词 `/` 分隔 / 未知跳过+计数 / 空）；Decode（SOS / `/`→空格 / 多空格容错 / 未知码→`#`+计数）；**round-trip**（一组文本 encode→decode == 大写原文）；LooksLikeMorse（摩斯/文本/空/混合）
- `internal/cli`：编码 / 自动解码 / `--encode`（`.`→`.-.-.-` 破歧义）/ `-d` / stdin / **skip 提示走 stderr 不污染 stdout** / `--json` / 空报错

## 退出码

| 状况 | exit code |
|------|-----------|
| 成功（含跳过/未知，仅 stderr 提示） | 0 |
| 空输入 / `--encode` 与 `--decode` 同时给 | 1 |

## 有意不做

| 候选 | 原因 |
|------|------|
| `--play` 蜂鸣 / `--blink` 灯闪 | 终端 beep 不可移植、gimmick；纯文本转换更通用 |
| `·`/`–` Unicode 美化 | 边际价值低；`.`/`-` 是标准写法 |
| 非英文摩斯（西里尔/和文摩斯） | 第一版聚焦 ITU 国际码 |

## TL;DR

1. `jdan morse "<text>"` 编码，`jdan morse "<.- 码>"` 自动解码
2. `--encode`/`-d` 破歧义，字母空格、单词 ` / `
3. 未知字符跳过/`#`，计数走 stderr（stdout 干净）
4. round-trip 还原（大小写丢失）
5. **0 新依赖**，纯查表，跟 figlet/qr 同属玩味一类
