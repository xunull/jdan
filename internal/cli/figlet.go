package cli

import (
	"fmt"
	"io"
	"os"
	"sort"
	"strings"

	"github.com/spf13/cobra"

	"github.com/xunull/jdan/internal/figlet"
)

type figletCmdDeps struct {
	out io.Writer
	in  io.Reader
}

func newFigletCommand(deps figletCmdDeps) *cobra.Command {
	if deps.out == nil {
		deps.out = os.Stdout
	}
	if deps.in == nil {
		deps.in = os.Stdin
	}
	cmd := &cobra.Command{
		Use:   "figlet [text]",
		Short: "把文字渲染成 ASCII art 大横幅",
		Long: `把文字渲染成 ASCII art 大横幅（figlet 风格）。0 新依赖，内置字体。

字体：standard（# 描边）/ block（实心块 █）
覆盖 A-Z / 0-9 / 空格 / 常见标点；小写折叠成大写；不支持字符用空白占位。

例：
  jdan figlet "Hello"
  jdan figlet Deploy OK              # 多 arg 拼接
  jdan figlet "READY" --font block   # 实心块
  jdan figlet "Title" --center --width 60
  echo "Build Done" | jdan figlet    # stdin
  jdan figlet --list                 # 列出字体`,
		Args:          cobra.ArbitraryArgs,
		SilenceUsage:  true,
		SilenceErrors: true,
		RunE: func(cmd *cobra.Command, args []string) error {
			list, _ := cmd.Flags().GetBool("list")
			if list {
				return runFigletList(deps.out)
			}
			font, _ := cmd.Flags().GetString("font")
			width, _ := cmd.Flags().GetInt("width")
			center, _ := cmd.Flags().GetBool("center")
			return runFiglet(args, deps, font, width, center)
		},
	}
	cmd.Flags().String("font", figlet.DefaultFont, "字体：standard / block")
	cmd.Flags().Int("width", 80, "最大宽度，超过自动换行（0 = 不换行）")
	cmd.Flags().Bool("center", false, "在 --width 内居中")
	cmd.Flags().Bool("list", false, "列出内置字体")
	return cmd
}

func runFigletList(out io.Writer) error {
	names := figlet.FontNames()
	sort.Strings(names)
	for _, n := range names {
		fmt.Fprintln(out, n)
	}
	return nil
}

func runFiglet(args []string, deps figletCmdDeps, font string, width int, center bool) error {
	var text string
	if len(args) > 0 {
		text = strings.Join(args, " ")
	} else {
		data, err := io.ReadAll(deps.in)
		if err != nil {
			return err
		}
		text = strings.TrimRight(string(data), "\r\n")
	}
	if strings.TrimSpace(text) == "" {
		return fmt.Errorf("no text (pass as args or stdin)")
	}

	lines, err := figlet.Render(text, font, width, center)
	if err != nil {
		return err
	}
	for _, l := range lines {
		fmt.Fprintln(deps.out, l)
	}
	return nil
}

func init() {
	rootCmd.AddCommand(newFigletCommand(figletCmdDeps{}))
}
