package cli

import (
	"fmt"
	"io"
	"os"

	"github.com/spf13/cobra"
	"golang.org/x/term"

	"github.com/xunull/jdan/internal/tree2"
)

type tree2Deps struct {
	out      io.Writer
	build    func(tree2.Options) ([]tree2.Node, error)
	render   func([]tree2.Node, tree2.Options) string
	getWidth func() (int, error)
}

func newTree2Command(deps tree2Deps) *cobra.Command {
	if deps.out == nil {
		deps.out = os.Stdout
	}
	if deps.build == nil {
		deps.build = tree2.Build
	}
	if deps.render == nil {
		deps.render = tree2.Render
	}
	if deps.getWidth == nil {
		deps.getWidth = stdoutWidth
	}

	var opts tree2.Options
	opts.Limit = tree2.DefaultLimit

	cmd := &cobra.Command{
		Use:   "tree2 [path]",
		Short: "按终端宽度多列显示两层目录树",
		Args:  cobra.MaximumNArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			if len(args) == 1 {
				opts.RootPath = args[0]
			} else {
				opts.RootPath = "."
			}
			if opts.Width < 0 {
				return fmt.Errorf("--width 必须大于等于 0")
			}
			if opts.Columns < 0 {
				return fmt.Errorf("--cols 必须大于等于 0")
			}
			if opts.Limit < 0 {
				return fmt.Errorf("--limit 必须大于等于 0")
			}
			if opts.Width == 0 {
				width, err := deps.getWidth()
				if err != nil || width <= 0 {
					width = tree2.DefaultWidth
				}
				opts.Width = width
			}

			nodes, err := deps.build(opts)
			if err != nil {
				return err
			}
			out := deps.render(nodes, opts)
			if out != "" {
				fmt.Fprintln(deps.out, out)
			}
			return nil
		},
	}

	cmd.Flags().IntVar(&opts.Columns, "cols", 0, "指定输出列数（默认自动推断）")
	cmd.Flags().IntVar(&opts.Width, "width", 0, "指定终端宽度（默认自动检测，失败时使用 80）")
	cmd.Flags().BoolVar(&opts.IncludeFiles, "files", false, "包含文件（默认只显示目录）")
	cmd.Flags().BoolVar(&opts.IncludeHidden, "all", false, "包含隐藏文件和目录")
	cmd.Flags().IntVar(&opts.Limit, "limit", tree2.DefaultLimit, "每个一级目录最多显示的子项数量（0 表示不限制）")
	return cmd
}

func stdoutWidth() (int, error) {
	width, _, err := term.GetSize(int(os.Stdout.Fd()))
	return width, err
}

func init() {
	rootCmd.AddCommand(newTree2Command(tree2Deps{}))
}
