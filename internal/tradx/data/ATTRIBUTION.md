# 数据来源与许可

本目录的 `*.txt` 词典来自 **OpenCC（Open Chinese Convert）**：

- 项目：https://github.com/BYVoid/OpenCC
- 取用 commit：`6c821528fc2be0a1fb78e7e0a4a1494a4e4c216e`（master，2026-07-18）
- 许可：**Apache License 2.0**（全仓含 `data/dictionary/`，见本目录 `LICENSE`）

## 收录的文件（取自 OpenCC `data/dictionary/`）

| 文件 | 用途 |
|---|---|
| STCharacters.txt / STPhrases.txt | 简→繁（单字 / 短语） |
| TSCharacters.txt / TSPhrases.txt | 繁→简（单字 / 短语） |
| TWPhrases.txt / TWVariants.txt / TWVariantsPhrases.txt | 台湾用词 / 字形变体 |
| HKVariants.txt / HKVariantsPhrases.txt | 香港字形变体 |

格式：`key\tvalue1 value2 …`（空格分隔多值，第一个为默认）。前几行为 `#` 注释头。

## 已知未收录（OpenCC 编译期生成，仓库无 .txt）

- `STPhrases_GeneratedFromRegionalPhrases`、`TSCharactersExt`

代价：约 1.7% 的台湾地区词（多为外国地名，如 索馬里/毛里塔尼亞）无法经 `s2t(t2s(K))==K`
回转，个别生僻字 t2s 可能不同。见 `tradx_test.go:TestTWPhrasesReachability`。

## 刷新步骤

```bash
git clone --depth 1 https://github.com/BYVoid/OpenCC /tmp/opencc
for f in STCharacters STPhrases TSCharacters TSPhrases TWPhrases TWVariants \
         TWVariantsPhrases HKVariants HKVariantsPhrases; do
  cp /tmp/opencc/data/dictionary/$f.txt internal/tradx/data/$f.txt
done
cp /tmp/opencc/LICENSE internal/tradx/data/LICENSE
# 更新本文件的 commit 号；跑 go test ./internal/tradx/ 确认解析与黄金用例仍过
```

数据为纯查表映射，`jdan` 只做前向最大匹配（自写，见 `convert.go`），不含 OpenCC 源码。
