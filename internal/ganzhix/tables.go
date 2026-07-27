// Package ganzhix 干支：六十甲子、四柱（八字）、五行阴阳与纳音。
//
// 与 lunarx 的分工：
//
//	lunarx   农历日期，年柱以正月初一（春节）为界 —— 生肖年
//	ganzhix  四柱，年柱以立春为界 —— 节气年
//
// 两者每年有最长约 30 天不一致，这是历法事实不是 bug：过年说「今年马年」
// 用的是生肖年，八字的年柱用的是节气年。两个口径都对，各自标注即可。
//
// 依赖方向（无环）：lunarx ──▶ ganzhix ──▶ jieqix
//
// 干支表放在本包而不是 lunarx，是因为 lunarx 有 1900–2100 的硬边界
// （内嵌农历表，出界报错），而干支和节气都不受此限。反方向依赖会让人
// 误以为 ganzhix 也被限住了。
package ganzhix

import "strings"

// TianGan 天干，索引 0..9。
var TianGan = [10]string{"甲", "乙", "丙", "丁", "戊", "己", "庚", "辛", "壬", "癸"}

// DiZhi 地支，索引 0..11。
var DiZhi = [12]string{"子", "丑", "寅", "卯", "辰", "巳", "午", "未", "申", "酉", "戌", "亥"}

// Zodiac 生肖，与地支一一对应。
var Zodiac = [12]string{"鼠", "牛", "虎", "兔", "龙", "蛇", "马", "羊", "猴", "鸡", "狗", "猪"}

// ganElement 天干五行：甲乙木 丙丁火 戊己土 庚辛金 壬癸水。
var ganElement = [10]string{"木", "木", "火", "火", "土", "土", "金", "金", "水", "水"}

// zhiElement 地支五行：寅卯木 巳午火 申酉金 亥子水 辰戌丑未土。
// 注意不是按索引两两分组——土占四个（辰戌丑未），分散在四季之交。
var zhiElement = [12]string{
	"水", // 子
	"土", // 丑
	"木", // 寅
	"木", // 卯
	"土", // 辰
	"火", // 巳
	"火", // 午
	"土", // 未
	"金", // 申
	"金", // 酉
	"土", // 戌
	"水", // 亥
}

// nayinPairs 六十甲子纳音，每两个干支共用一个，共 30 组。
// 下标 = 干支序号 / 2。
//
// 数据来源与核对（2026-07-27）：三个独立来源交叉比对，分歧处取多数：
//
//	#16 甲午乙未  沙中金 / 砂中金 / 沙中金  → 沙中金（「砂」为异体）
//	#21 甲辰乙巳  覆灯火 / 覆灯火 / 佛灯火  → 覆灯火（「佛灯火」亦见于文献）
//	#24 庚戌辛亥  钗钏金 / 钗环金 / 钗钏金  → 钗钏金（「钏」指手镯，与「钗」成对）
//
// 这三处是民俗文献里真实存在的异体写法，不是抄错。纳音名没有单一权威版本，
// 所以取通行写法并把异体记在这里，而不是假装只有一种。
var nayinPairs = [30]string{
	"海中金", "炉中火", "大林木", "路旁土", "剑锋金",
	"山头火", "涧下水", "城头土", "白蜡金", "杨柳木",
	"泉中水", "屋上土", "霹雳火", "松柏木", "长流水",
	"沙中金", "山下火", "平地木", "壁上土", "金箔金",
	"覆灯火", "天河水", "大驿土", "钗钏金", "桑柘木",
	"大溪水", "沙中土", "天上火", "石榴木", "大海水",
}

// Pillar 是一柱干支。
type Pillar struct {
	Index   int    // 0..59，六十甲子序号。0 = 甲子
	Gan     string // 甲
	Zhi     string // 子
	Element string // 木 —— 天干的五行
	ZhiElem string // 水 —— 地支的五行
	YinYang string // 阳
	Zodiac  string // 鼠 —— 地支对应的生肖
}

// String 返回「甲子」。
func (p Pillar) String() string { return p.Gan + p.Zhi }

// Nayin 返回该柱的纳音，如「海中金」。
func (p Pillar) Nayin() string { return nayinPairs[p.Index/2] }

// FromIndex 由六十甲子序号构造一柱。i 会先归一到 [0,60)，
// 所以传负数或超过 59 都不会 panic。
func FromIndex(i int) Pillar {
	i = ((i % 60) + 60) % 60
	gi, zi := i%10, i%12
	yy := "阳"
	if i%2 == 1 {
		yy = "阴"
	}
	return Pillar{
		Index:   i,
		Gan:     TianGan[gi],
		Zhi:     DiZhi[zi],
		Element: ganElement[gi],
		ZhiElem: zhiElement[zi],
		YinYang: yy,
		Zodiac:  Zodiac[zi],
	}
}

// IndexOf 反查干支的序号，如「甲子」→ 0。
// 第二个返回值为 false 表示不是合法干支——注意 60 个合法组合之外
// 还有 60 个「看起来像」的（如「甲丑」），阴阳不配，必须挡掉。
func IndexOf(gz string) (int, bool) {
	r := []rune(gz)
	if len(r) != 2 {
		return 0, false
	}
	gi := strings.Index(strings.Join(TianGan[:], ""), string(r[0]))
	zi := strings.Index(strings.Join(DiZhi[:], ""), string(r[1]))
	if gi < 0 || zi < 0 {
		return 0, false
	}
	gi /= 3 // 每个汉字 3 字节
	zi /= 3
	// 干支同步递进，天干走 10 步、地支走 12 步，只有同奇偶才配得上。
	// 60 个合法组合正是 lcm(10,12)，不是 10×12=120。
	if (gi-zi)%2 != 0 {
		return 0, false
	}
	for i := range 60 {
		if i%10 == gi && i%12 == zi {
			return i, true
		}
	}
	return 0, false
}

// Sexagenary 返回完整的六十甲子表，从甲子到癸亥。
func Sexagenary() []Pillar {
	out := make([]Pillar, 60)
	for i := range out {
		out[i] = FromIndex(i)
	}
	return out
}

// IndexFromYear 返回年份的干支序号：(year−4) mod 60。
//
// 传什么年就按什么口径：lunarx 传农历年（正月初一为界的生肖年），
// ganzhix 的四柱传节气年（立春为界）。同一个公式，输入不同，
// 结果在春节到立春之间会不一样——这正是两套口径的分歧所在。
func IndexFromYear(year int) int { return (((year - 4) % 60) + 60) % 60 }

// FromYear 返回年份对应的干支柱。
func FromYear(year int) Pillar { return FromIndex(IndexFromYear(year)) }

// ZodiacOfYear 返回年份对应的生肖。
func ZodiacOfYear(year int) string { return Zodiac[(((year-4)%12)+12)%12] }
