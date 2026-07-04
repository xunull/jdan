# jdan spt9

把中文翻成【**小鹤双拼**】在九宫格键盘上的按键 —— 每个字**固定按 2 下**。跟 [`jdan t9`](jdan-t9.md)（全拼九宫格，每字不定长按键）互补。

## 原理

双拼的核心:一个音节 = **一键声母 + 一键韵母**,共 2 码。比 t9 多一步「拼音 → 双拼两码」:

```
中文 ─(go-pinyin)→ 拼音 ─(小鹤方案)→ 两码字母 ─(T9键位)→ 每字2个数字键
中                  zhong             zh→v, ong→s = "vs"     v(8) s(7) = 87
```

小鹤(flypy)方案:声母 `zh/ch/sh` 分别记作 `v/i/u`,其余声母=自身;韵母按固定表映射到单个字母(如 `an→j`、`ong→s`、`iang→l`)。**这张表照 [RIME `rime-double-pinyin`](https://github.com/rime/rime-double-pinyin) 的 `double_pinyin_flypy.schema.yaml` 逐条写死**,不是凭记忆——所以准。得到两个字母后,各自落到它的**标准 T9 键**(复用 `t9x`)。

> 「T9 键位的小鹤」= 小鹤两码字母各自落在标准九宫格键上。官方实例:拼音 `dan` → 小鹤 `dj` → 按 3 键(def)+ 5 键(jkl)。这条被单测钉死。

零声母字(`a/o/e` 开头、没辅音声母)按小鹤规则:首字母 + 韵母键,如 `安 an → aj`、`饿 e → ee`、`爱 ai → ad`。

## 用法

```bash
jdan spt9 中文            # 逐字对照 + 底部整串
jdan spt9 "你好世界"      # 每字 2 键
echo 中国 | jdan spt9     # 管道
jdan spt9 中文 --digits   # 只出数字串（可管道）
jdan spt9 中文 --json     # 机读
```

输出示例:

```
$ jdan spt9 "你好世界 hi"
你  ni   ni  64
好  hao  hc  42
世  shi  ui  84
界  jie  jp  57
hi  —  —   44
─────
64 42 84 57 44
```

每行:汉字 · 拼音 · 小鹤两码 · 数字键。英文按普通 T9 字母映射(拼音/两码列显 `—`)、阿拉伯数字原样、空格/标点跳过、其它无法映射字符跳过并计数(走 stderr)。

**跟 `jdan t9` 的区别**(同一句 `中国`):

| | 按键 | 说明 |
|---|---|---|
| `jdan t9`(全拼) | `94664 486` | 每字按全拼字母,不定长 |
| `jdan spt9`(双拼) | `87 46` | 每字**恒 2 键** |

### Flags

| flag | 默认 | 说明 |
|------|------|------|
| `--digits` | false | 只输出整串数字 |
| `--json` | false | 结构化输出(`units[]` 含 `code` 两码 + `skipped`) |

无参数时从 stdin 读。

## 局限

- **只做小鹤方案**。米旮旯/辜氏等「专用九键双拼」是另一套直接铺 8 键的键位,不在此(可日后 `--scheme` 扩)。
- **多音字取最常见读音**(go-pinyin 默认),个别可能不准(如「行」默认 xíng)。
- 只做「每字确定性两码按键」,不是真实输入法(无候选/词频/整句)。

## 实现

```
internal/spt9x/spt9x.go   Encode(拼音)→小鹤两码 + Result 渲染   纯逻辑，0 依赖
internal/cli/spt9.go      切词 + 边缘调 go-pinyin，复用 t9 的字母→数字/切词工具
```

- **小鹤表来自权威源**:`finalLetter` 韵母表逐条对齐 RIME flypy schema;单测用官方实例 `dan→dj` 等钉死,另有真实 go-pinyin 集成测(`你好世界 → 64 42 84 57`)兜底。
- **复用**:`t9x.LettersToDigits`(字母→数字)、`t9.go` 的 `realPinyin`/`isHan` 等,不重复。

跟 `jdan t9`（全拼九宫格）、`jdan morse`、`jdan alpha` 同属「文字 ↔ 编码」一类。
