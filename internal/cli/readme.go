package cli

import (
	"fmt"
	"io"
	"os"
	"os/exec"

	"github.com/spf13/cobra"

	"github.com/xunull/jdan/internal/readme"
)

var readmeCmd = &cobra.Command{
	Use:   "readme [dir]",
	Short: "输出指定目录（默认当前目录）下的 README.md 内容",
	Long: `查找指定目录（或当前目录）下的 README.md 并输出其内容。
文件名大小写不敏感（README.md、readme.md、Readme.md 均可识别）。
若系统已安装 bat 则优先使用 bat（带语法高亮），否则使用 cat；
若两者都不可用（如 Windows 默认环境），则直接读取文件内容输出。

默认不分页（即使使用 bat 也直接一次性输出全部内容）。
使用 --paging 可强制启用 bat 的分页器（等同于 bat --paging=always）。`,
	Args: cobra.MaximumNArgs(1),
	RunE: func(cmd *cobra.Command, args []string) error {
		paging, _ := cmd.Flags().GetBool("paging")

		dir := "."
		if len(args) > 0 {
			dir = expandHome(args[0])
		}

		path, err := readme.FindReadme(dir)
		if err != nil {
			return err
		}
		return printReadmeFile(path, paging)
	},
}

func printReadmeFile(path string, paging bool) error {
	if batPath, err := exec.LookPath("bat"); err == nil {
		return runBat(batPath, path, paging)
	}
	if catPath, err := exec.LookPath("cat"); err == nil {
		return runViewer(catPath, path)
	}
	return copyFileToStdout(path)
}

func runBat(bin, path string, paging bool) error {
	pagingFlag := "--paging=never"
	if paging {
		pagingFlag = "--paging=always"
	}
	c := exec.Command(bin, pagingFlag, path)
	c.Stdout = os.Stdout
	c.Stderr = os.Stderr
	c.Stdin = os.Stdin
	return c.Run()
}

func runViewer(bin, path string) error {
	c := exec.Command(bin, path)
	c.Stdout = os.Stdout
	c.Stderr = os.Stderr
	c.Stdin = os.Stdin
	return c.Run()
}

func copyFileToStdout(path string) error {
	f, err := os.Open(path)
	if err != nil {
		return fmt.Errorf("打开文件失败: %w", err)
	}
	defer f.Close()
	_, err = io.Copy(os.Stdout, f)
	return err
}

func init() {
	readmeCmd.Flags().Bool("paging", false, "使用 bat 时强制启用分页（等同于 bat --paging=always）；默认不分页")
	rootCmd.AddCommand(readmeCmd)
}
