package cli

import (
	"encoding/json"
	"fmt"
	"io"
	"os"
	"strconv"
	"strings"
	"time"

	"github.com/spf13/cobra"
	"golang.org/x/term"

	"github.com/xunull/jdan/internal/calx"
)

type calCmdDeps struct {
	out io.Writer
	now time.Time // 注入便于测试「今天」高亮；零值用 time.Now()
}

func newCalCommand(deps calCmdDeps) *cobra.Command {
	if deps.out == nil {
		deps.out = os.Stdout
	}
	cmd := &cobra.Command{
		Use:   "cal [month] [year]",
		Short: "打印日历（月/年；高亮今天）",
		Long: `打印公历日历，高亮今天。默认本月、周一起始（ISO）。0 新依赖。

输入：
  jdan cal              本月
  jdan cal 12           今年 12 月
  jdan cal 12 2025      指定月/年
  jdan cal -y 2026      整年
  jdan cal -3           上/本/下月三联排

(Unix cal 把 "cal 6" 当成公元 6 年——反直觉；这里 cal 6 = 今年 6 月。)`,
		Args:          cobra.MaximumNArgs(2),
		SilenceUsage:  true,
		SilenceErrors: true,
		RunE: func(cmd *cobra.Command, args []string) error {
			now := deps.now
			if now.IsZero() {
				now = time.Now()
			}

			sunday, _ := cmd.Flags().GetBool("sunday")
			weekNum, _ := cmd.Flags().GetBool("week")
			noColor, _ := cmd.Flags().GetBool("no-color")
			yearMode, _ := cmd.Flags().GetBool("year")
			threeMode, _ := cmd.Flags().GetBool("three")
			asJSON, _ := cmd.Flags().GetBool("json")

			weekStart := time.Monday
			if sunday {
				weekStart = time.Sunday
			}
			opts := calx.Options{
				WeekStart: weekStart,
				WeekNum:   weekNum,
				Color:     !noColor && isTTY(deps.out),
			}

			todayFor := func(y int, m time.Month) int {
				if now.Year() == y && now.Month() == m {
					return now.Day()
				}
				return -1
			}

			// 整年
			if yearMode {
				year := now.Year()
				if len(args) >= 1 {
					y, err := strconv.Atoi(args[0])
					if err != nil {
						return fmt.Errorf("非法年份 %q", args[0])
					}
					year = y
				}
				if asJSON {
					return emitYearJSON(deps.out, year, opts.WeekStart)
				}
				var blocks [][]string
				for m := time.January; m <= time.December; m++ {
					blocks = append(blocks, calx.MonthLines(year, m, opts, todayFor(year, m)))
				}
				totalW := 3*calx.BlockWidth() + 2*3
				fmt.Fprintln(deps.out, calx.CenterWidth(fmt.Sprintf("%d 年", year), totalW))
				fmt.Fprintln(deps.out)
				fmt.Fprint(deps.out, calx.RenderBlocks(blocks, 3))
				return nil
			}

			// 三联排
			if threeMode {
				base := time.Date(now.Year(), now.Month(), 1, 0, 0, 0, 0, time.UTC)
				var blocks [][]string
				for off := -1; off <= 1; off++ {
					mt := base.AddDate(0, off, 0)
					blocks = append(blocks, calx.MonthLines(mt.Year(), mt.Month(), opts, todayFor(mt.Year(), mt.Month())))
				}
				fmt.Fprint(deps.out, calx.RenderBlocks(blocks, 3))
				return nil
			}

			// 单月
			year, month := now.Year(), now.Month()
			switch len(args) {
			case 1:
				m, err := parseMonth(args[0])
				if err != nil {
					return err
				}
				month = m
			case 2:
				m, err := parseMonth(args[0])
				if err != nil {
					return err
				}
				month = m
				y, err := strconv.Atoi(args[1])
				if err != nil {
					return fmt.Errorf("非法年份 %q", args[1])
				}
				year = y
			}

			if asJSON {
				return emitMonthJSON(deps.out, year, month, opts.WeekStart)
			}
			fmt.Fprint(deps.out, calx.Render(calx.MonthLines(year, month, opts, todayFor(year, month))))
			return nil
		},
	}
	cmd.Flags().BoolP("year", "y", false, "打印整年")
	cmd.Flags().BoolP("three", "3", false, "上/本/下月三联排")
	cmd.Flags().BoolP("sunday", "s", false, "周日起始（默认周一/ISO）")
	cmd.Flags().BoolP("week", "w", false, "左栏显示 ISO 周数")
	cmd.Flags().Bool("no-color", false, "关闭今天高亮")
	cmd.Flags().Bool("json", false, "结构化输出")
	return cmd
}

func parseMonth(s string) (time.Month, error) {
	if n, err := strconv.Atoi(s); err == nil {
		if n >= 1 && n <= 12 {
			return time.Month(n), nil
		}
		return 0, fmt.Errorf("月份要 1–12，得到 %d", n)
	}
	for m := time.January; m <= time.December; m++ {
		full := m.String()
		if strings.EqualFold(full, s) || strings.EqualFold(full[:3], s) {
			return m, nil
		}
	}
	return 0, fmt.Errorf("无法解析月份 %q", s)
}

func isTTY(w io.Writer) bool {
	f, ok := w.(*os.File)
	return ok && term.IsTerminal(int(f.Fd()))
}

func emitMonthJSON(out io.Writer, year int, month time.Month, weekStart time.Weekday) error {
	grid := calx.MonthGrid(year, month, weekStart)
	enc := json.NewEncoder(out)
	enc.SetIndent("", "  ")
	return enc.Encode(map[string]any{
		"year":       year,
		"month":      int(month),
		"week_start": weekStart.String(),
		"weeks":      grid,
	})
}

func emitYearJSON(out io.Writer, year int, weekStart time.Weekday) error {
	months := make([]map[string]any, 0, 12)
	for m := time.January; m <= time.December; m++ {
		months = append(months, map[string]any{
			"month": int(m),
			"weeks": calx.MonthGrid(year, m, weekStart),
		})
	}
	enc := json.NewEncoder(out)
	enc.SetIndent("", "  ")
	return enc.Encode(map[string]any{
		"year":       year,
		"week_start": weekStart.String(),
		"months":     months,
	})
}

func init() {
	rootCmd.AddCommand(newCalCommand(calCmdDeps{}))
}
