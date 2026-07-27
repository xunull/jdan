package cli

import (
	"fmt"
	"io"
	"os"
	"time"

	"github.com/spf13/cobra"

	"github.com/xunull/jdan/internal/ganzhix"
	"github.com/xunull/jdan/internal/lunarx"
)

type ganzhiCmdDeps struct {
	out io.Writer
	now time.Time // 注入便于测试；零值用 time.Now()
}

func newGanzhiCommand(deps ganzhiCmdDeps) *cobra.Command {
	if deps.out == nil {
		deps.out = os.Stdout
	}
	cmd := &cobra.Command{
		Use:   "ganzhi [date] [time]",
		Short: "干支四柱（八字）+ 五行纳音；六十甲子查询",
		Long: `排四柱：给一个时刻，算出年、月、日、时四个干支，附五行阴阳与年柱纳音。

口径（重要）：
  年柱以**立春**为界，不是以正月初一为界。所以 jdan ganzhi 和 jdan lunar
  在春节到立春之间会给出不同的干支——两个都对，是两套并行的官方口径：
  过年说「今年马年」用的是生肖年（正月初一界），八字的年柱用节气年（立春界）。
  两者每年最长差约 30 天。差异期内本命令会主动提示。

  月柱由**节气**定，不由公历月定（立春起寅月，惊蛰起卯月……）。
  只有 12 个「节」分月，12 个「气」（中气）不分。

例：
  jdan ganzhi                        现在的四柱
  jdan ganzhi 1990-05-20 14:30       指定时刻
  jdan ganzhi 1990-05-20             只给日期：出三柱，时柱留空（见下）
  jdan ganzhi --of 甲子               反查序号、五行、纳音
  jdan ganzhi --table                六十甲子全表
  jdan ganzhi 2026-07-27 23:30 --late-zi
  jdan ganzhi 1990-05-20 14:30 --json

不给时刻就不出时柱。默认成 00:00 会凭空造出一个子时，而你并没有提供那个信息。

子时分歧：23:00–23:59 算次日还是当日，两派都有传承，没有权威裁决。
默认取主流派（23:00 换日，日柱翻），--late-zi 切晚子时派（日柱不翻）。
它影响四柱里的**两柱**，落在这一小时内时输出会标明用的是哪派。

时区：--tz 声明的是**挂钟读数属于哪个时区**，它会改变答案（jdan jieqi 的
--tz 只改显示，两者不同）。具体改哪几柱，取决于你固定的是哪一头：

  给了日期时刻   挂钟不变、绝对瞬间平移 → 改**年柱月柱**（它们跟节气时刻比）
                 日柱时柱看本地几点，不受影响
  没给（用现在） 瞬间不变、挂钟平移     → 改**日柱时柱**

默认中国民用时：1929-01-01 起 UTC+8，之前用北京地方平太阳时 UTC+8:05:43。
显式传 --tz 时完全接管，1929 自动分支让位。

不做黄历宜忌、神煞、大运流年、命理解读——这是历法工具，不是算命工具。
也未做真太阳时校正（严格的八字要按出生地经度修正，需要额外输入经度）。`,
		Args:          cobra.MaximumNArgs(2),
		SilenceUsage:  true,
		SilenceErrors: true,
		RunE: func(cmd *cobra.Command, args []string) error {
			now := deps.now
			if now.IsZero() {
				now = time.Now()
			}
			asJSON, _ := cmd.Flags().GetBool("json")
			table, _ := cmd.Flags().GetBool("table")
			of, _ := cmd.Flags().GetString("of")
			lateZi, _ := cmd.Flags().GetBool("late-zi")
			tzName, _ := cmd.Flags().GetString("tz")

			// 模式分派用 flag，不做内容嗅探——三种模式的输出形状完全不同，
			// --json 消费方不该先判类型才知道拿到的是什么。
			// 与 internal/cli/lunar.go 的 --to-solar/--festivals 同一套路。
			switch {
			case table:
				if len(args) > 0 {
					return fmt.Errorf("--table 不接位置参数")
				}
				return runGanzhiTable(deps.out, asJSON)
			case of != "":
				if len(args) > 0 {
					return fmt.Errorf("--of 不接位置参数")
				}
				return runGanzhiOf(deps.out, of, asJSON)
			default:
				return runGanzhiPillars(deps.out, args, now, tzName, lateZi, asJSON)
			}
		},
	}
	cmd.Flags().Bool("table", false, "列六十甲子全表")
	cmd.Flags().String("of", "", "反查某个干支的序号、五行、纳音（如 --of 甲子）")
	cmd.Flags().Bool("late-zi", false, "23:00–23:59 用晚子时派（日柱不翻）")
	cmd.Flags().String("tz", "", "挂钟读数所属时区（IANA 名）；会改变答案，详见 --help")
	cmd.Flags().Bool("json", false, "结构化输出")
	cmd.MarkFlagsMutuallyExclusive("table", "of")
	return cmd
}

