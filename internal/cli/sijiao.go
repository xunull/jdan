package cli

import (
	"bufio"
	"fmt"
	"io"
	"os"
	"strings"

	"github.com/spf13/cobra"

	"github.com/xunull/jdan/internal/sijiaox"
)

type sijiaoDeps struct {
	out io.Writer
	in  io.Reader
}

func newSijiaoCommand(deps sijiaoDeps) *cobra.Command {
	if deps.out == nil {
		deps.out = os.Stdout
	}
	if deps.in == nil {
		deps.in = os.Stdin
	}

	cmd := &cobra.Command{
		Use:   "sijiao [text...]",
		Short: "查汉字四角号码（王云五检字法；数据来自 Unicode Unihan）",
		Long: `查汉字的四角号码。只做正查（字→码），反查（码→字）暂不支持。

四角号码看字四个角的笔形取 4 位主码 + 1 位附号（无附号则 4 位）。这只能查表——
四角靠字形，从码点算不出来（同笔顺）。数据是 Unicode Unihan 的 kFourCornerCode，
离线查表，收录约 1.69 万常用/传统字（深扩展区生僻字无码）。

十类笔形口诀（教学用，本命令只给码、不做逐角分解）：
  横一 垂二 三点捺 叉四 插五 方框六 七角 八八 九是小 点下有横变零头

非汉字（字母/数字/标点/emoji）跳过不计。是汉字但表里查不到的标「无」。

例：
  jdan sijiao 王              # 王  1010.4
  jdan sijiao 你              # 你  2729.0, 2729.2   （多值都列）
  jdan sijiao 口业专          # 口 6000.0 / 业 3210 / 专 5030
  echo 王 | jdan sijiao        # 从 stdin 读
  jdan sijiao --json 你口`,
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
				return fmt.Errorf("没有输入。用法：jdan sijiao 王（或管道传入）")
			}

			res := sijiaox.StringCodes(text)
			if asJSON {
				return writeIndentJSON(deps.out, sijiaoJSON(res))
			}
			writeSijiaoText(deps.out, res)
			return nil
		},
	}

	cmd.Flags().Bool("json", false, "结构化输出")
	return cmd
}

func writeSijiaoText(w io.Writer, res sijiaox.Result) {
	if len(res.Chars) == 0 {
		fmt.Fprintln(w, "没有汉字。")
		return
	}

	// 单字：一行。多值用 ", " 并列。
	if len(res.Chars) == 1 {
		c := res.Chars[0]
		if c.Known {
			fmt.Fprintf(w, "%c  %s\n", c.Rune, strings.Join(c.Codes, ", "))
		} else {
			fmt.Fprintf(w, "%c  无（不在四角号码表）\n", c.Rune)
		}
		return
	}

	// 多字：字与字之间用 " / "，同字多码用 ", "。
	parts := make([]string, 0, len(res.Chars))
	for _, c := range res.Chars {
		if c.Known {
			parts = append(parts, fmt.Sprintf("%c %s", c.Rune, strings.Join(c.Codes, ", ")))
		} else {
			parts = append(parts, fmt.Sprintf("%c ?", c.Rune))
		}
	}
	fmt.Fprintln(w, strings.Join(parts, " / "))

	if res.Unknown > 0 {
		fmt.Fprintf(w, "（另有 %d 字无四角号码，标为 ?）\n", res.Unknown)
	}
}

// sijiaoJSON 是 --json 的结构。
type sijiaoJSONChar struct {
	Char  string   `json:"char"`
	Codes []string `json:"codes"`
	Known bool     `json:"known"`
}

type sijiaoJSONResult struct {
	Chars   []sijiaoJSONChar `json:"chars"`
	Unknown int              `json:"unknown"`
}

func sijiaoJSON(res sijiaox.Result) sijiaoJSONResult {
	out := sijiaoJSONResult{
		Chars:   make([]sijiaoJSONChar, 0, len(res.Chars)),
		Unknown: res.Unknown,
	}
	for _, c := range res.Chars {
		codes := c.Codes
		if codes == nil {
			codes = []string{} // 表外字：JSON 里是 []，不是 null
		}
		out.Chars = append(out.Chars, sijiaoJSONChar{
			Char:  string(c.Rune),
			Codes: codes,
			Known: c.Known,
		})
	}
	return out
}

func init() {
	rootCmd.AddCommand(newSijiaoCommand(sijiaoDeps{}))
}
