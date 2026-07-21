package cli

import (
	"bufio"
	"fmt"
	"io"
	"os"
	"strings"

	"github.com/spf13/cobra"

	"github.com/xunull/jdan/internal/strokesx"
)

type strokesDeps struct {
	out io.Writer
	in  io.Reader
}

func newStrokesCommand(deps strokesDeps) *cobra.Command {
	if deps.out == nil {
		deps.out = os.Stdout
	}
	if deps.in == nil {
		deps.in = os.Stdin
	}

	cmd := &cobra.Command{
		Use:   "strokes [text...]",
		Short: "查汉字笔画数（逐字 + 总数；数据来自 Unicode Unihan）",
		Long: `查汉字笔画数。整句逐字列出并给总数——这是输入法给不了的（输入法只显示
你正在打的那一个字）。数据是 Unicode Unihan 的 kTotalStrokes，离线查表，
覆盖全部 CJK 汉字含扩展区（10 万+），起名用的生僻字也查得到。

只做笔画数，不做笔顺（横竖撇捺折序列）——笔顺没有权威的开放机读数据。

非汉字（字母/数字/标点/emoji）跳过不计。表里查不到的汉字标为「未知」，
总数不含它但会提示。

例：
  jdan strokes 龙                # 龙  5 画
  jdan strokes 龙凤呈祥          # 逐字 + 总数 26
  echo 龙凤呈祥 | jdan strokes    # 从 stdin 读
  jdan strokes 鑫 龗             # 生僻字：24 / 33
  jdan strokes --json 龙凤`,
		Args:          cobra.ArbitraryArgs,
		SilenceUsage:  true,
		SilenceErrors: true,
		RunE: func(cmd *cobra.Command, args []string) error {
			asJSON, _ := cmd.Flags().GetBool("json")

			text := strings.Join(args, "")
			if text == "" {
				b, err := io.ReadAll(bufio.NewReader(deps.in))
				if err != nil {
					return fmt.Errorf("读取输入失败：%w", err)
				}
				text = strings.TrimRight(string(b), "\r\n")
			}
			if strings.TrimSpace(text) == "" {
				return fmt.Errorf("没有输入。用法：jdan strokes 龙凤呈祥（或管道传入）")
			}

			res := strokesx.StringStrokes(text)

			if asJSON {
				return writeIndentJSON(deps.out, strokesJSON(res))
			}
			writeStrokesText(deps.out, res)
			return nil
		},
	}

	cmd.Flags().Bool("json", false, "结构化输出")
	return cmd
}

func writeStrokesText(w io.Writer, res strokesx.Result) {
	if len(res.Chars) == 0 {
		fmt.Fprintln(w, "没有汉字。")
		return
	}

	// 单字：一行简洁输出。
	if len(res.Chars) == 1 {
		c := res.Chars[0]
		if c.Known {
			fmt.Fprintf(w, "%c  %d 画\n", c.Rune, c.Strokes)
		} else {
			fmt.Fprintf(w, "%c  未知（不在 Unihan 笔画表）\n", c.Rune)
		}
		return
	}

	// 多字：逐字 + 总数。
	parts := make([]string, 0, len(res.Chars))
	for _, c := range res.Chars {
		if c.Known {
			parts = append(parts, fmt.Sprintf("%c %d", c.Rune, c.Strokes))
		} else {
			parts = append(parts, fmt.Sprintf("%c ?", c.Rune))
		}
	}
	fmt.Fprintln(w, strings.Join(parts, " / "))

	total := fmt.Sprintf("共 %d 画", res.Total)
	if res.Unknown > 0 {
		total += fmt.Sprintf("（另有 %d 字不在笔画表，未计入，总数可能偏小）", res.Unknown)
	}
	fmt.Fprintln(w, total)
}

// strokesJSON 是 --json 的结构。
type strokesJSONChar struct {
	Char    string `json:"char"`
	Strokes int    `json:"strokes"`
	Known   bool   `json:"known"`
}

type strokesJSONResult struct {
	Chars   []strokesJSONChar `json:"chars"`
	Total   int               `json:"total"`
	Unknown int               `json:"unknown"`
}

func strokesJSON(res strokesx.Result) strokesJSONResult {
	out := strokesJSONResult{
		Chars:   make([]strokesJSONChar, 0, len(res.Chars)),
		Total:   res.Total,
		Unknown: res.Unknown,
	}
	for _, c := range res.Chars {
		out.Chars = append(out.Chars, strokesJSONChar{
			Char:    string(c.Rune),
			Strokes: c.Strokes,
			Known:   c.Known,
		})
	}
	return out
}

func init() {
	rootCmd.AddCommand(newStrokesCommand(strokesDeps{}))
}