func runGanzhiPillars(out io.Writer, args []string, now time.Time, tzName string, lateZi, asJSON bool) error {
	at, hasTime, err := parseGanzhiWhen(args, now, tzName)
	if err != nil {
		return err
	}

	fp, err := ganzhix.Of(at, ganzhix.Options{LateZi: lateZi})
	if err != nil {
		return err
	}

	// 生肖年（正月初一界）vs 节气年（立春界）的差异提示。
	// ganzhix 不能 import lunarx（会成环），所以这个比较放在 CLI 层——
	// 这里也正是它该在的地方：口径差异是要讲给人听的，不是算法的一部分。
	lunarZodiacYear, lunarOK := lunarZodiacYearOf(at)
	differs := lunarOK && lunarZodiacYear != fp.Year.String()

	if asJSON {
		m := map[string]any{
			"input":        at.Format(time.RFC3339),
			"tz":           at.Location().String(),
			"term_year":    fp.TermYear,
			"year":         pillarJSON(fp.Year),
			"month":        pillarJSON(fp.Month),
			"day":          pillarJSON(fp.Day),
			"month_term":   fp.MonthTerm,
			"nayin":        fp.Nayin,
			"zodiac":       fp.Year.Zodiac,
			"late_zi":      fp.LateZi,
			"zi_disputed":  fp.ZiDisputed,
			"ganzhi_basis": "solar-term-year",
		}
		if hasTime {
			m["hour"] = pillarJSON(fp.Hour)
		} else {
			m["hour"] = nil
		}
		if lunarOK {
			m["lunar_zodiac_year"] = lunarZodiacYear
			m["term_year_differs"] = differs
		}
		return writeIndentJSON(out, m)
	}

	if hasTime {
		fmt.Fprintf(out, "输入: %s（%s）\n", at.Format("2006-01-02 15:04"), at.Location().String())
	} else {
		// 不打那个内部占位的 00:00——用户没提供时刻，显示一个具体钟点会让人
		// 以为是自己输的，进而怀疑时柱为什么空着。
		fmt.Fprintf(out, "输入: %s（%s，未提供时刻）\n", at.Format("2006-01-02"), at.Location().String())
	}
	fmt.Fprintln(out)
	fmt.Fprintf(out, "  年柱  %s   %s%s  %s  %s年\n",
		fp.Year, fp.Year.Element, fp.Year.ZhiElem, fp.Year.YinYang, fp.Year.Zodiac)
	fmt.Fprintf(out, "  月柱  %s   %s%s  %s  （%s 起）\n",
		fp.Month, fp.Month.Element, fp.Month.ZhiElem, fp.Month.YinYang, fp.MonthTerm)
	fmt.Fprintf(out, "  日柱  %s   %s%s  %s\n",
		fp.Day, fp.Day.Element, fp.Day.ZhiElem, fp.Day.YinYang)
	if hasTime {
		fmt.Fprintf(out, "  时柱  %s   %s%s  %s\n",
			fp.Hour, fp.Hour.Element, fp.Hour.ZhiElem, fp.Hour.YinYang)
	} else {
		fmt.Fprintf(out, "  时柱  ——   （未提供时刻）\n")
	}
	fmt.Fprintf(out, "\n  纳音  %s        口径  节气年（立春为界）\n", fp.Nayin)

	if fp.ZiDisputed {
		school := "23:00 换日（主流）"
		other := "--late-zi 可切晚子时派"
		if fp.LateZi {
			school = "晚子时（日柱不翻）"
			other = "去掉 --late-zi 可切主流派"
		}
		fmt.Fprintf(out, "\n提示: 23:00–23:59 两派分歧，此处用的是 %s；%s。\n", school, other)
	}
	if differs {
		fmt.Fprintf(out, "\n提示: 此日生肖年（%s）与节气年（%s）不同——前者以正月初一为界，\n",
			lunarZodiacYear, fp.Year)
		fmt.Fprintf(out, "      后者以立春为界。八字年柱用节气年，jdan lunar 显示的是生肖年。\n")
	}
	return nil
}

