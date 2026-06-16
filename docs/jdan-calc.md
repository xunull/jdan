# jdan calc

命令行算术表达式计算器。手写递归下降解析器，支持四则运算 / 幂 / 取模 / 括号 / 一元负号 / 函数 / 常量 / 进制操作数。0 新依赖（纯 stdlib `math` + `strconv`）。

## 它解决什么问题

命令行快算一个表达式，常见做法都别扭：

- `python3 -c "print(3 * (4 + 5) / 2)"` —— 启动慢、要记 print
- `echo "3 * (4 + 5) / 2" | bc -l` —— bc 报错难懂、没有 `^` 幂（要 `^` 但语义不同）、默认不带小数
- shell `$((...))` —— 只有整数、没有函数

`jdan calc` 一行搞定：

```bash
$ jdan calc "3 * (4 + 5) / 2"
13.5
```

## 支持的运算

| 运算 | 符号 | 说明 |
|------|------|------|
| 加减乘除 | `+ - * /` | |
| 取模 | `%` | 浮点取模（`math.Mod`） |
| 幂 | `^` 或 `**` | **右结合**（`2^3^2 = 2^9 = 512`） |
| 括号 | `( )` | 改变优先级 |
| 一元负号 | `-5` | 负数 / 取负 |

优先级（高→低）：`( )` > 函数 > `^` > `* / %` > `+ -`，一元 `-` 紧绑操作数。

```bash
$ jdan calc "2 + 3 * 4"      # * 高于 +
14
$ jdan calc "2 ^ 3 ^ 2"      # ^ 右结合
512
$ jdan calc "-5 + 3"         # 表达式可以负号开头
-2
$ jdan calc "2 * -3"         # 一元负号
-6
```

## 函数 + 常量

| 类别 | 名称 |
|------|------|
| 单参函数 | `sqrt` `abs` `floor` `ceil` `round` `ln` `log10` `sin` `cos` `tan` |
| 可变参数 | `min(a, b, ...)` `max(a, b, ...)` |
| 常量 | `pi` `e` `tau` |

函数名 / 常量名 **大小写不敏感**，可嵌套。

```bash
$ jdan calc "sqrt(2)"
1.4142135623730951
$ jdan calc "pi * 2"
6.283185307179586
$ jdan calc "max(3, 7, 2)"
7
$ jdan calc "sqrt(abs(-16))"
4
```

## 进制操作数

操作数可带 `0x` / `0b` / `0o` 前缀（跟 `jdan num` 呼应）：

```bash
$ jdan calc "0xFF + 1"
256
$ jdan calc "0b1010 * 2"
20
$ jdan calc "0o755"
493
```

## 输出格式

默认**智能显示**：整数结果不带 `.0`，小数用最短往返表示。

```bash
$ jdan calc "10 / 5"
2
$ jdan calc "10 / 4"
2.5
$ jdan calc "2 ^ 0.5"
1.4142135623730951
```

flags：

| flag | 用途 |
|------|------|
| `--hex` | 结果以十六进制输出（要求非负整数） |
| `--bin` | 二进制输出（要求非负整数） |
| `--precision N` | 固定 N 位小数 |
| `--json` | 结构化输出 |

```bash
$ jdan calc "255 + 1" --hex
0x100
$ jdan calc "10 / 3" --precision 2
3.33
$ jdan calc "2 ^ 8" --json
{"expr":"2 ^ 8","result":256}
```

`--hex` / `--bin` 对非整数结果会报错（`hex/bin output requires an integer result`）。

## 输入方式

```bash
jdan calc "3 * (4 + 5)"      # 加引号（推荐，避免 shell 解析 * 等）
jdan calc 2 + 3              # 多 arg 自动拼接
echo "1 + 2 * 3" | jdan calc # stdin
```

> 注意：表达式以 `-` 开头（如 `-5 + 3`）在很多 CLI 里会被误当 flag。`jdan calc`
> 关掉了 cobra 的 flag 解析、自己分离 flag 和表达式，所以负号开头正常工作。

