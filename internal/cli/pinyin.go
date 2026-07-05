package cli

import (
	"encoding/json"
	"fmt"
	"io"
	"os"
	"strings"

	"github.com/spf13/cobra"

	"github.com/xunull/jdan/internal/pinyinx"
)

type pinyinDeps struct {
	out io.Writer
	in  io.Reader
}

func newPinyinCommand(deps pinyinDeps) *cobra.Command {
	if deps.out == nil {
		deps.out = os.Stdout
	}
	if deps.in == nil {
		deps.in = os.Stdin
	}
	cmd := &cobra.Command{
		Use:   "pinyin [text...]",
		Short: "中文 → 拼音（多种声调样式，非汉字原样保留）",
		Long: `把中文转成拼音。底层是 go-pinyin 的 ~4 万条 Unihan 读音表（离线查表）。
是 t9/sp/spt9 的「第一步」单独成命令。

样式（--style，默认 tone）：
  tone      zhōng wén     带调符（真·拼音）
  num       zhong1 wen2   数字调（纯 ASCII）
  plain     zhong wen     无调（文件名/变量名友好）
  initials  zh w          只声母（零声母字为空，如 文/王）
  first     z w           只首字母（缩写）

例：
  jdan pinyin 中文                   # zhōng wén
  jdan pinyin 中文 --style plain     # zhong wen
  jdan pinyin "Hello 世界 2024"      # Hello shì jiè 2024（非汉字穿插保留）
  jdan pinyin 银行 --heteronym       # yín háng/xíng（多音字列全部读音）
  jdan pinyin 中文 --sep -           # zhōng-wén
  echo 你好 | jdan pinyin            # 管道
  jdan pinyin 中文 --json            # 逐字结构化

多音字默认取最常见读音（逐字，不按词消歧）；个别可能不准。`,
		SilenceUsage:  true,
		SilenceErrors: true,
		RunE: func(cmd *cobra.Command, args []string) error {
			style, _ := cmd.Flags().GetString("style")
			heteronym, _ := cmd.Flags().GetBool("heteronym")
			sep, _ := cmd.Flags().GetString("sep")
			asJSON, _ := cmd.Flags().GetBool("json")

			if !pinyinx.StyleValid(style) {
				return fmt.Errorf("未知样式 %q（可选：%s）", style, strings.Join(pinyinx.StyleNames(), " "))
			}

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

			tokens := pinyinx.Convert(text, pinyinx.Options{Style: style, Heteronym: heteronym})
			if asJSON {
				enc := json.NewEncoder(deps.out)
				enc.SetIndent("", "  ")
				return enc.Encode(map[string]any{
					"input":  text,
					"style":  style,
					"tokens": tokens,
					"result": pinyinx.Join(tokens, sep),
				})
			}
			fmt.Fprintln(deps.out, pinyinx.Join(tokens, sep))
			return nil
		},
	}
	cmd.Flags().StringP("style", "s", "tone", "声调样式：tone/num/plain/initials/first")
	cmd.Flags().Bool("heteronym", false, "多音字列出全部读音（用 / 连）")
	cmd.Flags().String("sep", " ", "拼音音节之间的分隔符")
	cmd.Flags().Bool("json", false, "逐字结构化 JSON 输出")
	return cmd
}

func init() {
	rootCmd.AddCommand(newPinyinCommand(pinyinDeps{}))
}
