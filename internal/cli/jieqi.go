package cli

import (
	"fmt"
	"io"
	"os"
	"strconv"
	"time"

	"github.com/spf13/cobra"

	"github.com/xunull/jdan/internal/jieqix"
)

type jieqiCmdDeps struct {
	out io.Writer
	now time.Time // 注入便于测试；零值用 time.Now()
}

func newJieqiCommand(deps jieqiCmdDeps) *cobra.Command {
	if deps.out == nil {
		deps.out = os.Stdout
	}
	cmd := &cobra.Command{
		Use:   "jieqi [year]",
		Short: "二十四节气的精确时刻（太阳视黄经，不限年份）",
		Long: `算二十四节气的精确时刻。节气是「太阳视黄经走到 15° 整倍数」的那一瞬间，
本命令把它算到分钟。纯算法、0 数据表、不限年份——不像 jdan lunar 被
1900–2100 的内嵌农历表卡死。

算法：截断 VSOP87 求地球日心黄经 → 转太阳地心视黄经（含章动、光行差）
→ 牛顿迭代解 15°k → 减 ΔT 得世界时。已用国立天文台（NAOJ）与
AstroPixels 两个独立权威源的 144 条锚点回归。

例：
  jdan jieqi                     今年 24 节气
  jdan jieqi 2026                指定年
  jdan jieqi --next              下一个节气及倒计时
  jdan jieqi --tz Asia/Tokyo     换个时区显示
  jdan jieqi 2026 --json

时区说明：本命令的 --tz 只影响显示。节气是一个物理瞬间，换时区只是换写法，
答案不变。（jdan ganzhi 的 --tz 不一样——那个会改变四柱。）

默认时区是中国民用时：1929-01-01 起 UTC+8，之前用北京地方平太阳时
UTC+8:05:43（中国 1929 年才正式启用东八区）。`,
		Args:          cobra.MaximumNArgs(1),
		SilenceUsage:  true,
		SilenceErrors: true,
		RunE: func(cmd *cobra.Command, args []string) error {
			now := deps.now
			if now.IsZero() {
				now = time.Now()
			}
			asJSON, _ := cmd.Flags().GetBool("json")
			next, _ := cmd.Flags().GetBool("next")
			tzName, _ := cmd.Flags().GetString("tz")

			if next {
				return runJieqiNext(deps.out, now, tzName, asJSON)
			}
			year := now.Year()
			if len(args) == 1 {
				y, err := strconv.Atoi(args[0])
				if err != nil {
					return fmt.Errorf("年份要是数字：%q", args[0])
				}
				year = y
			}
			return runJieqiYear(deps.out, year, tzName, asJSON)
		},
	}
	cmd.Flags().Bool("next", false, "只显示下一个节气及倒计时")
	cmd.Flags().String("tz", "", "显示时区（IANA 名，如 Asia/Tokyo）；默认中国民用时")
	cmd.Flags().Bool("json", false, "结构化输出")
	return cmd
}

// chinaZone 返回该日期适用的中国民用时时区。
//
// 中国 1929-01-01 才正式启用东八区；在那之前历书用的是北京地方平太阳时，
// 相当于 UTC+8:05:43。差 5 分 43 秒——对普通查询无所谓，但查民国日期
// 或者卡在交节前后六分钟内时，不处理就会和当时的历书对不上，
// 而且不会有任何提示。
func chinaZone(t time.Time) *time.Location {
	if t.Before(time.Date(1929, time.January, 1, 0, 0, 0, 0, time.FixedZone("", 8*3600))) {
		return time.FixedZone("LMT+8:05:43", 8*3600+5*60+43)
	}
	return time.FixedZone("CST", 8*3600)
}

// resolveZone 解析 --tz。空字符串表示用中国民用时（随日期自动切 1929 分支）。
func resolveZone(tzName string, ref time.Time) (*time.Location, error) {
	if tzName == "" {
		return chinaZone(ref), nil
	}
	loc, err := time.LoadLocation(tzName)
	if err != nil {
		return nil, fmt.Errorf("无法识别的时区 %q（要 IANA 名，如 Asia/Shanghai）", tzName)
	}
	return loc, nil
}

func runJieqiYear(out io.Writer, year int, tzName string, asJSON bool) error {
	ts, err := jieqix.Terms(year)
	if err != nil {
		return err
	}
	ref := time.Date(year, time.July, 1, 0, 0, 0, 0, time.UTC)
	loc, err := resolveZone(tzName, ref)
	if err != nil {
		return err
	}
	verified := jieqix.Verified(year)

	if asJSON {
		arr := make([]map[string]any, 0, len(ts))
		for _, x := range ts {
			arr = append(arr, map[string]any{
				"name":      x.Name,
				"longitude": x.Longitude,
				"major":     x.Major,
				"time":      x.Time.In(loc).Format(time.RFC3339),
			})
		}
		return writeIndentJSON(out, map[string]any{
			"year":     year,
			"tz":       loc.String(),
			"verified": verified,
			"terms":    arr,
		})
	}

	fmt.Fprintf(out, "%d 年二十四节气（%s）\n", year, loc.String())
	for _, x := range ts {
		kind := "气"
		if x.Major {
			kind = "节"
		}
		fmt.Fprintf(out, "  %s  %s  %s  %3d°\n",
			x.Time.In(loc).Format("01-02 15:04"), kind, x.Name, x.Longitude)
	}
	writeVerifyNote(out, year, verified)
	return nil
}

func runJieqiNext(out io.Writer, now time.Time, tzName string, asJSON bool) error {
	nx, err := jieqix.Next(now)
	if err != nil {
		return err
	}
	loc, err := resolveZone(tzName, now)
	if err != nil {
		return err
	}
	d := nx.Time.Sub(now)
	verified := jieqix.Verified(nx.Time.Year())

	if asJSON {
		return writeIndentJSON(out, map[string]any{
			"name":      nx.Name,
			"longitude": nx.Longitude,
			"major":     nx.Major,
			"time":      nx.Time.In(loc).Format(time.RFC3339),
			"tz":        loc.String(),
			"in_hours":  d.Hours(),
			"verified":  verified,
		})
	}
	kind := "气"
	if nx.Major {
		kind = "节"
	}
	fmt.Fprintf(out, "下一个节气：%s（%s，%d°）\n", nx.Name, kind, nx.Longitude)
	fmt.Fprintf(out, "  %s（%s）\n", nx.Time.In(loc).Format("2006-01-02 15:04"), loc.String())
	fmt.Fprintf(out, "  还有 %d 天 %d 小时\n", int(d.Hours())/24, int(d.Hours())%24)
	writeVerifyNote(out, nx.Time.Year(), verified)
	return nil
}

// writeVerifyNote 在超出已验证区间时打一行提示。
//
// 不提示的话，工具就在默默把不可信的数字当可信的发：算法本身不限年份，
// 但截断级数的误差随离 J2000 增大，ΔT 在 1620 前靠史料推算、2100 后靠外推。
// 输出长得和可信区间内一模一样，用户看不出来。
func writeVerifyNote(out io.Writer, year int, verified bool) {
	if verified {
		return
	}
	lo, hi := jieqix.VerifiedRange()
	fmt.Fprintf(out, "\n注意：%d 年超出已验证区间 %d–%d，时刻误差可能达分钟级。\n",
		year, lo, hi)
}

func init() {
	rootCmd.AddCommand(newJieqiCommand(jieqiCmdDeps{}))
}
