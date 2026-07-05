package cli

import (
	"fmt"
	"io"
	"os"
	"strings"
	"unicode"

	"github.com/spf13/cobra"

	"github.com/xunull/jdan/internal/pinyinx"
	"github.com/xunull/jdan/internal/t9x"
)

type t9Deps struct {
	out      io.Writer
	errOut   io.Writer
	in       io.Reader
	pinyinOf func(rune) string // 注入：汉字 → 拼音（无声调）；nil → go-pinyin
}

func newT9Command(deps t9Deps) *cobra.Command {
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
		deps.pinyinOf = realPinyin
	}
	cmd := &cobra.Command{
		Use:   "t9 [text...]",
		Short: "中文/英文 → 九宫格(T9)按键序列（汉字按拼音）",
		Long: `把一段文字翻成九宫格键盘（T9）上实际要按的数字键。
汉字先转拼音再映射（中 → zhong → 94664），英文字母直接映射（hi → 44）。
阿拉伯数字原样；空格/标点跳过；无法识别的字符跳过并计数（走 stderr）。

键位：2 abc / 3 def / 4 ghi / 5 jkl / 6 mno / 7 pqrs / 8 tuv / 9 wxyz。

例：
  jdan t9 中文           # 逐字对照 + 底部整串数字
  jdan t9 "你好 hi"      # 中英混排
  echo 中国 | jdan t9    # 管道
  jdan t9 中文 --digits  # 只出数字串（可管道）
  jdan t9 中文 --json    # 机读

多音字取最常见读音，个别可能不准。0 新 Go 逻辑依赖（汉字→拼音用 go-pinyin 字典）。`,
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

			res := buildT9(text, deps.pinyinOf)

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

// buildT9 把文本切成单元：逐个汉字、连续英文单词、连续数字段；空格/标点跳过。
func buildT9(text string, pinyinOf func(rune) string) t9x.Result {
	var res t9x.Result
	rs := []rune(text)
	for i := 0; i < len(rs); {
		r := rs[i]
		switch {
		case isHan(r):
			py := pinyinOf(r)
			if d := t9x.LettersToDigits(py); d != "" {
				res.Units = append(res.Units, t9x.Unit{Text: string(r), Pinyin: py, Digits: d})
			} else {
				res.Skipped++ // 字典里没这个字的拼音
			}
			i++
		case isASCIILetter(r):
			j := i
			for j < len(rs) && isASCIILetter(rs[j]) {
				j++
			}
			w := string(rs[i:j])
			res.Units = append(res.Units, t9x.Unit{Text: w, Digits: t9x.LettersToDigits(w)})
			i = j
		case r >= '0' && r <= '9':
			j := i
			for j < len(rs) && rs[j] >= '0' && rs[j] <= '9' {
				j++
			}
			d := string(rs[i:j])
			res.Units = append(res.Units, t9x.Unit{Text: d, Digits: d})
			i = j
		case unicode.IsSpace(r) || unicode.IsPunct(r) || unicode.IsSymbol(r):
			i++ // 分隔/标点，静默跳过
		default:
			res.Skipped++ // emoji / 其他文种等
			i++
		}
	}
	return res
}

func isHan(r rune) bool {
	return (r >= 0x4E00 && r <= 0x9FFF) || // CJK 统一表意
		(r >= 0x3400 && r <= 0x4DBF) // 扩展 A
}

func isASCIILetter(r rune) bool {
	return (r >= 'a' && r <= 'z') || (r >= 'A' && r <= 'Z')
}

// realPinyin 取一个汉字的无声调拼音（最常见读音），供 t9/sp/spt9 共用。
// 委托给 pinyinx（拼音基建归一处）。
func realPinyin(r rune) string {
	return pinyinx.Plain(r)
}

func init() {
	rootCmd.AddCommand(newT9Command(t9Deps{}))
}
