// Package barcode 生成 Code128 一维条码（纯函数，0 依赖）。
//
// 原理：一维条码 = 一排竖条+空白，靠宽度编码。Code128 共 107 个符号，每个符号宽 11 模块
// （Stop 13），由 3 条+3 空（6 个元素，各宽 1-4）组成。结构：
//
//	[静区] Start [数据...] 校验 Stop [静区]
//
// 校验 = (Start值 + Σ(位置×符号值)) mod 103。这里内嵌 107 行模式表自己编码，不引外部库。
package barcode

import (
	"errors"
	"fmt"
)

// code128Patterns[v] 是符号值 v 的宽度模式（交替 条/空，首元素为条）。
// 0-102 数据/校验，103-105 起始符（A/B/C），106 为 Stop（含终止条，7 元素 13 模块）。
var code128Patterns = [107]string{
	"212222", "222122", "222221", "121223", "121322", "131222", "122213", "122312",
	"132212", "221213", "221312", "231212", "112232", "122132", "122231", "113222",
	"123122", "123221", "223211", "221132", "221231", "213212", "223112", "312131",
	"311222", "321122", "321221", "312212", "322112", "322211", "212123", "212321",
	"232121", "111323", "131123", "131321", "112313", "132113", "132311", "211313",
	"231113", "231311", "112133", "112331", "132131", "113123", "113321", "133121",
	"313121", "211331", "231131", "213113", "213311", "213131", "311123", "311321",
	"331121", "312113", "312311", "332111", "314111", "221411", "431111", "111224",
	"111422", "121124", "121421", "141122", "141221", "112214", "112412", "122114",
	"122411", "142112", "142211", "241211", "221114", "413111", "241112", "134111",
	"111242", "121142", "121241", "114212", "124112", "124211", "411212", "421112",
	"421211", "212141", "214121", "412121", "111143", "111341", "131141", "114113",
	"114311", "411113", "411311", "113141", "114131", "311141", "411131",
	"211412",  // 103 Start A
	"211214",  // 104 Start B
	"211232",  // 105 Start C
	"2331112", // 106 Stop（含终止条）
}

const quietModules = 10 // 两侧静区，不留扫不出来

// Symbol 是一次编码结果。
type Symbol struct {
	Data     string `json:"data"`
	CodeSet  string `json:"code_set"` // B 或 C
	Checksum int    `json:"checksum"`
	Modules  []bool `json:"-"` // true=黑条，已含两侧静区
}

// Width 返回总模块数（含静区）。
func (s Symbol) Width() int { return len(s.Modules) }

// Encode 把字符串编成 Code128 模块位图。
// 字符集：默认 B（可打印 ASCII 32-126）；输入全为数字且偶数长度时用 C（密度翻倍）。
func Encode(data string) (Symbol, error) {
	if data == "" {
		return Symbol{}, errors.New("空输入")
	}

	useC := allDigits(data) && len(data)%2 == 0
	var (
		values   []int
		startVal int
		codeSet  string
	)
	if useC {
		startVal, codeSet = 105, "C"
		for i := 0; i < len(data); i += 2 {
			values = append(values, int(data[i]-'0')*10+int(data[i+1]-'0'))
		}
	} else {
		startVal, codeSet = 104, "B"
		for i := 0; i < len(data); i++ {
			c := data[i]
			if c < 32 || c > 126 {
				return Symbol{}, fmt.Errorf("字符 %q 不在 Code128-B 可编码范围（可打印 ASCII 32-126）", string(c))
			}
			values = append(values, int(c)-32)
		}
	}

	// 校验：(Start + Σ 位置×值) mod 103，位置从 1 起
	sum := startVal
	for i, v := range values {
		sum += (i + 1) * v
	}
	check := sum % 103

	// 完整符号序列：Start, 数据..., 校验, Stop
	seq := make([]int, 0, len(values)+3)
	seq = append(seq, startVal)
	seq = append(seq, values...)
	seq = append(seq, check, 106)

	// 展开成模块（首元素为条，交替）
	bars := make([]bool, 0, len(seq)*11+13)
	for _, v := range seq {
		bar := true
		for _, w := range code128Patterns[v] {
			for range int(w - '0') {
				bars = append(bars, bar)
			}
			bar = !bar
		}
	}

	// 两侧静区
	modules := make([]bool, 0, len(bars)+2*quietModules)
	modules = append(modules, make([]bool, quietModules)...)
	modules = append(modules, bars...)
	modules = append(modules, make([]bool, quietModules)...)

	return Symbol{Data: data, CodeSet: codeSet, Checksum: check, Modules: modules}, nil
}

func allDigits(s string) bool {
	for i := 0; i < len(s); i++ {
		if s[i] < '0' || s[i] > '9' {
			return false
		}
	}
	return len(s) > 0
}
