package cli

import (
	"encoding/json"
	"fmt"
	"io"
	"os"
	"strconv"
	"time"

	"github.com/spf13/cobra"

	"github.com/xunull/jdan/internal/ganzhix"
	"github.com/xunull/jdan/internal/lunarx"
)

type lunarCmdDeps struct {
	out io.Writer
	now time.Time // 注入便于测试；零值用 time.Now()
}

var cnWeekdays = []string{"周日", "周一", "周二", "周三", "周四", "周五", "周六"}

func newLunarCommand(deps lunarCmdDeps) *cobra.Command {
	if deps.out == nil {
		deps.out = os.Stdout
	}
	cmd := &cobra.Command{
		Use:   "lunar [date]",
		Short: "公历 ↔ 农历（干支/生肖/农历节日；1900–2100）",
		Long: `公历 ↔ 农历（中国阴历）转换，含干支纪年、生肖、农历节日。内嵌 1900–2100
农历表，0 新依赖。

例：
  jdan lunar                       今天的农历
  jdan lunar 2026-06-26            指定公历日 → 农历
  jdan lunar --to-solar 2026 1 1   农历 2026 正月初一 → 公历（今年春节几号）
  jdan lunar --to-solar 2025 6 1 --leap   农历闰六月初一 → 公历
  jdan lunar 2026 --festivals      列某年的农历节日
  jdan lunar 2026-06-26 --json

干支以正月初一为界（生肖年）。八字的年柱以**立春**为界，是另一套口径，
两者每年最长差约 30 天——差异期内本命令会提示，四柱请用 jdan ganzhi。
节气本身见 jdan jieqi。不做黄历宜忌。`,
		Args:          cobra.MaximumNArgs(3),
		SilenceUsage:  true,
		SilenceErrors: true,
		RunE: func(cmd *cobra.Command, args []string) error {
			now := deps.now
			if now.IsZero() {
				now = time.Now()
			}
			toSolar, _ := cmd.Flags().GetBool("to-solar")
			festivals, _ := cmd.Flags().GetBool("festivals")
			leap, _ := cmd.Flags().GetBool("leap")
			asJSON, _ := cmd.Flags().GetBool("json")

			switch {
			case toSolar:
				return runToSolar(deps.out, args, leap, asJSON)
			case festivals:
				return runFestivals(deps.out, args, now, asJSON)
			default:
				return runInspect(deps.out, args, now, asJSON)
			}
		},
	}
	cmd.Flags().Bool("to-solar", false, "农历 → 公历：参数为 <年> <月> <日>")
	cmd.Flags().Bool("leap", false, "配合 --to-solar：指定为闰月")
	cmd.Flags().Bool("festivals", false, "列某公历年的农历节日（参数为年，默认今年）")
	cmd.Flags().Bool("json", false, "结构化输出")
	return cmd
}

func runInspect(out io.Writer, args []string, now time.Time, asJSON bool) error {
	t := now
	if len(args) >= 1 {
		parsed, err := time.Parse("2006-01-02", args[0])
		if err != nil {
			return fmt.Errorf("日期要 YYYY-MM-DD：%q", args[0])
		}
		t = parsed
	}
	l, err := lunarx.SolarToLunar(t)
	if err != nil {
		return err
	}
	// 节气年（立春为界）—— 只为了做口径差异提示。
	// 尽力而为：算不出来就不提示，绝不让 lunar 因此失败。lunar 的本职是农历，
	// 节气只是拿来解释一个常被当成 bug 的不一致。
	termYearGZ, termOK := solarTermYearGanzhi(t)
	differs := termOK && termYearGZ != lunarx.GanzhiYear(l.Year)

	if asJSON {
		// ⚠️ 下面 9 个字段是已发布接口，名称/类型/含义**不得变更**。
		// 新增字段只能加在后面。internal/cli/lunar_test.go 的
		// TestLunarCmd_JSONFieldsAreStable 逐个守着它们。
		//
		// 尤其是 ganzhi：它一直是、也必须继续是**生肖年**（正月初一为界）。
		// 八字的年柱以立春为界，是另一个值——不要因为新加了 jdan ganzhi
		// 就把这里改成节气年，那会静默改掉下游脚本的语义。
		m := map[string]any{
			"solar":   t.Format("2006-01-02"),
			"weekday": cnWeekdays[int(t.Weekday())],
			"lunar":   l.String(),
			"year":    l.Year,
			"month":   l.Month,
			"day":     l.Day,
			"is_leap": l.IsLeap,
			"ganzhi":  lunarx.GanzhiYear(l.Year),
			"zodiac":  lunarx.Zodiac(l.Year),
			// 以下为新增，标明上面那个 ganzhi 用的是哪套口径
			"ganzhi_basis": "lunar-new-year",
		}
		if termOK {
			m["solar_term_ganzhi"] = termYearGZ
			m["term_year_differs"] = differs
		}
		return writeIndentJSON(out, m)
	}
	fmt.Fprintf(out, "公历: %s (%s)\n", t.Format("2006-01-02"), cnWeekdays[int(t.Weekday())])
	fmt.Fprintf(out, "农历: %s  (生肖 %s)\n", l.String(), lunarx.Zodiac(l.Year))
	fmt.Fprintf(out, "口径: 生肖年（正月初一为界）\n")
	if differs {
		fmt.Fprintf(out, "\n提示: 此日节气年（%s，立春为界）与生肖年（%s）不同。\n",
			termYearGZ, lunarx.GanzhiYear(l.Year))
		fmt.Fprintf(out, "      八字年柱用节气年，请用 jdan ganzhi。\n")
	}
	return nil
}

