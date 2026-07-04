package cli

import (
	"encoding/json"
	"fmt"
	"io"
	"os"
	"strings"
	"unicode"

	"github.com/mattn/go-runewidth"
	"github.com/spf13/cobra"

	"github.com/xunull/jdan/internal/shuangpinx"
)

type spDeps struct {
	out      io.Writer
	errOut   io.Writer
	in       io.Reader
	pinyinOf func(rune) string // 注入；nil → go-pinyin
}

type spUnit struct {
	text   string
	pinyin string
	han    bool
}

func newSPCommand(deps spDeps) *cobra.Command {
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
		deps.pinyinOf = realPinyin // 复用 t9.go 的 go-pinyin 封装
	}
	cmd := &cobra.Command{
		Use:   "sp [text...]",
		Short: "中文 → 标准 26 键双拼按键（小鹤/自然码/微软/智能ABC/拼音加加）",
		Long: `把中文翻成标准 26 键双拼要按的字母键 —— 每个字 2 键（一键声母 + 一键韵母）。
先转拼音，再按所选双拼方案编成两码字母。例（小鹤）：中 → zhong → zh=v、ong=s → "vs"。

方案（--scheme，默认小鹤）：小鹤 flypy / 自然码 ziranma / 微软 mspy / 智能ABC abc /
拼音加加 pyjj（搜狗双拼=微软布局，--scheme sogou 即可）。规则逐条照 RIME
rime-double-pinyin 各 schema 抄，非凭记忆。

例：
  jdan sp 中文                    # 默认小鹤，逐字对照 + 底部整串
  jdan sp 中文 --scheme 微软       # 换方案（中文名或 id 均可）
  jdan sp 中文 --all              # 所有方案的结果一次对比
  jdan sp 中文 --codes            # 只出码串（可管道）
  jdan sp 中文 --json             # 机读

英文按字母本身、数字原样、标点跳过。多音字取常见读音，个别可能不准。
跟 jdan spt9（九宫格双拼，出数字键）互补：这个是 26 键，出字母键。`,
		SilenceUsage:  true,
		SilenceErrors: true,
		RunE: func(cmd *cobra.Command, args []string) error {
			schemeName, _ := cmd.Flags().GetString("scheme")
			all, _ := cmd.Flags().GetBool("all")
			codesOnly, _ := cmd.Flags().GetBool("codes")
			asJSON, _ := cmd.Flags().GetBool("json")

			scheme := shuangpinx.Default()
			if schemeName != "" {
				s, ok := shuangpinx.Get(schemeName)
				if !ok {
					return fmt.Errorf("未知方案 %q（可选：%s；搜狗=微软）", schemeName, strings.Join(shuangpinx.IDs(), " "))
				}
				scheme = s
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

			units, skipped := spTokenize(text, deps.pinyinOf)

			switch {
			case asJSON && all:
				fmt.Fprintln(deps.out, spJSONAll(units))
			case asJSON:
				fmt.Fprintln(deps.out, spJSONOne(units, scheme))
			case all:
				fmt.Fprint(deps.out, spRenderAll(units))
			case codesOnly:
				fmt.Fprintln(deps.out, spConcat(units, scheme))
			default:
				fmt.Fprint(deps.out, spRenderOne(units, scheme))
			}

			if skipped > 0 {
				fmt.Fprintf(deps.errOut, "（跳过 %d 个无法映射的字符）\n", skipped)
			}
			return nil
		},
	}
	cmd.Flags().StringP("scheme", "s", "", "双拼方案（小鹤/自然码/微软/智能ABC/拼音加加，默认小鹤）")
	cmd.Flags().Bool("all", false, "一次输出所有方案的结果对比")
	cmd.Flags().Bool("codes", false, "只输出整串码（省掉逐字对照）")
	cmd.Flags().Bool("json", false, "JSON 输出")
	return cmd
}

