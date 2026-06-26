package cli

import (
	"fmt"
	"io"
	"os"
	"sort"

	"github.com/spf13/cobra"

	"github.com/xunull/jdan/internal/diskx"
)

type diskCmdDeps struct {
	out      io.Writer
	mounts   func() ([]diskx.Mount, error)     // 注入便于测试；零值用 diskx.Mounts
	statPath func(string) (diskx.Mount, error) // 注入便于测试；零值用 diskx.StatPath
}

func newDiskCommand(deps diskCmdDeps) *cobra.Command {
	if deps.out == nil {
		deps.out = os.Stdout
	}
	if deps.mounts == nil {
		deps.mounts = diskx.Mounts
	}
	if deps.statPath == nil {
		deps.statPath = diskx.StatPath
	}
	cmd := &cobra.Command{
		Use:   "disk [path]",
		Short: "磁盘使用一览（各挂载点容量/占用，df 式）",
		Long: `像 df：列各挂载点的容量/已用/可用/使用率，带使用率条和高占用染色。
给路径则只看该路径所在的文件系统。0 新依赖（纯 syscall）。仅 darwin / linux。

例：
  jdan disk            列所有真实挂载点
  jdan disk /          只看根分区所在文件系统
  jdan disk -a         含伪文件系统（devfs/tmpfs/map…）
  jdan disk -i         显示 inode 用量
  jdan disk --bytes    原始字节
  jdan disk --json`,
		Args:          cobra.MaximumNArgs(1),
		SilenceUsage:  true,
		SilenceErrors: true,
		RunE: func(cmd *cobra.Command, args []string) error {
			all, _ := cmd.Flags().GetBool("all")
			inodes, _ := cmd.Flags().GetBool("inodes")
			asBytes, _ := cmd.Flags().GetBool("bytes")
			asJSON, _ := cmd.Flags().GetBool("json")
			noColor, _ := cmd.Flags().GetBool("no-color")

			var mounts []diskx.Mount
			if len(args) == 1 {
				m, err := deps.statPath(args[0])
				if err != nil {
					return fmt.Errorf("无法读取 %q：%w", args[0], err)
				}
				mounts = []diskx.Mount{m}
			} else {
				ms, err := deps.mounts()
				if err != nil {
					return err
				}
				mounts = diskx.Filter(ms, all)
			}
			sort.Slice(mounts, func(i, j int) bool { return mounts[i].Mountpoint < mounts[j].Mountpoint })

			if asJSON {
				return writeIndentJSON(deps.out, diskx.JSONData(mounts))
			}
			fmt.Fprint(deps.out, diskx.Render(mounts, diskx.RenderOptions{
				Inodes: inodes,
				Bytes:  asBytes,
				Color:  !noColor && isTTY(deps.out),
			}))
			return nil
		},
	}
	cmd.Flags().BoolP("all", "a", false, "显示伪文件系统（devfs/tmpfs/map…）")
	cmd.Flags().BoolP("inodes", "i", false, "显示 inode 用量")
	cmd.Flags().Bool("bytes", false, "原始字节而非人类可读")
	cmd.Flags().Bool("no-color", false, "关闭高占用染色")
	cmd.Flags().Bool("json", false, "结构化输出")
	return cmd
}

func init() {
	rootCmd.AddCommand(newDiskCommand(diskCmdDeps{}))
}
