# jdan figlet

把文字渲染成 ASCII art 大横幅（figlet 风格）。0 新依赖（内置字体，纯 stdlib）。跟 `jdan qr`（生成二维码）同属"把文字变成视觉输出"的小工具家族。

## 它能干什么

```bash
$ jdan figlet "jdan"
  ### ####   ###  #   #
   #  #   # #   # ##  #
   #  #   # ##### # # #
#  #  #   # #   # #  ##
 ##   ####  #   # #   #
```

给 CLI 输出加标题、做 section 分隔、README banner、终端 MOTD、脚本里的步骤提示。

## 用法

```bash
jdan figlet "Hello"
jdan figlet Deploy OK              # 多 arg 自动拼接
jdan figlet "READY" --font block   # 实心块字体
jdan figlet "Title" --center --width 60
echo "Build Done" | jdan figlet    # stdin
jdan figlet --list                 # 列出字体
```

## 字体

内置 2 种字体（`--list` 查看）：

| 字体 | 风格 | 例 |
|------|------|-----|
| `standard`（默认） | `#` 描边，5 行高 | `#   #` |
| `block` | 实心块 `█` | `█   █` |

```bash
$ jdan figlet "OK" --font block
 ███  █   █
█   █ █  █
█   █ ███
█   █ █  █
 ███  █   █
```

覆盖 **A-Z / a-z / 0-9 / 空格 / 常见标点**（`! ? . , : ; - _ + = * / \ ( ) [ ] < > # @ $ % & ' "`）。小写**折叠成大写**（banner 字体惯例）。不支持的字符（如 CJK）用空白占位，不报错。

## flags

| flag | 默认 | 用途 |
|------|------|------|
| `--font` | standard | 字体（standard / block） |
| `--width` | 80 | 最大宽度，超过自动换到下一"块"（多行堆叠）；`0` = 不换行 |
| `--center` | false | 在 `--width` 内居中 |
| `--list` | false | 列出内置字体 |

### `--width` 自动换行

文字太长超过 `--width` 时，自动换到下一块（垂直堆叠）：

```bash
$ jdan figlet "ABCDEFGHIJ" --width 30
[前 5 个字母一块]
[后 5 个字母一块]
```

### `--center` 居中

```bash
$ jdan figlet "HI" --center --width 40
            #   # ###
             #   #  #
             #####  #
             #   #  #
            #   # ###
```

## 输入方式

```bash
jdan figlet "multi word"      # 加引号
jdan figlet multi word        # 多 arg 自动拼接（空格连接）
echo "piped" | jdan figlet    # stdin
```

## 跟现有命令的关系

跟 `jdan qr`（文字 → 二维码）一样属于"把文字变成视觉输出"的家族：

```bash
# 脚本里做 section 分隔
jdan figlet "STEP 2"

# 给 http serve 的就绪提示加 banner
jdan figlet "READY" --font block
```

## 内部架构

```
internal/figlet/
  fonts.go       Font 结构 + Lookup / FontNames + registerBitmap（字模规整）
  fonts_data.go  standardGlyphs（5 行 # bitmap，A-Z/0-9/标点）
  render.go      Render（逐字符取字模、横向拼接、--width 换行、--center 居中）

internal/cli/figlet.go
```

**字体设计**：`standard` 是手写的 5 行 `#` bitmap 字模；`block` 复用同一套字模，把 `#` 换成实心块 `█`（一套字模派生两种字体）。`█` 是 3 字节 UTF-8，渲染时按 **rune 计数**对齐（不是 byte），保证多字节字符不破坏列对齐。

**0 新依赖**：不加载外部 `.flf` 字体文件，字模是 Go 源码常量。

## 测试

- 18 unit tests on `internal/figlet`：
  - Lookup 大小写不敏感 / FontNames
  - Glyph 小写折叠 / 不支持字符返回 nil
  - Render 高度 / 字母形状 / 多字符 / 数字 / 未知字体 / 空文本
  - **block 字体 █ 多字节不 panic**（byte/rune 混用回归）
  - `--width` 换行（行数 = 字体高度倍数）/ `--center` 前导空格
  - 不支持字符空白占位
  - 字体数据完整性（A-Z + 0-9 全覆盖 / 每字模 5 行）
- 9 CLI tests：基本 / 多 arg / block / stdin / --list / 未知字体 / 空文本 / center

## 退出码

| 状况 | exit code |
|------|-----------|
| 成功 | 0 |
| 未知字体 / 空文本 | 1 |

## 有意不做的事

| 候选 | 不做原因 |
|------|---------|
| 加载外部 `.flf` 字体文件 | 引文件依赖 + 解析 .flf 格式复杂；内置精选字体够用 |
| 几百种字体 | 体积爆炸；2 种覆盖常用场景，未来可加 |
| 颜色 / 渐变 | 第一版纯 ASCII；可配合 `lolcat` 等管道上色 |
| Unicode / CJK 大字 | ASCII 字模即可；CJK 字模工作量巨大 |

## TL;DR

1. `jdan figlet "text"` —— 文字 → ASCII art 大横幅
2. `--font standard`（`#` 描边）/ `block`（实心块 `█`）
3. `--width` 自动换行，`--center` 居中
4. 多 arg 拼接 / stdin；小写折叠大写；不支持字符空白占位
5. **0 新依赖**，内置字模常量；跟 `jdan qr` 同属"文字→视觉输出"家族