// solarTermYearGanzhi 返回该日所属节气年（立春为界）的干支。
//
// 与 lunarx.GanzhiYear 是两套并行的官方口径，都对：
//
//	生肖年  正月初一（春节）为界 —— 过年说「今年马年」用的是这个
//	节气年  立春为界             —— 八字的年柱用这个
//
// 春节在 1/21–2/20 之间浮动、立春固定在 2/3 前后，所以每年都有一段
// 最长约 30 天的时间两者不一致。这不是 bug，是历法事实。
func solarTermYearGanzhi(t time.Time) (string, bool) {
	// 用当天正午做代表：口径差异是按天讲的，不需要精确到时刻，
	// 而正午能避开日界与交节时刻恰好落在午夜附近的边角。
	noon := time.Date(t.Year(), t.Month(), t.Day(), 12, 0, 0, 0, chinaZone(t))
	fp, err := ganzhix.Of(noon, ganzhix.Options{})
	if err != nil {
		return "", false
	}
	return fp.Year.String(), true
}

func runToSolar(out io.Writer, args []string, leap, asJSON bool) error {
	if len(args) != 3 {
		return fmt.Errorf("--to-solar 需要 <年> <月> <日> 三个参数")
	}
	y, err1 := strconv.Atoi(args[0])
	m, err2 := strconv.Atoi(args[1])
	dd, err3 := strconv.Atoi(args[2])
	if err1 != nil || err2 != nil || err3 != nil {
		return fmt.Errorf("年/月/日要是数字")
	}
	t, err := lunarx.LunarToSolar(y, m, dd, leap)
	if err != nil {
		return err
	}
	if asJSON {
		return writeIndentJSON(out, map[string]any{
			"lunar":   lunarx.Lunar{Year: y, Month: m, Day: dd, IsLeap: leap}.String(),
			"solar":   t.Format("2006-01-02"),
			"weekday": cnWeekdays[int(t.Weekday())],
		})
	}
	fmt.Fprintf(out, "农历: %s\n", lunarx.Lunar{Year: y, Month: m, Day: dd, IsLeap: leap}.String())
	fmt.Fprintf(out, "公历: %s (%s)\n", t.Format("2006-01-02"), cnWeekdays[int(t.Weekday())])
	return nil
}

func runFestivals(out io.Writer, args []string, now time.Time, asJSON bool) error {
	year := now.Year()
	if len(args) >= 1 {
		y, err := strconv.Atoi(args[0])
		if err != nil {
			return fmt.Errorf("年份要是数字：%q", args[0])
		}
		year = y
	}
	fs, err := lunarx.Festivals(year)
	if err != nil {
		return err
	}
	if asJSON {
		arr := make([]map[string]string, 0, len(fs))
		for _, f := range fs {
			arr = append(arr, map[string]string{"name": f.Name, "date": f.Date.Format("2006-01-02")})
		}
		return writeIndentJSON(out, map[string]any{"year": year, "festivals": arr})
	}
	fmt.Fprintf(out, "%d 年农历节日：\n", year)
	for _, f := range fs {
		fmt.Fprintf(out, "  %s  %s (%s)\n", f.Date.Format("2006-01-02"), f.Name, cnWeekdays[int(f.Date.Weekday())])
	}
	return nil
}

func writeIndentJSON(out io.Writer, v any) error {
	enc := json.NewEncoder(out)
	enc.SetIndent("", "  ")
	return enc.Encode(v)
}

func init() {
	rootCmd.AddCommand(newLunarCommand(lunarCmdDeps{}))
}
