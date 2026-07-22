package cli

import (
	"bufio"
	"fmt"
	"io"
	"os"
	"strings"

	"github.com/spf13/cobra"

	"github.com/xunull/jdan/internal/cangjiex"
)

type cangjieDeps struct {
	out io.Writer
	in  io.Reader
}

func newCangjieCommand(deps cangjieDeps) *cobra.Command {
	if deps.out == nil {
		deps.out = os.Stdout
	}
	if deps.in == nil {
		deps.in = os.Stdin
	}

	cmd := &cobra.Command{
		Use:   "cangjie [text...]",
		Short: "查汉字仓颉码（含字根，如 明 AB 日月；数据来自 Unicode Unihan）",
		Long: `查汉字的仓颉码（朱邦復输入法，台/港主流）。正查（字→码），并把字母码翻成字根一并显示。

仓颉把一个字拆成 1-5 个字根，每根对应一个字母键。明=日+月=AB。数据是 Unicode Unihan 的
kCangjie（仓颉三代），离线查表，收录约 2.9 万字。拆成哪几个字根靠字形、从码点算不出来，只能查表。

25 键字根表（速查）：
  A日 B月 C金 D木 E水 F火 G土   H竹 I戈 J十 K大 L中 M一 N弓
  O人 P心 Q手 R口   S尸 T廿 U山 V女 W田   Y卜   X難

非汉字（字母/数字/标点/emoji）跳过不计。表里查不到的汉字标「无」。反查（码→字）暂不支持。

例：
  jdan cangjie 明              # 明  AB（日月）
  jdan cangjie 你              # 你  ONF（人弓火）
  jdan cangjie 明变            # 逐字：明 AB（日月） / 变 …
  echo 明 | jdan cangjie        # 从 stdin 读
  jdan cangjie --json 明你`,
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
				return fmt.Errorf("没有输入。用法：jdan cangjie 明（或管道传入）")
			}

			res := cangjiex.StringCodes(text)
			if asJSON {
				return writeIndentJSON(deps.out, cangjieJSON(res))
			}
			writeCangjieText(deps.out, res)
			return nil
		},
	}

	cmd.Flags().Bool("json", false, "结构化输出")
	return cmd
}

func writeCangjieText(w io.Writer, res cangjiex.Result) {
	if len(res.Chars) == 0 {
		fmt.Fprintln(w, "没有汉字。")
		return
	}

	// 单字：一行 `码（字根）`。
	if len(res.Chars) == 1 {
		c := res.Chars[0]
		if c.Known {
			fmt.Fprintf(w, "%c  %s（%s）\n", c.Rune, c.Code, c.Roots)
		} else {
			fmt.Fprintf(w, "%c  无（不在仓颉表）\n", c.Rune)
		}
		return
	}

	// 多字：字与字之间用 " / "。
	parts := make([]string, 0, len(res.Chars))
	for _, c := range res.Chars {
		if c.Known {
			parts = append(parts, fmt.Sprintf("%c %s（%s）", c.Rune, c.Code, c.Roots))
		} else {
			parts = append(parts, fmt.Sprintf("%c ?", c.Rune))
		}
	}
	fmt.Fprintln(w, strings.Join(parts, " / "))

	if res.Unknown > 0 {
		fmt.Fprintf(w, "（另有 %d 字无仓颉码，标为 ?）\n", res.Unknown)
	}
}

// cangjieJSON 是 --json 的结构。非汉字不进 chars（与文本模式"跳过"一致）。
type cangjieJSONChar struct {
	Char  string `json:"char"`
	Code  string `json:"code"`
	Roots string `json:"roots"`
	Known bool   `json:"known"`
}

type cangjieJSONResult struct {
	Chars   []cangjieJSONChar `json:"chars"`
	Unknown int               `json:"unknown"`
}

func cangjieJSON(res cangjiex.Result) cangjieJSONResult {
	out := cangjieJSONResult{
		Chars:   make([]cangjieJSONChar, 0, len(res.Chars)),
		Unknown: res.Unknown,
	}
	for _, c := range res.Chars {
		out.Chars = append(out.Chars, cangjieJSONChar{
			Char:  string(c.Rune),
			Code:  c.Code,
			Roots: c.Roots,
			Known: c.Known,
		})
	}
	return out
}

func init() {
	rootCmd.AddCommand(newCangjieCommand(cangjieDeps{}))
}
