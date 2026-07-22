# jdan trad — 简繁转换（词汇级，离线，0 新依赖）

中文简↔繁转换。不止换字形，还**按词消歧**（发→發/髮）、可选**地区用词**（软件→軟體）。
数据是 [OpenCC](https://github.com/BYVoid/OpenCC)（Apache-2.0）离线词典，`go:embed` 内嵌
9 个 `.txt`（~1.18MB），算法是自写的前向最大匹配，不引第三方库。

## 为什么逐字替换一定错

简繁不是一一映射：

- **一简对多繁**：`发→發(发展)/髮(头发)`、`干→幹(干部)/乾(干燥)/干(干戈)`、
  `台→臺(台湾)/颱(台风)/檯(写字台)`、里/裡、面/麵、松/鬆、后/後、钟→鐘/鍾、系→係/繫/系。
  单字表解不了，必须**看词**：`头发→頭髮`、`发展→發展`。
- **地区用词**：`软件→軟體`、`网络→網路`、`打印机→印表機`。这不是字形，是台/港用词替换。

## 原理

`config = conversion_chain`：若干 conversion 顺序执行，后者吃前者输出。每个 conversion 对文本做
**前向最大匹配**（位置 i 取最长命中，输出译文、前进命中长度；未命中原样前进一字）。词典组两种语义：

- **union**：跨成员取最长（平手取前者）。
- **short_circuit**：按序试成员，第一个有命中者即返回它自己的最长前缀（后面的不看）。

各方向的链（对照 OpenCC 真 config 与源码）：

```
t  (s2t)  : [ SC{STPhrases, STCharacters} ]
s  (t2s)  : [ SC{TSPhrases, TSCharacters} ]
tw (s2tw) : [ s2t链, SC{TWVariantsPhrases, TWVariants} ]
twp(s2twp): [ s2t链, SC{TWPhrases, TWVariantsPhrases, TWVariants} ]   ← 软件→軟體 在这层
hk (s2hk) : [ s2t链, SC{HKVariantsPhrases, HKVariants} ]
```

`软件→軟體` 是两阶段：`软件` 不是任何词典 key，s2t 先按字形得 `軟件`，再由 TWPhrases（键在繁体侧）
`軟件→軟體`。多值恒取第一个（如 `发→發`）。

## 用法

```bash
$ jdan trad 头发和发展            # 頭髮和發展（同字不同繁，按词分）
$ jdan trad --to twp 软件网络     # 軟體網路
$ jdan trad --to s 軟體           # 软体（繁→简）
$ echo 软件 | jdan trad --to twp  # 从 stdin 读（大输入逐行处理）
$ jdan trad --diff --to twp 软件和网络
「軟體」和「網路」
改动 2 处：
  软件 → 軟體
  网络 → 網路
$ jdan trad --json --to twp 软件
```

`--to`：`t`(默认) / `tw` / `twp` / `hk` / `s`。非汉字（英文/数字/标点/emoji）原样透传。

## 能力边界（诚实划界）

- 简繁 + 地区用词到 OpenCC 词典为止，**不做全量翻译**，词典没有的词不自造。
- **s2t/t2s 与 OpenCC 逐字节一致**（无预分词，纯 greedy = OpenCC `Conversion::AppendConverted`）。
- **tw/twp/hk 未实现 MMSEG 预分词**：OpenCC 对这几个会先切词再逐段转换，`jdan` 走 greedy 端到端，
  仅在"greedy 跨词边界过度匹配"的罕见点上可能与 OpenCC 有别。用户两个 headline
  （发→發/髮、软件→軟體）不依赖 MMSEG。
- **约 1.7% 台湾地区词回转不到**：OpenCC 另有两个编译期生成词典
  （`STPhrases_GeneratedFromRegionalPhrases`、`TSCharactersExt`）仓库无 `.txt`，
  未收录。受影响的多为外国地名（索馬里/毛里塔尼亞…）。量化见
  `internal/tradx/tradx_test.go:TestTWPhrasesReachability`。

## 实现要点

- `go:embed data/*.txt` + `sync.Once` 懒加载：~1.18MB 词典只在首次 `jdan trad` 时解析一次，
  不拖累其余子命令启动；解析失败显式报错不吞成空表。
- 解析按内容跳空行/`#` 行（不硬编码跳固定行数），防上游改注释头时吞真数据。
- `--diff`/`--json` 的改动段是对"原文↔译文"做事后 rune 级 LCS diff（与分趟无关），`pos` 用输入侧全局偏移。
- 数据许可 Apache-2.0，署名见 `internal/tradx/data/ATTRIBUTION.md` 与 `LICENSE`。
