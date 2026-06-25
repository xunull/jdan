package cli

import (
	"encoding/json"
	"fmt"
	"io"
	"os"

	"github.com/spf13/cobra"

	"github.com/xunull/jdan/internal/mimetype"
)

type mimeCmdDeps struct {
	out io.Writer
	in  io.Reader
}

// mimeInfo 是单个文件的检测结果。
type mimeInfo struct {
	Path        string `json:"path"`
	Mime        string `json:"mime"`
	Ext         string `json:"ext,omitempty"`
	ExtMismatch bool   `json:"ext_mismatch"`
}

func newMimeCommand(deps mimeCmdDeps) *cobra.Command {
	if deps.out == nil {
		deps.out = os.Stdout
	}
	if deps.in == nil {
		deps.in = os.Stdin
	}
	cmd := &cobra.Command{
		Use:   "mime [file...]",
		Short: "按内容（magic bytes）判断文件 MIME 类型，不看扩展名",
		Long: `读文件头的 magic bytes 报真实 content-type，不看扩展名（改了名也认得出）。
0 新依赖（stdlib http.DetectContentType + 精选 magic 表）。

例：
  jdan mime logo.png           # image/png
  jdan mime *.bin              # 批量 → 对齐表格
  jdan mime < file.bin         # stdin
  jdan mime weird.txt --json   # JSON 数组

扩展名与实测类型不符时会提示（如 .txt 实为 image/png）。`,
		Args:          cobra.ArbitraryArgs,
		SilenceUsage:  true,
		SilenceErrors: true,
		RunE: func(cmd *cobra.Command, args []string) error {
			asJSON, _ := cmd.Flags().GetBool("json")
			return runMime(deps, args, asJSON)
		},
	}
	cmd.Flags().Bool("json", false, "JSON 数组输出")
	return cmd
}

func runMime(deps mimeCmdDeps, args []string, asJSON bool) error {
	var infos []mimeInfo
	var failed bool

	if len(args) == 0 {
		data, err := readSniff(deps.in)
		if err != nil {
			return err
		}
		m := mimetype.Detect(data)
		infos = append(infos, mimeInfo{Path: "<stdin>", Mime: m})
	} else {
		for _, path := range args {
			info, err := inspectMime(path)
			if err != nil {
				fmt.Fprintf(deps.out, "%s: %v\n", path, err)
				failed = true
				continue
			}
			infos = append(infos, info)
		}
	}

	if asJSON {
		if err := writeMimeJSON(deps.out, infos); err != nil {
			return err
		}
	} else {
		writeMimeText(deps.out, infos)
	}

	if failed {
		return fmt.Errorf("部分文件无法读取")
	}
	return nil
}

func inspectMime(path string) (mimeInfo, error) {
	f, err := os.Open(path)
	if err != nil {
		return mimeInfo{}, err
	}
	defer f.Close()
	data, err := readSniff(f)
	if err != nil {
		return mimeInfo{}, err
	}
	m := mimetype.Detect(data)
	ext, mismatch := mimetype.ExtMismatch(path, m)
	return mimeInfo{Path: path, Mime: m, Ext: ext, ExtMismatch: mismatch}, nil
}

// readSniff 读前 MaxSniff 字节（足够所有 magic）。
func readSniff(r io.Reader) ([]byte, error) {
	buf := make([]byte, mimetype.MaxSniff())
	n, err := io.ReadFull(r, buf)
	if err != nil && err != io.EOF && err != io.ErrUnexpectedEOF {
		return nil, err
	}
	return buf[:n], nil
}

func writeMimeJSON(out io.Writer, infos []mimeInfo) error {
	if infos == nil {
		infos = []mimeInfo{}
	}
	data, err := json.MarshalIndent(infos, "", "  ")
	if err != nil {
		return err
	}
	fmt.Fprintln(out, string(data))
	return nil
}

func writeMimeText(out io.Writer, infos []mimeInfo) {
	switch len(infos) {
	case 0:
		return
	case 1:
		fmt.Fprintln(out, mimeLine(infos[0]))
	default:
		w := 0
		for _, i := range infos {
			if len(i.Path) > w {
				w = len(i.Path)
			}
		}
		for _, i := range infos {
			fmt.Fprintf(out, "%-*s  %s\n", w, i.Path, mimeLine(i))
		}
	}
}

// mimeLine 返回 mime（不符时附扩展名提示）。
func mimeLine(i mimeInfo) string {
	if i.ExtMismatch {
		return fmt.Sprintf("%s   (扩展名 %s 不符)", i.Mime, i.Ext)
	}
	return i.Mime
}

func init() {
	rootCmd.AddCommand(newMimeCommand(mimeCmdDeps{}))
}
