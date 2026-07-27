package ganzhix

import (
	"fmt"
	"time"

	"github.com/xunull/jdan/internal/jieqix"
)

// jiaziAnchor 是日干支的相位锚点：这一天是甲子日。
//
// 权威来源：zh.wikipedia「干支」条目「1912年（中華民國元年）2月18日
// （農曆壬子年正月初一日）…都是『甲子日』」。同条目另给 1949-10-01 也是甲子日，
// 两日相隔 13740 天 = 229 × 60，自洽；且两日的农历日期都能被本仓库的 lunarx 复核。
// 完整验证链见 internal/ganzhix/testdata/day_pillar.tsv。
//
// 写锚点日期而不是写偏移常数，是刻意的：
//   - 偏移常数（本例为 11）是从这个日期推出来的中间结果，直接写会让
//     「这一天是甲子日」这个真正的事实消失在一个魔数里
//   - 相减抵消掉 JDN 的整体平移，万一 JDN 实现有系统性偏差，日柱不受影响
//
// 写这份实现时先手打错过一次那个偏移常数（写成 38），这条注释是有实证的。
var jiaziAnchorJDN = jieqix.JDN(1912, time.February, 18)

// Options 控制有争议的口径。
type Options struct {
	// LateZi 选「晚子时」派：23:00–23:59 日柱不翻，只有时柱用子。
	// 默认（false）是主流的「23:00 换日」派，与多数万年历一致。
	//
	// 两派都有传承，没有权威裁决。它影响四柱里的**两柱**（日柱与时柱），
	// 所以必须在算之前定，不是事后能加的开关。
	LateZi bool
}

// FourPillars 是一个时刻的四柱（八字）及其附带属性。
type FourPillars struct {
	Year, Month, Day, Hour Pillar

	// TermYear 是节气年——以立春为界，不是以正月初一为界。
	// 上层要做「生肖年 vs 节气年」的差异提示，拿它和 lunarx 的农历年比。
	TermYear int

	// Nayin 是年柱的纳音，如「海中金」。
	Nayin string

	// MonthTerm 是当前月令所依的「节」，如「小暑」。月柱由它定，不由公历月定。
	MonthTerm string

	// Local 是实际用于定日界与时辰的本地时刻。四柱看的是「本地几点」，
	// 所以这个字段决定了结果；输出时应当把它一并显示，避免用户以为
	// 自己传的时区没生效。
	Local time.Time

	// ZiDisputed 为 true 表示时刻落在 23:00–23:59 这个两派分歧的窗口里。
	// 上层据此在输出里标一行说明用的是哪派——不标的话，用户拿去对别的
	// 万年历会发现对不上却不知道为什么。
	ZiDisputed bool

	// LateZi 记录实际采用的是哪派，便于输出与 --json 消费方判断。
	LateZi bool
}

// Of 排出 t 时刻的四柱。
//
// t 自带的时区就是计算所用的本地时区——日柱看本地过没过日界、时柱看本地几点。
// 传 UTC 的时刻会排出「按 UTC 的四柱」，那多半不是调用方想要的：
// 中国民用时的口径（1929 前 UTC+8:05:43、之后 UTC+8）由上层 CLI 负责构造。
func Of(t time.Time, o Options) (FourPillars, error) {
	out := FourPillars{Local: t, LateZi: o.LateZi}

	// ---- 年柱：以立春为界 ----
	//
	// 注意不能用 t.Year() 直接算。立春在 2 月初，1 月 1 日到立春之间的日期
	// 属于上一个节气年。这正是与 lunarx 口径分歧的来源。
	lichun, err := lichunOf(t.Year())
	if err != nil {
		return out, err
	}
	termYear := t.Year()
	if t.Before(lichun) {
		termYear--
	}
	out.TermYear = termYear
	out.Year = FromYear(termYear)
	out.Nayin = out.Year.Nayin()

	// ---- 月柱：由「节」定，不由公历月定 ----
	major, err := jieqix.PrevMajor(t)
	if err != nil {
		return out, fmt.Errorf("定月令失败: %w", err)
	}
	out.MonthTerm = major.Name
	// jieqix 的 termDefs 按立春起排，12 个「节」在偶数下标上。
	// n = 0 是立春（寅月），n = 11 是小寒（丑月）。
	n := major.Index / 2
	monthZhi := (2 + n) % 12
	// 五虎遁：寅月干 = (年干×2 + 2) mod 10，之后逐月加一。
	// 自验：甲年(0)→丙(2)「甲己之年丙作首」；乙年(1)→戊(4)「乙庚之岁戊为头」。
	monthGan := (out.Year.Index%10*2 + 2 + n) % 10
	out.Month = fromGanZhi(monthGan, monthZhi)

	// ---- 日柱：儒略日连续推算 ----
	y, m, d := t.Date()
	if t.Hour() >= 23 && !o.LateZi {
		// 23:00 换日派：已进入次日子时，日柱翻到明天。
		next := time.Date(y, m, d, 12, 0, 0, 0, t.Location()).AddDate(0, 0, 1)
		y, m, d = next.Date()
	}
	out.Day = FromIndex(jieqix.JDN(y, m, d) - jiaziAnchorJDN)

	// ---- 时柱 ----
	//
	// 时辰边界：23:00–00:59 子，01:00–02:59 丑，…… 每两小时一辰。
	// 子时跨午夜，所以先 +1 再整除 2 才能把 23 点和 0 点归到同一辰。
	hourZhi := ((t.Hour() + 1) / 2) % 12
	// 五鼠遁：子时干 = (日干×2) mod 10，之后逐辰加一。
	// 自验：甲日(0)→甲(0)「甲己还加甲」；戊日(4)→壬(8)「戊癸何方发，壬子是真途」。
	//
	// 用的是**输出的**日干：默认派日柱已翻到次日，时干就按次日的日干推；
	// --late-zi 日柱没翻，时干就按当日的推。两派的差别正落在这里。
	hourGan := (out.Day.Index%10*2 + hourZhi) % 10
	out.Hour = fromGanZhi(hourGan, hourZhi)

	out.ZiDisputed = t.Hour() == 23

	return out, nil
}

// lichunOf 返回该公历年立春的精确时刻。
func lichunOf(year int) (time.Time, error) {
	ts, err := jieqix.Terms(year)
	if err != nil {
		return time.Time{}, fmt.Errorf("算 %d 年节气失败: %w", year, err)
	}
	for _, x := range ts {
		if x.Name == "立春" {
			return x.Time, nil
		}
	}
	return time.Time{}, fmt.Errorf("%d 年没算出立春", year)
}

// fromGanZhi 由天干、地支索引构造一柱。
//
// 干支同步递进，只有同奇偶配得上，所以 (gi, zi) 唯一确定 [0,60) 里的一个序号。
// 传入不配的组合（如甲丑）会 panic——那是调用方的逻辑错误，
// 不是可以静默兜住的数据问题：静默返回一个「最接近」的柱会让错误传下去。
func fromGanZhi(gi, zi int) Pillar {
	gi = ((gi % 10) + 10) % 10
	zi = ((zi % 12) + 12) % 12
	if (gi-zi)%2 != 0 {
		panic(fmt.Sprintf("ganzhix: 天干 %s 与地支 %s 阴阳不配，不构成合法干支",
			TianGan[gi], DiZhi[zi]))
	}
	for i := range 60 {
		if i%10 == gi && i%12 == zi {
			return FromIndex(i)
		}
	}
	panic("ganzhix: 不可达——六十甲子必然覆盖所有同奇偶组合")
}
