package cli

import (
	"fmt"
	"io"
	"os"
	"runtime"
	"sync/atomic"
	"time"

	"github.com/spf13/cobra"

	"github.com/xunull/jdan/internal/sizex"
	"github.com/xunull/jdan/internal/termx"
)

type sizeCmdDeps struct {
	out    io.Writer
	errOut io.Writer // 进度写这里；非 TTY 时静默
	scan   func(sizex.Options) (*sizex.Result, error)
}

// 进度刷新间隔。扫大目录要几秒到几十秒，静默会让人以为卡死。
const sizeProgressInterval = 200 * time.Millisecond

func newSizeCommand(deps sizeCmdDeps) *cobra.Command {
	if deps.out == nil {
		deps.out = os.Stdout
	}
	if deps.errOut == nil {
		deps.errOut = os.Stderr
	}
	if deps.scan == nil {
		deps.scan = sizex.Scan
	}

	cmd := &cobra.Command{
		Use:   "size [path]",
		Short: "目录体积排行（du 的排行榜版，带占比条形图）",
		Long: `扫描目录树，按占盘大小排行，带占比条形图。省掉 du -sh * | sort -hr | head 这串管道。

默认量的是**实际占盘**（st_blocks × 512）而不是逻辑大小，因为你问的是
「删掉能腾出多少空间」。两者差得比直觉大：500 个 1 字节文件逻辑 500 B、
实际占 2 MB（4 KiB 块取整）；稀疏文件则相反，逻辑 1 GiB、实际 0 B。
用 --apparent 切换到逻辑大小（Finder 显示的那个）。

语义对齐 du：硬链接只计一次、默认不跨文件系统、不跟随符号链接、目录自身
的块计入总量。根总量与 du -sh 一致。

例：
  jdan size                     当前目录，排行榜前 10
  jdan size ~/Library           指定目录
  jdan size --depth 3           展开三层
  jdan size --top 20            每层显示 20 项
  jdan size --apparent          按逻辑大小（对齐 Finder）
  jdan size --files             把文件也列出来，不只是目录
  jdan size --one-file-system=false  跨越挂载点
  jdan size --json | jq         全树 JSON（不受 --top/--depth 影响）`,
		Args:          cobra.MaximumNArgs(1),
		SilenceUsage:  true,
		SilenceErrors: true,
		RunE: func(cmd *cobra.Command, args []string) error {
			root := "."
			if len(args) == 1 {
				root = args[0]
			}
			depth, _ := cmd.Flags().GetInt("depth")
			top, _ := cmd.Flags().GetInt("top")
			apparent, _ := cmd.Flags().GetBool("apparent")
			oneFS, _ := cmd.Flags().GetBool("one-file-system")
			all, _ := cmd.Flags().GetBool("all")
			files, _ := cmd.Flags().GetBool("files")
			jobs, _ := cmd.Flags().GetInt("jobs")
			noColor, _ := cmd.Flags().GetBool("no-color")
			verbose, _ := cmd.Flags().GetBool("verbose")
			asJSON, _ := cmd.Flags().GetBool("json")

			if jobs <= 0 {
				jobs = defaultSizeJobs()
			}

			var scanned atomic.Uint64
			// 进度只在 stderr 是 TTY 且不输出 JSON 时显示：管道场景静默，
			// 免得污染下游。
			stopProgress := func() {}
			if !asJSON && isTTY(deps.errOut) {
				stopProgress = startSizeProgress(deps.errOut, &scanned)
			}

			start := time.Now()
			res, err := deps.scan(sizex.Options{
				Root:          root,
				Apparent:      apparent,
				OneFileSystem: oneFS,
				IncludeHidden: all,
				IncludeFiles:  files,
				Jobs:          jobs,
				Scanned:       &scanned,
			})
			stopProgress()
			if err != nil {
				return err
			}
			elapsed := time.Since(start)

			if asJSON {
				return writeIndentJSON(deps.out, res.JSONData())
			}

			tree := sizex.BuildTree(res, sizex.TreeOptions{Depth: depth, Top: top})
			maxWidth := 0
			if isTTY(deps.out) {
				maxWidth = termWidth(deps.out)
			}
			fmt.Fprint(deps.out, sizex.Render(res, tree, sizex.RenderOptions{
				Color:    !noColor && !noColorEnv() && isTTY(deps.out),
				MaxWidth: maxWidth,
				Verbose:  verbose,
				Elapsed:  elapsed,
			}))
			return nil
		},
	}

	cmd.Flags().Int("depth", 1, "展开层数（1 = 根 + 直接子项）")
	cmd.Flags().Int("top", 10, "每层最多显示几项，其余合并为「其他 N 项」；0 = 不限")
	cmd.Flags().Bool("apparent", false, "用逻辑大小（Size()）而非实际占盘")
	cmd.Flags().BoolP("one-file-system", "x", true, "不跨越文件系统边界（跨盘用 --one-file-system=false）")
	cmd.Flags().BoolP("all", "a", false, "含隐藏文件和目录")
	cmd.Flags().Bool("files", false, "把文件也列出来（默认只列目录）")
	cmd.Flags().Int("jobs", 0, "并发度（0 = 自动；机械盘建议调到 2-4）")
	cmd.Flags().Bool("no-color", false, "关闭染色（同时尊重 NO_COLOR 环境变量）")
	cmd.Flags().Bool("verbose", false, "列出每条无权访问的路径")
	cmd.Flags().Bool("json", false, "输出全树 JSON（不受 --top/--depth 影响）")
	return cmd
}

// defaultSizeJobs 按 CPU 数给个上限 16 的默认并发度。
//
// 并发度该按存储介质定而不是 CPU：SSD 上 8-16 最好，机械盘 2-4（并发反而
// 因寻道变慢）。自动探测介质类型成本高，所以默认按 SSD 来，机械盘用户用
// --jobs 调低。实测 ~/.claude（11793 文件，热缓存）：jobs=1 194ms、
// jobs=8 31ms、jobs=16 33ms。
func defaultSizeJobs() int {
	n := runtime.NumCPU()
	if n > 16 {
		n = 16
	}
	if n < 1 {
		n = 1
	}
	return n
}

// noColorEnv 尊重 NO_COLOR 事实标准（https://no-color.org）。
func noColorEnv() bool {
	_, ok := os.LookupEnv("NO_COLOR")
	return ok
}

// startSizeProgress 起一个节流的进度显示，返回停止函数。
//
// 轮询原子计数器而不是接回调：每个目录回调一次既浪费又会乱序（多个 worker
// 各自 Add 后再回调，进度数字会往回跳）。
func startSizeProgress(w io.Writer, scanned *atomic.Uint64) func() {
	done := make(chan struct{})
	finished := make(chan struct{})

	go func() {
		defer close(finished)
		ticker := time.NewTicker(sizeProgressInterval)
		defer ticker.Stop()
		painted := false
		for {
			select {
			case <-done:
				if painted {
					fmt.Fprint(w, "\r\033[K") // 清掉进度行，别留在最终输出里
				}
				return
			case <-ticker.C:
				fmt.Fprintf(w, "\r\033[K已扫描 %s 个条目…", termx.Comma(scanned.Load()))
				painted = true
			}
		}
	}()

	return func() {
		close(done)
		<-finished
	}
}

func init() {
	rootCmd.AddCommand(newSizeCommand(sizeCmdDeps{}))
}
