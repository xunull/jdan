package cli

import (
	"fmt"
	"io"
	"os"
	"strconv"
	"strings"

	"github.com/spf13/cobra"
)

const alphaN = 26

type alphaDeps struct {
	out io.Writer
}

func newAlphaCommand(deps alphaDeps) *cobra.Command {
	if deps.out == nil {
		deps.out = os.Stdout
	}
	cmd := &cobra.Command{
		Use:   "alpha [letter|number]",
		Short: "字母表 ↔ 序号对照（A1Z26）",
		Long: `无参数：打印 26 个字母 + 对齐在正下方的序号（1-26）。
带参数：单向查询 —— 字母 → 序号（k → 11），序号 → 字母（11 → k）。
0 依赖（纯 stdlib）。

例：
  jdan alpha          打印对照表（小写）
  jdan alpha -u       大写
  jdan alpha k        → 11
  jdan alpha 11       → k`,
		Args:          cobra.MaximumNArgs(1),
		SilenceUsage:  true,
		SilenceErrors: true,
		RunE: func(cmd *cobra.Command, args []string) error {
			upper, _ := cmd.Flags().GetBool("upper")

			if len(args) == 0 {
				fmt.Fprint(deps.out, renderAlpha(upper))
				return nil
			}

			arg := strings.TrimSpace(args[0])
			// 纯数字 → 字母
			if n, err := strconv.Atoi(arg); err == nil {
				letter, ok := numToLetter(n, upper)
				if !ok {
					return fmt.Errorf("序号 %d 超出范围（1-26）", n)
				}
				fmt.Fprintln(deps.out, letter)
				return nil
			}
			// 单个字母 → 序号
			if num, ok := letterToNum(arg); ok {
				fmt.Fprintln(deps.out, num)
				return nil
			}
			return fmt.Errorf("无法识别 %q（应为单个字母 a-z/A-Z 或序号 1-26）", args[0])
		},
	}
	cmd.Flags().BoolP("upper", "u", false, "用大写字母")
	return cmd
}

// renderAlpha 打印字母行 + 对齐在正下方的序号行。列宽按序号宽度（1 或 2）对齐，
// 使每个字母正好落在它的序号上方。
func renderAlpha(upper bool) string {
	base := byte('a')
	if upper {
		base = 'A'
	}
	letters := make([]string, alphaN)
	nums := make([]string, alphaN)
	for i := range alphaN {
		l := string(base + byte(i))
		n := strconv.Itoa(i + 1)
		w := len(n) // 字母恒 1 宽，按序号宽度补齐即可对齐
		letters[i] = fmt.Sprintf("%-*s", w, l)
		nums[i] = n
	}
	line1 := strings.TrimRight(strings.Join(letters, " "), " ")
	line2 := strings.Join(nums, " ")
	return line1 + "\n" + line2 + "\n"
}

// letterToNum 单个字母 → 序号（a/A → 1 … z/Z → 26）。
func letterToNum(s string) (int, bool) {
	if len(s) != 1 {
		return 0, false
	}
	c := s[0]
	switch {
	case c >= 'a' && c <= 'z':
		return int(c-'a') + 1, true
	case c >= 'A' && c <= 'Z':
		return int(c-'A') + 1, true
	}
	return 0, false
}

// numToLetter 序号 → 字母（1 → a/A … 26 → z/Z）。
func numToLetter(n int, upper bool) (string, bool) {
	if n < 1 || n > alphaN {
		return "", false
	}
	base := byte('a')
	if upper {
		base = 'A'
	}
	return string(base + byte(n-1)), true
}

func init() {
	rootCmd.AddCommand(newAlphaCommand(alphaDeps{}))
}