// parseGanzhiWhen 解析位置参数为一个带时区的时刻。
// 第二个返回值表示用户是否给了时刻——没给就不出时柱。
func parseGanzhiWhen(args []string, now time.Time, tzName string) (time.Time, bool, error) {
	if len(args) == 0 {
		loc, err := resolveZone(tzName, now)
		if err != nil {
			return time.Time{}, false, err
		}
		return now.In(loc), true, nil
	}

	d, err := time.Parse("2006-01-02", args[0])
	if err != nil {
		return time.Time{}, false, fmt.Errorf("日期要 YYYY-MM-DD：%q", args[0])
	}
	hour, minute, hasTime := 0, 0, false
	if len(args) == 2 {
		hm, err := time.Parse("15:04", args[1])
		if err != nil {
			return time.Time{}, false, fmt.Errorf("时刻要 HH:MM：%q", args[1])
		}
		hour, minute, hasTime = hm.Hour(), hm.Minute(), true
	}

	// 先用一个近似时刻定时区（1929 分支只看日期，不受这点误差影响），
	// 再用该时区重新构造。
	loc, err := resolveZone(tzName, d)
	if err != nil {
		return time.Time{}, false, err
	}
	return time.Date(d.Year(), d.Month(), d.Day(), hour, minute, 0, 0, loc), hasTime, nil
}

// lunarZodiacYearOf 返回该日的生肖年干支（正月初一为界）。
// 超出 lunarx 的 1900–2100 表范围时返回 false——那种情况下没法做差异比较，
// 但四柱本身照算不误。
func lunarZodiacYearOf(t time.Time) (string, bool) {
	l, err := lunarx.SolarToLunar(t)
	if err != nil {
		return "", false
	}
	return lunarx.GanzhiYear(l.Year), true
}

func runGanzhiOf(out io.Writer, gz string, asJSON bool) error {
	i, ok := ganzhix.IndexOf(gz)
	if !ok {
		return fmt.Errorf("%q 不是合法干支。天干地支同步递进，只有同奇偶配得上，"+
			"合法组合共 60 个（不是 10×12=120），如「甲丑」就不存在", gz)
	}
	p := ganzhix.FromIndex(i)
	if asJSON {
		m := pillarJSON(p)
		m["ordinal"] = i + 1
		return writeIndentJSON(out, m)
	}
	fmt.Fprintf(out, "%s  六十甲子第 %d 个\n", p, i+1)
	fmt.Fprintf(out, "  天干 %s %s%s   地支 %s %s   生肖 %s\n",
		p.Gan, p.YinYang, p.Element, p.Zhi, p.ZhiElem, p.Zodiac)
	fmt.Fprintf(out, "  纳音 %s\n", p.Nayin())
	return nil
}

func runGanzhiTable(out io.Writer, asJSON bool) error {
	all := ganzhix.Sexagenary()
	if asJSON {
		arr := make([]map[string]any, 0, len(all))
		for i, p := range all {
			m := pillarJSON(p)
			m["ordinal"] = i + 1
			arr = append(arr, m)
		}
		return writeIndentJSON(out, map[string]any{"count": len(arr), "sexagenary": arr})
	}
	fmt.Fprintln(out, "六十甲子（每两个共用一个纳音）")
	for i, p := range all {
		fmt.Fprintf(out, "  %2d  %s  %s%s  %s  %s\n",
			i+1, p, p.YinYang, p.Element, p.Zodiac, p.Nayin())
		if i%10 == 9 && i != len(all)-1 {
			fmt.Fprintln(out)
		}
	}
	return nil
}

func pillarJSON(p ganzhix.Pillar) map[string]any {
	return map[string]any{
		"ganzhi":      p.String(),
		"gan":         p.Gan,
		"zhi":         p.Zhi,
		"index":       p.Index,
		"gan_element": p.Element,
		"zhi_element": p.ZhiElem,
		"yin_yang":    p.YinYang,
		"zodiac":      p.Zodiac,
		"nayin":       p.Nayin(),
	}
}

func init() {
	rootCmd.AddCommand(newGanzhiCommand(ganzhiCmdDeps{}))
}
