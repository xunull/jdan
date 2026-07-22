# jdan cangjie — 仓颉码查询（字→码 + 字根，离线，0 新依赖）

查汉字的**仓颉码**（朱邦復输入法，台/港主流），并把字母码翻成字根一并显示：`明 → AB（日月）`。
数据是 [Unicode Unihan](https://www.unicode.org/charts/unihan.html) 的 `kCangjie` 字段（仓颉三代），
生成 Go 表二分查找，**0 新依赖** —— 跟 `strokes`（笔画）、`sijiao`（四角）是三胞胎（同一份 Unihan、同一套 gen + 排序表 + 二分）。

## 原理

仓颉把一个字拆成 1-5 个**字根**，每根对应一个字母键（A-Y）。`明` = 日 + 月 = `AB`。

25 键字根表（固定，可硬编码）：
```
A日 B月 C金 D木 E水 F火 G土   H竹 I戈 J十 K大 L中 M一 N弓
O人 P心 Q手 R口   S尸 T廿 U山 V女 W田   Y卜   X難
```

**拆成哪几个字根靠字形、从码点算不出来**（同笔顺），只能查表——表就是 `kCangjie`，和 `strokes` 的 `kTotalStrokes`、`sijiao` 的 `kFourCornerCode` 一个模子。但**字母↔字根是固定映射**，所以能显示 `AB（日月）`：不只告诉你敲什么键，还告诉你**为什么**是 AB。这是 `sijiao` 给不了的（四角只有最终码、无法逐角分解）。

## 用法

```bash
$ jdan cangjie 明              # 明  AB（日月）
$ jdan cangjie 你              # 你  ONF（人弓火）
$ jdan cangjie 明变            # 明 AB（日月） / 变 YCE（卜金水）   （字与字用斜杠）
$ echo 明 | jdan cangjie        # 从 stdin 读
$ jdan cangjie --json 明你
```

非汉字（字母/数字/标点/emoji）跳过不计。表里查不到的汉字标「无 / ?」，末尾提示「另有 N 字无仓颉码」。

## 能力边界（诚实划界）

- **仓颉三代**：`kCangjie` 是仓颉三代取码，和你手上某个输入法（如五代）可能有出入。
- **覆盖约 2.9 万字**（29,189，介于 strokes 10 万与 sijiao 1.7 万之间）。表外字标「无」。
- **只做正查（字→码）**：反查（码→字，敲字母出字，仓颉的正向用法）本版不做，留作后续增量。
- 仓颉主要台/港用，niche。价值是"离线、补输入法/检字线、加上字根的教学感"，非日常刚需。

## 实现要点

- `//go:generate` 从 `kCangjie` 生成 `cangjie_dict.go`（`cangjieCP []rune` 升序 + `cangjieCode []string` 平行），二分查找。gen **按 `field==kCangjie` 过滤**（该文件与 kFourCornerCode 等共用）、跳 `#` 注释头。
- **全单值**（29,189 字均单码，比 `sijiao` 的 149 多值字更简单），码为字母串 A-Y（含 X=難，无 Z），长 1-5。
- 字根：`rootOf` 是 25 键静态表（A=日…X=難），`Roots(code)` 逐字节翻（码是 ASCII，字节安全；未知字母原样留防越界）。
- 查表顺序同 strokesx/sijiaox：先查表、查不到再 `unicode.Is(unicode.Han,r)` 分未知/跳过。
- 数据源 Unicode，公开可嵌；生成 Go 表进仓库，不嵌原始文件。刷新：重跑 `_tools/gen_cangjie.go`。
- 字根表正确性经变异测试守卫（删任一键 → 完整性测试 + 黄金用例变红）。