## 错误处理

错误带**位置信息**，比 `bc` 友好：

```bash
$ jdan calc "1 +"
Error: unexpected end of expression (expected operand)
$ jdan calc "1 / 0"
Error: division by zero
$ jdan calc "2 @ 3"
Error: unexpected character "@" at position 2
$ jdan calc "(1 + 2"
Error: expected ')' at position 6, got end of expression
$ jdan calc "sqrt(-1)"
Error: sqrt of negative number
$ jdan calc "foo(2)"
Error: unknown function "foo"
```

## 跟 jdan num 的边界

`jdan calc` 做**算术 + 函数**；**位运算**（AND / OR / XOR / shift）归 `jdan num bit`，两者不重叠：

```bash
jdan calc "0xFF * 2" --hex     # 算术 + 进制输出 → 0x1FE
jdan num bit "0xFF AND 0x0F"   # 位运算 → 0xF
```

## 内部架构

经典**递归下降解析器**（recursive descent）：

```
internal/calc/
  token.go    tokenize（数字含进制前缀 / 运算符 / 括号 / 标识符）
  ast.go      AST 节点（NumNode / ConstNode / BinNode / UnaryNode / CallNode）
  parser.go   parseExpr → parseTerm → parseUnary → parsePower → parseAtom
  eval.go     Eval(AST) → float64；函数表 + 常量表

internal/cli/calc.go    DisableFlagParsing + 手动 flag 分离（处理负号开头）
```

文法（优先级低→高）：

```
expr   := term  (('+' | '-') term)*
term   := unary (('*' | '/' | '%') unary)*
unary  := '-' unary | power
power  := atom ('^' unary)?        // ^ 右结合
atom   := number | const | ident '(' args ')' | '(' expr ')'
```

**0 新依赖**，纯 stdlib。

## 测试

- 14 unit test 组 on `internal/calc`（每组多断言）：
  - 基本算术 + 优先级（含 ^ 右结合 / / 左结合）
  - 一元负号（含双重负号 / 负指数）
  - `**` 别名 / 进制操作数 / 小数 / 科学计数
  - 函数（含嵌套 / 大小写不敏感）+ 常量
  - 错误（缺操作数 / 除零 / 模零 / 非法字符 / 括号不匹配 / 未知函数/常量 /
    单参函数多参 / 空表达式）
- 16 CLI tests：基本 / 整数显示 / **负号开头** / 多 arg 拼接 / 函数 /
  --hex/--bin / 非整数 hex 报错 / --precision / --json / stdin / 互斥 flag

## 退出码

| 状况 | exit code |
|------|-----------|
| 成功 | 0 |
| 解析错误 / 除零 / 函数域错误 / --hex 非整数 | 1 |

## 有意不做的事

| 候选 | 不做原因 |
|------|---------|
| 位运算（AND / OR / shift） | 归 `jdan num bit`，不重叠 |
| 变量赋值（`x = 5; x * 2`） | 有状态，scope 大；第一版纯求值 |
| 单位换算（`5km + 3mi`） | 单独大 scope |
| 任意精度（bignum / 分数） | float64 覆盖日常；要精确用 python decimal |
| 矩阵 / 复数 / 微积分 | 远超 CLI 快算 scope |

## TL;DR

1. `jdan calc "3 * (4 + 5) / 2"` —— 四则 + 幂（^ 右结合）+ 取模 + 括号
2. 函数（sqrt/abs/min/max/...）+ 常量（pi/e/tau），大小写不敏感可嵌套
3. 进制操作数 `0xFF + 1`，`--hex` / `--bin` 输出
4. **负号开头正常工作**（关掉 cobra flag 解析自己分离）
5. 错误带位置信息，比 bc 友好；位运算归 `jdan num bit`
6. **0 新依赖**，手写递归下降 parser
