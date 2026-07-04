# jdan sp

把中文翻成**标准 26 键双拼**要按的字母键 —— 每个字**恒 2 键**（一键声母 + 一键韵母）。支持多套主流方案，还能 `--all` 一次把所有方案的结果摆出来对比。

跟 [`jdan spt9`](jdan-spt9.md)（九宫格双拼，出数字键）互补：这个是 26 键，出**字母键**。

## 原理

比全拼多一步「拼音 → 双拼两码」：

```
中文 ─(go-pinyin)→ 拼音 ─(所选双拼方案)→ 两码字母（=要按的键）
中                  zhong             小鹤：zh=v, ong=s → "vs"
```

双拼方案就是一张「声母/韵母 → 字母」的固定映射。本工具支持 5 套，**规则逐条照 [RIME `rime-double-pinyin`](https://github.com/rime/rime-double-pinyin) 各 schema 文件抄，非凭记忆**（官方实例 `dan → 小鹤 dj` 被单测钉死）：

| 方案 | id | 说明 |
|------|-----|------|
| 小鹤（默认） | `flypy` | 最流行 |
| 自然码 | `ziranma` | RIME 里的基础款 `double_pinyin` |
| 微软 | `mspy` | 搜狗双拼=此布局，`-s sogou` 即可 |
| 智能ABC | `abc` | |
| 拼音加加 | `pyjj` | |

各方案对同一音节可能不同，例如「中 zhong」：小鹤/自然码/微软都是 `vs`，智能ABC 是 `as`，拼音加加是 `vy`。

## 用法

```bash
jdan sp 中文                    # 默认小鹤，逐字对照 + 底部整串
jdan sp 中文 --scheme 微软       # 换方案（中文名或 id 都行）
jdan sp 中文 -s pyjj            # 简写
jdan sp 中文输入法 --all         # 所有方案一次对比 ★
jdan sp 中文 --codes            # 只出码串（可管道）
jdan sp 中文 --json             # 机读
echo 中国 | jdan sp             # 管道
```

默认（单方案）输出：

```
$ jdan sp "你好世界 hi"
你  ni   ni
好  hao  hc
世  shi  ui
界  jie  jp
hi  —    hi
─────
ni hc ui jp hi
```

`--all` 对比（一眼看出方案差异）：

```
$ jdan sp 中文输入法 --all
小鹤      vs wf uu ru fa
自然码    vs wf uu ru fa
微软      vs wf uu ru fa
智能ABC   as wf vu ru fa
拼音加加  vy wr iu ru fa
```

英文按**字母本身**（26 键上你就是那么按的）、阿拉伯数字原样、空格/标点跳过、其它无法映射字符跳过并计数（走 stderr）。

**跟另两个文字命令的关系**（同一句「中国」）：

| 命令 | 输出 | 键盘 |
|------|------|------|
| `jdan t9`（全拼九宫格） | `94664 486` | 九宫格，不定长 |
| `jdan spt9`（双拼九宫格） | `87 46` | 九宫格，每字 2 键 |
| `jdan sp`（双拼 26 键） | `vs go` | 全键盘，每字 2 键 |

### Flags

| flag | 默认 | 说明 |
|------|------|------|
| `-s, --scheme <名>` | 小鹤 | 方案（中文名 / id / 别名 sogou） |
| `--all` | false | 所有方案的结果一次对比 |
| `--codes` | false | 只输出整串码 |
| `--json` | false | 结构化（`--all` 时输出各方案码串的 map） |

无参数时从 stdin 读。

## 局限

- **零声母字**（`a/o/e` 开头、无辅音声母）按各方案的 RIME 规则处理；个别方案与你的输入法习惯可能略有出入。
- **多音字取最常见读音**（go-pinyin 默认），个别可能不准。
- 只做「每字确定性两码」，不是真实输入法（无候选/词频/整句）。
- 只收录 5 套主流方案（RIME 里的 `st` 四通等小众未收；日后可加）。

## 实现

```
internal/shuangpinx/shuangpinx.go   多方案编码：把 RIME 的 xform 规则按序套用    纯 stdlib、0 依赖
internal/cli/sp.go                  切词 + 边缘调 go-pinyin + 单/全方案渲染
```

- **小解释器**：每套方案是一串有序正则替换（照 RIME schema 抄），`Encode` 依次套用得两码。比手搓「声母/韵母表」更贴原始来源、更少誊写错。
- **复用**：`jdan spt9` 的小鹤编码已改为复用本包的 `flypy` 方案（小鹤表只存一处）；切词/go-pinyin 封装复用 `t9.go`。
- **测试**：每套方案钉几个锚点（`中/文/dan/双` 等照 RIME 手算），外加真实 go-pinyin 集成测（`你好 → ni hc`）。

跟 `jdan t9`、`jdan spt9`、`jdan morse`、`jdan alpha` 同属「文字 ↔ 编码」一类。
