# jdan entropy

算一段字符串/文件/stdin 的 **Shannon 熵**（字节分布的信息量，0–8 bits/byte）。判断数据是否加密/压缩/随机、找高熵 secret、估可压缩性。0 新依赖（纯 `math`）。

## 它能干什么

```bash
$ jdan entropy "hello world"
bytes:    11
entropy:  2.85 bits/byte   (低：文本/结构化)
total:    31.3 bits
distinct: 8 / 256 字节值

$ head -c 4096 /dev/urandom | jdan entropy | head -2
bytes:    4096
entropy:  7.96 bits/byte   (极高：疑似加密/压缩/随机)
```

## 「熵」的两种含义（本命令锚定哪个）

| | 是什么 | 本命令 |
|---|---|---|
| **Shannon 熵**（数据分布）| `H = -Σ p·log2(p)`，0–8 bits/byte | ✅ 核心 |
| **密码强度 bits**（搜索空间）| `长度 × log2(字符集)`，另一套算法 | ⚠ `--charset` 可选估算 |

短密码 `Password1` 的 Shannon 熵/字节看着不低但它弱——强度该用搜索空间 bits + 字典检查（zxcvbn 那套，要引库）。所以 `jdan entropy` 核心给**数据随机性**这个严格量，不冒充强度评分。

**标签**（bits/byte）：`<1` 极低（高度重复）· `1–4` 低（文本/结构化）· `4–6` 中 · `6–7.5` 高 · `≥7.5` 极高（疑似加密/压缩/随机）。

## 输入

| 形式 | 说明 |
|------|------|
| `jdan entropy "<string>"` | 量这段字符串的字节 |
| `jdan entropy -f <path>` | 量文件 |
| `echo ... \| jdan entropy` | stdin |

## 滑窗 sparkline（找高熵区段）

`--window <bytes>` 把输入切块、逐块算熵，画一行 sparkline，**一眼看出固件/二进制里被压缩/加密的那段**：

```bash
$ jdan entropy -f firmware.bin --window 512
▁▁▂▃█████▇▆▂▁
峰值 7.97 @ 偏移 0x1A00
```

> 注意：窗口越小，熵上限越低（N 字节窗口熵 ≤ log2(N)）。16 字节窗口最高 4 bits/byte，512 字节才有意义的高熵分辨力。

## 搜索空间 bits（可选，非强度评分）

`--charset` 额外给一行 `长度 × log2(字符集大小)`（检测小写/大写/数字/符号/空格），**明确标注是理论搜索空间，不是强度评分**：

```bash
$ jdan entropy "Tr0ub4dour" --charset
... (Shannon 行)
charset:  62 符号集 ≈ 59.5 bits（搜索空间，非强度评分）
```

## flags

| flag | 默认 | 用途 |
|------|------|------|
| `-f, --file` | — | 量文件而非位置参数字符串 |
| `--window` | 0（整体） | 滑窗字节数，开启 sparkline |
| `--charset` | false | 附加搜索空间 bits 估算 |
| `--json` | false | 结构化（bytes/bits_per_byte/total_bits/label/distinct/chunks/charset） |

## 内部架构 & 可测性

```
internal/entropyx/entropyx.go
  Shannon(data) float64                 —— -Σ p·log2(p)（纯函数）
  Analyze(data, window) Result          —— 整体 + 分块 + 峰值
  CharsetBits(s) (int, float64)         —— 搜索空间 bits 估算
  Sparkline(chunks) string
  (Result).FormatText / .FormatJSON
internal/cli/entropy.go
```

`Shannon` 喂已知向量断言（不依赖随机性，可复现）：全同字节→0；两值各半→1.0；全 256 均匀→8.0；`"hello world"`→≈2.845。

## 测试

- `internal/entropyx`：Shannon（空/单值/两值半/256 均匀/已知文本）；Analyze（整体/分块数/尾块/峰值偏移）；标签分档；Sparkline（边界 ▁/█、单调递增）；CharsetBits（纯小写 26/加数字 36/全类 94/空）；FormatJSON 合法 + FormatText charset 行开关
- `internal/cli`：string / stdin / `-f` 文件 / `--window` sparkline / `--charset` / `--json` / 空输入报错

## 退出码

| 状况 | exit code |
|------|-----------|
| 成功 | 0 |
| 空输入 / 文件读不了 | 1 |

## 有意不做

| 候选 | 原因 |
|------|------|
| zxcvbn 式真·密码强度（字典/模式）| 要引库（违 0 依赖）；charset bits 够给个估算 |
| 熵图 PNG/SVG | 终端 sparkline 够用 |
| 逐 rune（Unicode）熵 | 标准「文件熵」是字节级，更通用 |

## TL;DR

1. `jdan entropy "<string>"` / `-f file` / stdin —— 算 Shannon 熵（bits/byte）
2. 高 ≥7.5 ≈ 加密/压缩/随机，低 ≈ 文本/重复
3. `--window` 滑窗 sparkline 找高熵区段，`--charset` 估搜索空间 bits
4. **严格 Shannon 定义**，不冒充密码强度评分
5. **0 新依赖**，纯 math，跟 mime/hash/pem 同属检视器簇
