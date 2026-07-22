package cli

import (
	"bufio"
	"fmt"
	"io"
	"os"
	"strings"

	"github.com/spf13/cobra"

	"github.com/xunull/jdan/internal/jyutpingx"
)

type jyutpingDeps struct {
	out io.Writer
	in  io.Reader
}

func newJyutpingCommand(deps jyutpingDeps) *cobra.Command {
	if deps.out == nil {
		deps.out = os.Stdout
	}
	if deps.in == nil {
		deps.in = os.Stdin
	}

	cmd := &cobra.Command{
		Use:   "jyutping [text...]",
		Short: "查汉字粤拼（粤语读音，如 你 nei5；数据来自 Unicode Unihan）",
		Long: `查汉字的粤拼（Jyutping 粤语读音）——pinyin 给普通话，jyutping 给粤语。逐字对照输出。

数据是 Unicode Unihan 的 kCantonese（Jyutping 粤拼，声调 1-6），离线查表，收录约 2.99 万字。

诚实划界：kCantonese 每字只存一个主读音，**列不了多音字的其它读法**（不像 pinyin 的 --heteronym），
也不做词级上下文消歧（行只给 hang4，给不了"走路"的 haang4）。要多音+词级得换更全的粤语词典。

非汉字（字母/数字/标点/emoji）跳过不计。表里查不到的汉字标「无」。反查暂不支持。

例：
  jdan jyutping 你              # 你  nei5
  jdan jyutping 你好            # 你 nei5 / 好 hou2
  jdan jyutping 我爱广东        # 我 ngo5 / 爱 oi3 / 广 gwong2 / 东 dung1
  echo 你好 | jdan jyutping      # 从 stdin 读
  jdan jyutping --json 你好`,
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
				return fmt.Errorf("没有输入。用法：jdan jyutping 你好（或管道传入）")
			}

			res := jyutpingx.StringReadings(text)
			if asJSON {
				return writeIndentJSON(deps.out, jyutpingJSON(res))
			}
			writeJyutpingText(deps.out, res)
			return nil
		},
	}

	cmd.Flags().Bool("json", false, "结构化输出")
	return cmd
}

func writeJyutpingText(w io.Writer, res jyutpingx.Result) {
	if len(res.Chars) == 0 {
		fmt.Fprintln(w, "没有汉字。")
		return
	}

	// 单字：一行 `字 读音`。
	if len(res.Chars) == 1 {
		c := res.Chars[0]
		if c.Known {
			fmt.Fprintf(w, "%c  %s\n", c.Rune, c.Reading)
		} else {
			fmt.Fprintf(w, "%c  无（不在粤拼表）\n", c.Rune)
		}
		return
	}

	// 多字：字与字之间用 " / "。
	parts := make([]string, 0, len(res.Chars))
	for _, c := range res.Chars {
		if c.Known {
			parts = append(parts, fmt.Sprintf("%c %s", c.Rune, c.Reading))
		} else {
			parts = append(parts, fmt.Sprintf("%c ?", c.Rune))
		}
	}
	fmt.Fprintln(w, strings.Join(parts, " / "))

	if res.Unknown > 0 {
		fmt.Fprintf(w, "（另有 %d 字无粤拼，标为 ?）\n", res.Unknown)
	}
}

// jyutpingJSON 是 --json 的结构。非汉字不进 chars。
type jyutpingJSONChar struct {
	Char    string `json:"char"`
	Reading string `json:"reading"`
	Known   bool   `json:"known"`
}

type jyutpingJSONResult struct {
	Chars   []jyutpingJSONChar `json:"chars"`
	Unknown int                `json:"unknown"`
}

func jyutpingJSON(res jyutpingx.Result) jyutpingJSONResult {
	out := jyutpingJSONResult{
		Chars:   make([]jyutpingJSONChar, 0, len(res.Chars)),
		Unknown: res.Unknown,
	}
	for _, c := range res.Chars {
		out.Chars = append(out.Chars, jyutpingJSONChar{
			Char:    string(c.Rune),
			Reading: c.Reading,
			Known:   c.Known,
		})
	}
	return out
}

func init() {
	rootCmd.AddCommand(newJyutpingCommand(jyutpingDeps{}))
}
