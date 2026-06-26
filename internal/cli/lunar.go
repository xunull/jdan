package cli

import (
	"encoding/json"
	"fmt"
	"io"
	"os"
	"strconv"
	"time"

	"github.com/spf13/cobra"

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

干支以正月初一为界（生肖年），不做节气/黄历宜忌（节气属太阳历，另议）。`,
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
	if asJSON {
		return writeIndentJSON(out, map[string]any{
			"solar":   t.Format("2006-01-02"),
			"weekday": cnWeekdays[int(t.Weekday())],
			"lunar":   l.String(),
			"year":    l.Year,
			"month":   l.Month,
			"day":     l.Day,
			"is_leap": l.IsLeap,
			"ganzhi":  lunarx.GanzhiYear(l.Year),
			"zodiac":  lunarx.Zodiac(l.Year),
		})
	}
	fmt.Fprintf(out, "公历: %s (%s)\n", t.Format("2006-01-02"), cnWeekdays[int(t.Weekday())])
	fmt.Fprintf(out, "农历: %s  (生肖 %s)\n", l.String(), lunarx.Zodiac(l.Year))
	return nil
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