// spTokenize 切词：逐个汉字（带拼音）、连续英文单词、连续数字段；空格/标点跳过。
func spTokenize(text string, pinyinOf func(rune) string) ([]spUnit, int) {
	var units []spUnit
	skipped := 0
	rs := []rune(text)
	for i := 0; i < len(rs); {
		r := rs[i]
		switch {
		case isHan(r):
			py := pinyinOf(r)
			if py == "" {
				skipped++
			} else {
				units = append(units, spUnit{text: string(r), pinyin: py, han: true})
			}
			i++
		case isASCIILetter(r):
			j := i
			for j < len(rs) && isASCIILetter(rs[j]) {
				j++
			}
			units = append(units, spUnit{text: string(rs[i:j])})
			i = j
		case r >= '0' && r <= '9':
			j := i
			for j < len(rs) && rs[j] >= '0' && rs[j] <= '9' {
				j++
			}
			units = append(units, spUnit{text: string(rs[i:j])})
			i = j
		case unicode.IsSpace(r) || unicode.IsPunct(r) || unicode.IsSymbol(r):
			i++
		default:
			skipped++
			i++
		}
	}
	return units, skipped
}

// spCode 返回一个单元在某方案下的码：汉字走双拼编码，英文/数字为其本身（即按的键）。
func spCode(u spUnit, s shuangpinx.Scheme) string {
	if u.han {
		return s.Encode(u.pinyin)
	}
	return strings.ToLower(u.text)
}

func spConcat(units []spUnit, s shuangpinx.Scheme) string {
	parts := make([]string, 0, len(units))
	for _, u := range units {
		if c := spCode(u, s); c != "" {
			parts = append(parts, c)
		}
	}
	return strings.Join(parts, " ")
}

// spRenderOne 逐字对照（字·拼音·双拼两码）+ 底部整串。
func spRenderOne(units []spUnit, s shuangpinx.Scheme) string {
	if len(units) == 0 {
		return ""
	}
	wt, wp := 0, 0
	for _, u := range units {
		if w := runewidth.StringWidth(u.text); w > wt {
			wt = w
		}
		p := u.pinyin
		if p == "" {
			p = "—"
		}
		if len(p) > wp {
			wp = len(p)
		}
	}
	var b strings.Builder
	for _, u := range units {
		p := u.pinyin
		if p == "" {
			p = "—"
		}
		fmt.Fprintf(&b, "%s%s  %s%s  %s\n",
			u.text, pad(wt-runewidth.StringWidth(u.text)),
			p, pad(wp-runewidth.StringWidth(p)), spCode(u, s))
	}
	b.WriteString(strings.Repeat("─", 5) + "\n")
	b.WriteString(spConcat(units, s) + "\n")
	return b.String()
}

// spRenderAll 每个方案一行：方案名 + 整串码。
func spRenderAll(units []spUnit) string {
	wn := 0
	for _, s := range shuangpinx.All() {
		if w := runewidth.StringWidth(s.Name); w > wn {
			wn = w
		}
	}
	var b strings.Builder
	for _, s := range shuangpinx.All() {
		fmt.Fprintf(&b, "%s%s  %s\n", s.Name, pad(wn-runewidth.StringWidth(s.Name)), spConcat(units, s))
	}
	return b.String()
}

func pad(n int) string {
	if n <= 0 {
		return ""
	}
	return strings.Repeat(" ", n)
}

func spJSONOne(units []spUnit, s shuangpinx.Scheme) string {
	type u struct {
		Text   string `json:"text"`
		Pinyin string `json:"pinyin,omitempty"`
		Code   string `json:"code"`
	}
	arr := make([]u, 0, len(units))
	for _, x := range units {
		arr = append(arr, u{Text: x.text, Pinyin: x.pinyin, Code: spCode(x, s)})
	}
	out := struct {
		Scheme string `json:"scheme"`
		Units  []u    `json:"units"`
	}{Scheme: s.ID, Units: arr}
	data, _ := json.MarshalIndent(out, "", "  ")
	return string(data)
}

func spJSONAll(units []spUnit) string {
	m := make(map[string]string, len(shuangpinx.All()))
	for _, s := range shuangpinx.All() {
		m[s.ID] = spConcat(units, s)
	}
	data, _ := json.MarshalIndent(m, "", "  ")
	return string(data)
}

func init() {
	rootCmd.AddCommand(newSPCommand(spDeps{}))
}
