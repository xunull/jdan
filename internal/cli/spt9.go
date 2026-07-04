package cli

import (
	"fmt"
	"io"
	"os"
	"strings"
	"unicode"

	"github.com/spf13/cobra"

	"github.com/xunull/jdan/internal/spt9x"
	"github.com/xunull/jdan/internal/t9x"
)

type spt9Deps struct {
	out      io.Writer
	errOut   io.Writer
	in       io.Reader
	pinyinOf func(rune) string // 注入：汉字 → 拼音（无声调）；nil → go-pinyin
}

func newSPT9Command(deps spt9Deps) *cobra.Command {
	if deps.out == nil {
		deps.out = os.Stdout
	}
	if deps.errOut == nil {
		deps.errOut = os.Stderr
	}
	if deps.in == nil {
		deps.in = os.Stdin
	}
	if deps.pinyinOf == nil {
		deps.pinyinOf = realPinyin // 复用 t9.go 里的 go-pinyin 封装
	}
	cmd := &cobra.Command{
		Use:   "spt9 [text...]",
		Short: "中文 → 小鹤双拼九宫格(T9)按键（每字 2 键）",
		Long: `把中文翻成【小鹤双拼】在九宫格键盘上的按键 —— 每个字固定按 2 下。
先转拼音，再按小鹤方案拆成「一键声母 + 一键韵母」两码字母，两个字母各自落到
它的 T9 数字键。例：中 → zhong → 声母 zh=v、韵母 ong=s = "vs" → 8、7。

键位 2 abc / 3 def / 4 ghi / 5 jkl / 6 mno / 7 pqrs / 8 tuv / 9 wxyz。
小鹤方案照 RIME rime-double-pinyin 的 flypy 规则写死（声母 zh/ch/sh = v/i/u）。

例：
  jdan spt9 中文            # 逐字对照 + 底部整串
  jdan spt9 "你好世界"      # 每字 2 键
  echo 中国 | jdan spt9     # 管道
  jdan spt9 中文 --digits   # 只出数字串
  jdan spt9 中文 --json     # 机读

英文按普通 T9 字母映射、数字原样、空格/标点跳过。多音字取常见读音，个别可能不准。
跟 jdan t9（全拼九宫格，每字不定长）互补：这个是双拼，每字恒 2 键。`,
		SilenceUsage:  true,
		SilenceErrors: true,
		RunE: func(cmd *cobra.Command, args []string) error {
			digitsOnly, _ := cmd.Flags().GetBool("digits")
			asJSON, _ := cmd.Flags().GetBool("json")

			text := strings.Join(args, " ")
			if text == "" {
				b, err := io.ReadAll(deps.in)
				if err != nil {
					return err
				}
				text = strings.TrimRight(string(b), "\r\n")
			}
			if strings.TrimSpace(text) == "" {
				return fmt.Errorf("没有输入文本")
			}

			res := buildSPT9(text, deps.pinyinOf)

			switch {
			case asJSON:
				s, err := res.FormatJSON()
				if err != nil {
					return err
				}
				fmt.Fprintln(deps.out, s)
			case digitsOnly:
				fmt.Fprintln(deps.out, res.DigitString())
			default:
				fmt.Fprint(deps.out, res.Render())
			}

			if res.Skipped > 0 {
				fmt.Fprintf(deps.errOut, "（跳过 %d 个无法映射的字符）\n", res.Skipped)
			}
			return nil
		},
	}
	cmd.Flags().Bool("digits", false, "只输出整串数字（省掉逐字对照）")
	cmd.Flags().Bool("json", false, "JSON 输出")
	return cmd
}

// buildSPT9 切词并编码：逐个汉字（小鹤双拼两码）、连续英文单词（普通 T9）、
// 连续数字段（原样）；空格/标点跳过。
func buildSPT9(text string, pinyinOf func(rune) string) spt9x.Result {
	var res spt9x.Result
	rs := []rune(text)
	for i := 0; i < len(rs); {
		r := rs[i]
		switch {
		case isHan(r):
			py := pinyinOf(r)
			if code, ok := spt9x.Encode(py); ok {
				res.Units = append(res.Units, spt9x.Unit{
					Text: string(r), Pinyin: py, Code: code, Digits: t9x.LettersToDigits(code),
				})
			} else {
				res.Skipped++
			}
			i++
		case isASCIILetter(r):
			j := i
			for j < len(rs) && isASCIILetter(rs[j]) {
				j++
			}
			w := string(rs[i:j])
			res.Units = append(res.Units, spt9x.Unit{Text: w, Digits: t9x.LettersToDigits(w)})
			i = j
		case r >= '0' && r <= '9':
			j := i
			for j < len(rs) && rs[j] >= '0' && rs[j] <= '9' {
				j++
			}
			d := string(rs[i:j])
			res.Units = append(res.Units, spt9x.Unit{Text: d, Digits: d})
			i = j
		case unicode.IsSpace(r) || unicode.IsPunct(r) || unicode.IsSymbol(r):
			i++
		default:
			res.Skipped++
			i++
		}
	}
	return res
}

func init() {
	rootCmd.AddCommand(newSPT9Command(spt9Deps{}))
}
