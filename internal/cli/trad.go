package cli

import (
	"bufio"
	"fmt"
	"io"
	"os"
	"strings"
	"unicode/utf8"

	"github.com/spf13/cobra"

	"github.com/xunull/jdan/internal/tradx"
)

type tradDeps struct {
	out io.Writer
	in  io.Reader
}

func newTradCommand(deps tradDeps) *cobra.Command {
	if deps.out == nil {
		deps.out = os.Stdout
	}
	if deps.in == nil {
		deps.in = os.Stdin
	}

	cmd := &cobra.Command{
		Use:   "trad [text...]",
		Short: "简繁转换（词汇级：发→發/髮 消歧、软件→軟體 地区词；数据来自 OpenCC）",
		Long: `中文简↔繁转换。不止换字形，还按词消歧、可选地区用词——这是逐字替换给不了的。
数据是 OpenCC（Apache-2.0）离线词典，前向最长匹配。

方向用 --to 选：
  t    简→繁（标准，默认）      jdan trad 头发           # 頭髮（发→髮，靠词消歧）
  tw   简→繁台湾字形变体        jdan trad --to tw 里面
  twp  简→繁台湾（含地区用词）  jdan trad --to twp 软件  # 軟體（网络→網路、打印机→印表機）
  hk   简→繁香港字形变体        jdan trad --to hk 里面
  s    繁→简                    jdan trad --to s 軟體     # 软体

非汉字（英文/数字/标点/emoji）原样透传。无参数从 stdin 读（大输入逐行处理）。
--json 结构化输出；--diff 标出改动段。

例：
  jdan trad 头发和发展            # 頭髮和發展（同字不同繁，按词分）
  jdan trad --to twp 软件网络     # 軟體網路
  echo 软件 | jdan trad --to twp  # 从 stdin 读
  jdan trad --diff --to twp 软件网络
  jdan trad --json --to twp 软件

边界（诚实划界）：简繁+地区词到 OpenCC 词典为止，不做全量翻译。s2t/t2s 与 OpenCC 一致；
tw/twp/hk 未做 MMSEG 预分词，极少数跨词边界例可能与 OpenCC 有别。另有约 1.7% 台湾地区词
（多为外国地名）因缺 OpenCC 编译期生成词典而回转不到，属已知缺口。`,
		Args:          cobra.ArbitraryArgs,
		SilenceUsage:  true,
		SilenceErrors: true,
		RunE: func(cmd *cobra.Command, args []string) error {
			target, _ := cmd.Flags().GetString("to")
			asJSON, _ := cmd.Flags().GetBool("json")
			diff, _ := cmd.Flags().GetBool("diff")

			conv, err := tradx.NewConverter(target)
			if err != nil {
				return err
			}

			fromStdin := len(args) == 0

			// 结构化 / diff：需要"原文↔译文"整体对比，走缓冲路径。
			if asJSON || diff {
				text := strings.Join(args, " ")
				if fromStdin {
					b, err := io.ReadAll(bufio.NewReader(deps.in))
					if err != nil {
						return fmt.Errorf("读取输入失败：%w", err)
					}
					text = strings.TrimRight(string(b), "\r\n")
				}
				if text == "" {
					return fmt.Errorf("没有输入。用法：jdan trad 头发（或管道传入）")
				}
				out := conv.Convert(text)
				if asJSON {
					return writeIndentJSON(deps.out, tradJSONResult{
						Config:  target,
						From:    text,
						To:      out,
						Changed: tradx.Diff(text, out),
					})
				}
				writeTradDiff(deps.out, text, out)
				return nil
			}

			// 普通模式：有参数直接转；无参数逐行流式读 stdin（大管道不爆内存）。
			if !fromStdin {
				text := strings.Join(args, " ")
				if text == "" {
					return fmt.Errorf("没有输入。用法：jdan trad 头发")
				}
				fmt.Fprintln(deps.out, conv.Convert(text))
				return nil
			}

			sc := bufio.NewScanner(deps.in)
			sc.Buffer(make([]byte, 0, 64*1024), 4*1024*1024)
			any := false
			for sc.Scan() {
				any = true
				fmt.Fprintln(deps.out, conv.Convert(sc.Text()))
			}
			if err := sc.Err(); err != nil {
				return fmt.Errorf("读取输入失败：%w", err)
			}
			if !any {
				return fmt.Errorf("没有输入。用法：jdan trad 头发（或管道传入）")
			}
			return nil
		},
	}

	cmd.Flags().String("to", "t", "转换目标：t(简→繁)/tw/twp/hk/s(繁→简)")
	cmd.Flags().Bool("json", false, "结构化输出")
	cmd.Flags().Bool("diff", false, "标出改动段")
	return cmd
}

// writeTradDiff 用「」括出改动段，并列出每处 原→新。
func writeTradDiff(w io.Writer, in, out string) {
	changes := tradx.Diff(in, out)
	a := []rune(in)
	prev := 0
	var b strings.Builder
	for _, ch := range changes {
		b.WriteString(string(a[prev:ch.Pos]))
		b.WriteString("「")
		b.WriteString(ch.Conv)
		b.WriteString("」")
		prev = ch.Pos + utf8.RuneCountInString(ch.Orig)
	}
	b.WriteString(string(a[prev:]))
	fmt.Fprintln(w, b.String())

	if len(changes) == 0 {
		fmt.Fprintln(w, "（无改动）")
		return
	}
	fmt.Fprintf(w, "改动 %d 处：\n", len(changes))
	for _, ch := range changes {
		fmt.Fprintf(w, "  %s → %s\n", ch.Orig, ch.Conv)
	}
}

type tradJSONResult struct {
	Config  string         `json:"config"`
	From    string         `json:"from"`
	To      string         `json:"to"`
	Changed []tradx.Change `json:"changed"`
}

func init() {
	rootCmd.AddCommand(newTradCommand(tradDeps{}))
}
