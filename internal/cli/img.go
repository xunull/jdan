package cli

import (
	"bytes"
	"encoding/json"
	"fmt"
	"io"
	"os"

	"github.com/spf13/cobra"

	"github.com/xunull/jdan/internal/imageinfo"
)

type imgCmdDeps struct {
	out io.Writer
	in  io.Reader
}

func newImgCommand(deps imgCmdDeps) *cobra.Command {
	if deps.out == nil {
		deps.out = os.Stdout
	}
	if deps.in == nil {
		deps.in = os.Stdin
	}
	cmd := &cobra.Command{
		Use:   "img [file...]",
		Short: "读图片文件头报出尺寸/格式/颜色/大小（PNG/JPEG/GIF）",
		Long: `只读图片文件头报出尺寸/格式/颜色模型/大小，不解码整张图。0 新依赖。

支持 PNG / JPEG / GIF（stdlib 解码器）。

例：
  jdan img logo.png            # 单文件详细信息
  jdan img *.jpg               # 批量 → 对齐表格
  cat logo.png | jdan img      # stdin
  jdan img logo.png --json     # JSON 数组`,
		Args:          cobra.ArbitraryArgs,
		SilenceUsage:  true,
		SilenceErrors: true,
		RunE: func(cmd *cobra.Command, args []string) error {
			asJSON, _ := cmd.Flags().GetBool("json")
			return runImg(deps, args, asJSON)
		},
	}
	cmd.Flags().Bool("json", false, "JSON 数组输出")
	return cmd
}

func runImg(deps imgCmdDeps, args []string, asJSON bool) error {
	var infos []imageinfo.Info
	var failed bool

	if len(args) == 0 {
		// stdin 模式
		data, err := io.ReadAll(deps.in)
		if err != nil {
			return err
		}
		info, err := imageinfo.Inspect("<stdin>", bytes.NewReader(data), int64(len(data)))
		if err != nil {
			return err
		}
		infos = append(infos, info)
	} else {
		for _, path := range args {
			info, err := inspectFile(path)
			if err != nil {
				fmt.Fprintf(deps.out, "%s: %v\n", path, err)
				failed = true
				continue
			}
			infos = append(infos, info)
		}
	}

	if asJSON {
		if err := writeImgJSON(deps.out, infos); err != nil {
			return err
		}
	} else {
		writeImgText(deps.out, infos)
	}

	if failed {
		return fmt.Errorf("部分文件无法读取")
	}
	return nil
}

func inspectFile(path string) (imageinfo.Info, error) {
	f, err := os.Open(path)
	if err != nil {
		return imageinfo.Info{}, err
	}
	defer f.Close()
	fi, err := f.Stat()
	if err != nil {
		return imageinfo.Info{}, err
	}
	info, err := imageinfo.Inspect(path, f, fi.Size())
	if err != nil {
		// 去掉 Inspect 包的 "path: " 前缀，避免 CLI 再重复 path
		return imageinfo.Info{}, fmt.Errorf("不是支持的图片或已损坏")
	}
	return info, nil
}

func writeImgJSON(out io.Writer, infos []imageinfo.Info) error {
	if infos == nil {
		infos = []imageinfo.Info{}
	}
	data, err := json.MarshalIndent(infos, "", "  ")
	if err != nil {
		return err
	}
	fmt.Fprintln(out, string(data))
	return nil
}

func writeImgText(out io.Writer, infos []imageinfo.Info) {
	switch len(infos) {
	case 0:
		return
	case 1:
		writeImgDetail(out, infos[0])
	default:
		writeImgTable(out, infos)
	}
}

func writeImgDetail(out io.Writer, i imageinfo.Info) {
	color := i.Color
	if i.HasAlpha() {
		color += " (含 alpha)"
	}
	fmt.Fprintln(out, i.Path)
	fmt.Fprintf(out, "  格式: %s\n", upperFormat(i.Format))
	fmt.Fprintf(out, "  尺寸: %d x %d\n", i.Width, i.Height)
	fmt.Fprintf(out, "  颜色: %s\n", color)
	fmt.Fprintf(out, "  大小: %s\n", imageinfo.HumanizeBytes(i.Bytes))
}

func writeImgTable(out io.Writer, infos []imageinfo.Info) {
	// 计算各列宽（按 rune 对齐够用，路径/尺寸都是 ASCII）
	var pathW, dimW int
	dims := make([]string, len(infos))
	for idx, i := range infos {
		if len(i.Path) > pathW {
			pathW = len(i.Path)
		}
		dims[idx] = fmt.Sprintf("%dx%d", i.Width, i.Height)
		if len(dims[idx]) > dimW {
			dimW = len(dims[idx])
		}
	}
	for idx, i := range infos {
		fmt.Fprintf(out, "%-*s  %-*s  %-5s  %s\n",
			pathW, i.Path, dimW, dims[idx], upperFormat(i.Format), imageinfo.HumanizeBytes(i.Bytes))
	}
}

func upperFormat(f string) string {
	switch f {
	case "png":
		return "PNG"
	case "jpeg":
		return "JPEG"
	case "gif":
		return "GIF"
	default:
		return f
	}
}

func init() {
	rootCmd.AddCommand(newImgCommand(imgCmdDeps{}))
}
