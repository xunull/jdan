// Package tradx 做中文简↔繁转换（词汇级，离线，0 外部依赖）。
//
// 数据是 Unicode 之外的 OpenCC（Open Chinese Convert）词典（Apache-2.0，公开），
// 由 go:embed 嵌入 data/*.txt，运行时按需解析。算法是忠实复刻 OpenCC 的
// **前向最大匹配 + 词典组（union / short_circuit）**，见 convert.go。
//
// 能力边界（诚实划界）：
//   - 简繁转换 + 台/港地区用词到 OpenCC 词典为止，不做全量本地化/翻译；词典没有的词不自造。
//   - s2t/t2s（--to t/s）与 OpenCC 逐字节一致（无预分词，纯 greedy）。
//   - s2tw/s2twp/s2hk（--to tw/twp/hk）v0 不实现 MMSEG 预分词，仅在 greedy 跨词边界
//     过度匹配的罕见点上可能与 OpenCC 有别。用户 headline（发→發/髮、软件→軟體）不依赖 MMSEG。
//   - 缺 2 个 OpenCC 编译期生成词典（STPhrases_GeneratedFromRegionalPhrases、TSCharactersExt），
//     影响个别地区短语回转 / 生僻字 t2s。见 README。
package tradx

// Target 是转换目标代号（对应 OpenCC 的 config 名）。
const (
	ToT   = "t"   // 简→繁（OpenCC 标准，s2t）
	ToTW  = "tw"  // 简→繁台湾字形变体（s2tw）
	ToTWP = "twp" // 简→繁台湾（含地区用词，软件→軟體，s2twp）
	ToHK  = "hk"  // 简→繁香港字形变体（s2hk）
	ToS   = "s"   // 繁→简（t2s）
)

// Targets 是全部合法目标，按 CLI 展示顺序。
var Targets = []string{ToT, ToTW, ToTWP, ToHK, ToS}

// Convert 用 target 指定的 config 转换 text。便捷入口；批量/流式请用 NewConverter。
func Convert(text, target string) (string, error) {
	c, err := NewConverter(target)
	if err != nil {
		return "", err
	}
	return c.Convert(text), nil
}
