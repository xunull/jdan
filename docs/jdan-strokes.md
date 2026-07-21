# jdan strokes — 汉字笔画数查询

查汉字笔画数,整句逐字列出并给总数。

```
$ jdan strokes 龙凤呈祥
龙 5 / 凤 4 / 呈 7 / 祥 10
共 26 画
```

**这是输入法给不了的**:输入法只显示你正在打的那一个字的笔画,给不了整句逐字 + 总数。起名算总笔画、查一个 slogan、查生僻字都用得上。

## 数据来自 Unicode 官方

笔画数用 Unicode Unihan 的 `kTotalStrokes` 字段,公有领域,**离线查表,零依赖**。覆盖全部 CJK 汉字含扩展区,实测 Unicode 17.0.0 共 **102,998 字**,起名爱用的生僻字也查得到:

```
$ jdan strokes 鑫龗
鑫 24 / 龗 33
共 57 画
```

## 繁简是两个字

`龙` 和 `龍` 是不同码点,各有各的笔画:

```
$ jdan strokes 龙     # 简体
龙  5 画
$ jdan strokes 龍     # 繁体
龍  16 画
```

## 只做笔画数,不做笔顺

**不提供笔顺(横竖撇捺折序列)。** 原因:笔顺没有权威的开放机读数据 —— Unicode 没有笔顺字段;国家语委《通用规范汉字笔顺规范》没有开放机读版;开源的 `makemeahanzi` 之类都是从书法字体推导的近似值,且笔顺不完全等同国标。而查笔顺的人要的正是标准答案,给一个「看起来权威实则可能错」的笔顺比不给更糟。「火/必/凹凸」这些字的笔顺大陆/台湾/日本还各不同。

## 非汉字与未知字

```
$ jdan strokes "Hello 世界 2024！"
世 5 / 界 9
共 14 画
```

- **非汉字**(字母/数字/标点/emoji)跳过不计,不出现在统计里
- **未知汉字**(是汉字但不在 Unihan 笔画表)标为 `?`,总数不含它,并在总数行提示「未计入,总数可能偏小」—— 缺失如实说,不静默

## 用法

```bash
jdan strokes 龙                # 单字：龙  5 画
jdan strokes 龙凤呈祥          # 逐字 + 总数
echo 龙凤呈祥 | jdan strokes    # 从 stdin 读（管道）
jdan strokes 鑫 龗             # 多个参数拼接
jdan strokes --json 龙凤       # 结构化输出
```

| 选项 | 说明 |
|---|---|
| `--json` | 结构化输出 |

无参数时从 stdin 读。

## JSON 输出

```bash
$ jdan strokes --json 龙凤 | jq
{
  "chars": [
    { "char": "龙", "strokes": 5, "known": true },
    { "char": "凤", "strokes": 4, "known": true }
  ],
  "total": 9,
  "unknown": 0
}
```

- `known: false` 表示该汉字不在笔画表(生僻/异体),`strokes` 为 0、不计入 `total`
- `unknown` 是未知汉字的个数
- 非汉字不进 `chars`

## 数据更新

笔画表 `internal/strokesx/strokes_dict.go` 由 `_tools/gen_strokes.go` 从 Unihan 生成,已提交入库,构建不联网。Unicode 大版本更新时重生成:

```bash
curl -sL https://www.unicode.org/Public/UCD/latest/ucd/Unihan.zip -o /tmp/Unihan.zip
# 注意 kTotalStrokes 在 IRGSources.txt，不在 DictionaryLikeData.txt
unzip -p /tmp/Unihan.zip Unihan_IRGSources.txt | grep kTotalStrokes > /tmp/ks.txt
go run internal/strokesx/_tools/gen_strokes.go /tmp/ks.txt > internal/strokesx/strokes_dict.go
gofmt -w internal/strokesx/strokes_dict.go
```

## 相关命令

- `jdan pinyin` — 中文 → 拼音(同属中文工具线)
- `jdan lunar` — 公历 ↔ 农历
