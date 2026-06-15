# jdan num

进制转换 + 位运算工具。主命令自动检测输入进制，一次性输出 dec/hex/bin/oct + 位信息；`bit` 子命令做位运算。uint64 范围，0 新依赖（纯 stdlib `strconv` + `math/bits`）。

## 它解决什么问题

开发者每天要在进制间转换：

- 看寄存器值（`0xDEADBEEF` 是多少十进制？）
- Unix 权限位（`0o755` 的二进制是哪几位？）
- flag mask（`0x04 | 0x10` 等于啥？）
- 子网掩码、颜色值、协议字段...

掏计算器 / 开 python / 心算都慢。`jdan num` 一行出全部：

```bash
$ jdan num 0xDEADBEEF
Decimal:  3735928559
Hex:      0xDEADBEEF
Binary:   0b11011110101011011011111011101111
Octal:    0o33653337357
Bits:     24 set (...), width 32
```

## 主命令：进制转换

**自动检测输入进制**（按前缀）：

| 前缀 | 进制 | 例 |
|------|------|-----|
| `0x` / `0X` | 十六进制 | `0xFF` |
| `0b` / `0B` | 二进制 | `0b1010` |
| `0o` / `0O` | 八进制 | `0o755` |
| 前导 `0` + 数字 | 八进制（C 风格） | `0755` |
| 纯数字 | 十进制 | `255` |

```bash
$ jdan num 255
Decimal:  255
Hex:      0xFF
Binary:   0b11111111
Octal:    0o377
Bits:     8 set (0,1,2,3,4,5,6,7), width 8
```

`Bits:` 行显示 popcount（置位的 bit 数）、哪些位置位、最高有效位宽度。

下划线分隔符（跟 Go 字面量一致）：

```bash
$ jdan num 0xFF_FF
Decimal:  65535
...
```

### `--bits`：位展示

看 flag / mask 时方便：

```bash
$ jdan num 0b10110 --bits
Decimal:  22
Hex:      0x16
Binary:   0b10110
Octal:    0o26
Bits:     3 set (1,2,4), width 5
          bit:  4 3 2 1 0
          val:  1 0 1 1 0
```

### `--width`：二进制零填充

看寄存器对齐时用：

```bash
$ jdan num 0xFF --width 16
Binary:   0b0000000011111111
```

### `--json`

```bash
$ jdan num 255 --json
{
  "input": "255",
  "detected_base": "decimal",
  "decimal": 255,
  "hex": "0xFF",
  "binary": "0b11111111",
  "octal": "0o377",
  "bits_set": 8,
  "bit_width": 8
}
```

## bit 子命令：位运算

```bash
$ jdan num bit "0xFF AND 0x0F"
0x0F  (15, 0b1111)

$ jdan num bit "5 OR 2"
0x07  (7, 0b111)

$ jdan num bit "0b1010 XOR 0b0110"
0x0C  (12, 0b1100)

$ jdan num bit "1 << 8"
0x100  (256, 0b100000000)

$ jdan num bit "0xFF00 >> 4"
0xFF0  (4080, 0b111111110000)
```

**单目 NOT**（按 `--width` 取反，默认 64 位）：

```bash
$ jdan num bit "NOT 0xFF" --width 8
0x0  (0, 0b0)        # 8 位内 ~0xFF = 0x00

$ jdan num bit "~ 0x0F" --width 8
0xF0  (240, 0b11110000)
```

### 支持的运算符

| 运算符 | 别名 | 说明 |
|--------|------|------|
| `AND` | `&` | 按位与 |
| `OR` | `\|` | 按位或 |
| `XOR` | `^` | 按位异或 |
| `NOT` | `~` | 单目取反（按 `--width`） |
| `<<` | `SHL` | 左移 |
| `>>` | `SHR` | 右移 |

大小写不敏感。运算符可紧贴操作数（`1<<4`）或加空格（`1 << 4`）。也可以不加引号传多个 arg：`jdan num bit 0xFF AND 0x0F`。

`--json` 给脚本消费。

## 数值范围

用 **uint64**（64 位无符号），覆盖绝大多数寄存器 / 权限位 / flag mask / 子网掩码场景。

- 负数 → 清晰报错（`negative numbers not supported`）
- 超 uint64（> 18446744073709551615）→ 报错（`exceeds uint64 range`），不静默 wrap

## 跟现有命令的关系

`jdan num` 跟 `jdan hash` / `jdan b64` 同属"编码/进制"工具：

```bash
# Unix 权限位运算
jdan num 0o755                     # 看二进制
jdan num bit "0o755 AND 0o077"     # mask 计算 → 0o55

# 看颜色 / 协议字段的十进制
jdan num 0xFF0000                  # 红色通道
```

## 内部架构

```
internal/numconv/
  parse.go    DetectBase（前缀检测）+ ParseValue（→ uint64，溢出/负数报错）
  format.go   Convert（全进制 + popcount + bit width）/ SetBits / BinaryPadded / BitRows
  bitexpr.go  EvalBitExpr（tokenize + 二元 a op b / 单目 NOT，符号别名）

internal/cli/num.go   主命令（转换）+ bit 子命令（运算）
```

**0 新依赖**：`strconv` + `math/bits` 都是 stdlib。

## 测试

- 22 unit tests on `internal/numconv`：
  - DetectBase 全进制前缀 + 大小写
  - ParseValue max uint64 / 溢出 / 负数 / 非法 / 下划线
  - Convert 全进制对齐 + bit width 边界（0/1/256/0x80000000）
  - SetBits / BinaryPadded
  - EvalBitExpr 二元（AND/OR/XOR/shift + 符号别名）/ 单目 NOT / 紧贴 token /
    移位溢出 / 错误（未知运算符 / token 数 / NOT 误用 / 坏操作数）
- 15 CLI tests：
  - 主命令 dec/hex/oct 输入 / --bits / --width / --json / 非法 / 溢出
  - bit 子命令 AND/shift/NOT/multi-arg/JSON/非法

## 退出码

| 状况 | exit code |
|------|-----------|
| 成功 | 0 |
| 非法输入 / 溢出 / 负数 / 非法运算符 | 1 |

## 有意不做的事

| 候选 | 不做原因 |
|------|---------|
| 浮点 / IEEE 754 位分解 | 单独复杂场景；第一版聚焦整数 |
| 任意精度大整数（> 64 位） | uint64 覆盖 99% 寄存器/mask 场景；要 bignum 用 python |
| 有符号补码显示 | 可作为 `--signed` flag 未来加；第一版无符号 |
| 完整表达式（括号 + 优先级） | bit 子命令只做二元 `a op b` + 单目 NOT，不做完整解析器 |
| 字符 ↔ 码点 | 那是 `jdan char` 的范畴 |

## TL;DR

1. `jdan num <value>` —— 自动检测进制，一次出 dec/hex/bin/oct + 位信息
2. `--bits` 看 flag/mask 的位展示，`--width` 二进制零填充对齐
3. `jdan num bit "a AND b"` —— AND/OR/XOR/NOT/shift，符号别名 `& | ^ ~`
4. uint64 范围，负数/溢出清晰报错不 wrap
5. **0 新依赖**，纯 stdlib
