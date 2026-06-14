package cli

import (
	"encoding/json"
	"fmt"
	"io"
	"os"

	"github.com/spf13/cobra"

	"github.com/xunull/jdan/internal/extract"
)

type extractCmdDeps struct {
	out io.Writer
}

func newExtractCommand(deps extractCmdDeps) *cobra.Command {
	if deps.out == nil {
		deps.out = os.Stdout
	}
	cmd := &cobra.Command{
		Use:   "extract <archive>",
		Short: "通用解压 .zip / .tar / .tar.gz / .tar.bz2 / .gz / .bz2",
		Long: `解压 archive 到目标目录。识别 8 种格式（按文件扩展名），
拒绝 directory traversal（".." 跳出 root）。

默认解压到当前目录的 <archive-name>/ 子目录。
.tar.gz / .tar.bz2 / .tgz 的子目录名是去掉双后缀的 base。

例：
  jdan extract release.tar.gz             # → ./release/
  jdan extract data.zip -o /tmp/out       # → /tmp/out/
  jdan extract docs.zip --here            # → ./<flat 解压>
  jdan extract data.zip --list            # 只列内容不解压
  jdan extract data.zip --json            # 结构化输出

支持：.zip .tar .tar.gz .tgz .tar.bz2 .tbz2 .gz .bz2
不支持：.7z（外部 lib 复杂） / .tar.xz（新 dep 太重，v1 不做）`,
		Args:          cobra.ExactArgs(1),
		SilenceUsage:  true,
		SilenceErrors: true,
		RunE: func(cmd *cobra.Command, args []string) error {
			return runExtract(cmd, args[0], deps.out)
		},
	}
	cmd.Flags().StringP("output", "o", "", "解压目标目录（默认当前目录的 <archive-name>/ 子目录）")
	cmd.Flags().Bool("here", false, "解压到当前目录而不是子目录（覆盖 -o）")
	cmd.Flags().Bool("list", false, "只列内容，不实际解压")
	cmd.Flags().Bool("json", false, "结构化 JSON 输出")
	return cmd
}

func runExtract(cmd *cobra.Command, archivePath string, out io.Writer) error {
	outDir, _ := cmd.Flags().GetString("output")
	here, _ := cmd.Flags().GetBool("here")
	listOnly, _ := cmd.Flags().GetBool("list")
	asJSON, _ := cmd.Flags().GetBool("json")

	// 验证 archive 文件存在
	if _, err := os.Stat(archivePath); err != nil {
		return err
	}

	if listOnly {
		// list 模式不需要 OutDir
		entries, err := extract.Extract(archivePath, extract.Options{ListOnly: true})
		if err != nil {
			return err
		}
		return renderExtractList(out, archivePath, entries, asJSON)
	}

	// 决定 OutDir
	switch {
	case here:
		outDir = "."
	case outDir == "":
		outDir = extract.DefaultOutDir(archivePath)
	}

	entries, err := extract.Extract(archivePath, extract.Options{
		OutDir: outDir,
	})
	if err != nil {
		return err
	}

	if asJSON {
		payload := map[string]any{
			"archive":     archivePath,
			"output_dir":  outDir,
			"entry_count": len(entries),
			"entries":     entries,
		}
		enc := json.NewEncoder(out)
		enc.SetIndent("", "  ")
		return enc.Encode(payload)
	}

	fmt.Fprintf(out, "✓ extracted %d entry(ies) to %s\n", len(entries), outDir)
	return nil
}

func renderExtractList(out io.Writer, archivePath string, entries []extract.Entry, asJSON bool) error {
	if asJSON {
		payload := map[string]any{
			"archive": archivePath,
			"entries": entries,
		}
		enc := json.NewEncoder(out)
		enc.SetIndent("", "  ")
		return enc.Encode(payload)
	}

	var totalSize int64
	for _, e := range entries {
		totalSize += e.Size
	}
	fmt.Fprintf(out, "archive: %s  (%d entries, %s total)\n\n", archivePath, len(entries), humanSize(totalSize))
	for _, e := range entries {
		mark := " "
		if e.IsDir {
			mark = "d"
		}
		fmt.Fprintf(out, "%s %10s  %s\n", mark, humanSize(e.Size), e.Name)
	}
	return nil
}

func humanSize(n int64) string {
	const k = 1024
	switch {
	case n == 0:
		return "-"
	case n < k:
		return fmt.Sprintf("%dB", n)
	case n < k*k:
		return fmt.Sprintf("%.1fKB", float64(n)/k)
	case n < k*k*k:
		return fmt.Sprintf("%.1fMB", float64(n)/(k*k))
	}
	return fmt.Sprintf("%.1fGB", float64(n)/(k*k*k))
}

func init() {
	rootCmd.AddCommand(newExtractCommand(extractCmdDeps{}))
}
