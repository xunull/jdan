# jdan jyutping — 粤拼（粤语读音，离线，0 新依赖）

查汉字的**粤拼**（Jyutping 粤语读音）：`你 → nei5`。`pinyin` 给普通话读音，`jyutping` 给粤语，补齐两条读音线。
数据是 [Unicode Unihan](https://www.unicode.org/charts/unihan.html) 的 `kCantonese` 字段，
生成 Go 表二分查找，**0 新依赖** —— 跟 `strokes`/`sijiao`/`cangjie` 是四胞胎（同一份 Unihan、同一套 gen + 排序表 + 二分），而且**比 cangjie 还简单**（无字根表、全单值）。

## 用法

```bash
$ jdan jyutping 你              # 你  nei5
$ jdan jyutping 你好            # 你 nei5 / 好 hou2
$ jdan jyutping 我爱广东        # 我 ngo5 / 爱 oi3 / 广 gwong2 / 东 dung1   （字与字用斜杠）
$ echo 你好 | jdan jyutping      # 从 stdin 读
$ jdan jyutping --json 你好
```

非汉字（字母/数字/标点/emoji）跳过不计。表里查不到的汉字标「无 / ?」，末尾提示「另有 N 字无粤拼」。

## 能力边界（诚实划界）

- **单读音**：`kCantonese` 每字只存一个主读音，**列不了多音字的其它读法**。这点和 `pinyin` 不同 —— `pinyin`（wrap go-pinyin）本身也是逐字、无词级消歧，但它有 `--heteronym` 能列出多音字的所有读法；`kCantonese` 只有一个值，jyutping 列不了。例：`行→hang4`（缺"走路"haang4）、`長→coeng4`（缺"長官"zoeng2）、`重→cung5`（缺"重复"zung6）。两者都不做词级上下文消歧。
- **Jyutping 方案**（数字调，如 `nei5`），不是耶鲁式（`néih`）；声调 1-6。
- **覆盖约 2.99 万字**（29,936）。表外字标「无」。
- **只做正查（字→读音）**：反查（读音→字）本版不做。要多音+词级需换 rime-cantonese / CC-Canto 等更全的粤语词典（属独立增量）。

## 实现要点

- `//go:generate` 从 `kCantonese` 生成 `jyutping_dict.go`（`jyutCP []rune` 升序 + `jyutReading []string` 平行），二分查找。
- gen **先跳 `#` 注释头/空行，再按 `field==kCantonese` 过滤**：注意注释头就是 `#\tkCantonese`，字段名恰好也是 kCantonese，单靠字段过滤挡不住它（会多算 1 条变 29,937），必须先跳 `#`。
- 全单值（每字一个主读音），字符集 `a-z0-9`（测试守卫）。
- 逐字对照渲染直接沿用 `cangjie`/`sijiao` 的 `writeCangjieText` 形状（读音替代码）；不做 flat 空格拼接，避免与将来单字多值的空格撞车、以及"跳非汉字却留表外字"的不自洽。flat（`nei5 hou2` 读音相连）留 `--flat` 后续。
- 查表顺序同 strokesx/sijiaox/cangjiex：先查表、查不到再 `unicode.Is(unicode.Han,r)` 分未知/跳过。
- 数据源 Unicode，公开可嵌；生成 Go 表进仓库，不嵌原始文件。刷新：重跑 `_tools/gen_jyutping.go`。
