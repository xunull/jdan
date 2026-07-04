// Package shuangpinx 把全拼音节编码成【标准 26 键双拼】的两码字母，支持多套方案。
//
// 每套方案就是一串有序的「正则替换」规则，把拼音（声母+韵母）逐步换成键位字母。
// 规则全部逐条照 RIME rime-double-pinyin 的各 schema 抄（xform 规则 + 小鹤/自然码
// 的零声母倍写规则），不凭记忆。纯 stdlib、0 依赖。
//
// 例（小鹤）：中 zhong → 声母 zh=v、韵母 ong=s → "vs"。
package shuangpinx

import (
	"regexp"
	"strings"
)

type rule struct {
	re   *regexp.Regexp
	repl string
}

// Scheme 是一套双拼方案。
type Scheme struct {
	ID    string // 英文 id，如 "flypy"
	Name  string // 中文名，如 "小鹤"
	rules []rule
}

func compile(pairs [][2]string) []rule {
	rs := make([]rule, len(pairs))
	for i, p := range pairs {
		rs[i] = rule{regexp.MustCompile(p[0]), p[1]}
	}
	return rs
}

// Encode 把一个全拼音节编码成双拼两码字母（小写）。非法/空输入原样返回。
func (s Scheme) Encode(py string) string {
	py = strings.ToLower(strings.TrimSpace(py))
	py = strings.ReplaceAll(py, "ü", "v")
	if py == "" {
		return ""
	}
	for _, r := range s.rules {
		py = r.re.ReplaceAllString(py, r.repl)
	}
	return strings.ToLower(py)
}

// Valid 报告一个音节编码后是否是合法双拼码（恒 2 键）。
func (s Scheme) Valid(py string) (string, bool) {
	code := s.Encode(py)
	return code, len([]rune(code)) == 2
}

// schemes 按展示顺序排列；第一个是默认（小鹤）。
var schemes = []Scheme{
	{ID: "flypy", Name: "小鹤", rules: compile([][2]string{
		{`^([aoe])([ioun])$`, `${1}${1}${2}`}, {`^([aoe])(ng)?$`, `${1}${1}${2}`},
		{`iu$`, `Q`}, {`(.)ei$`, `${1}W`}, {`uan$`, `R`}, {`[uv]e$`, `T`}, {`un$`, `Y`},
		{`^sh`, `U`}, {`^ch`, `I`}, {`^zh`, `V`}, {`uo$`, `O`}, {`ie$`, `P`},
		{`i?ong$`, `S`}, {`ing$|uai$`, `K`}, {`(.)ai$`, `${1}D`}, {`(.)en$`, `${1}F`},
		{`(.)eng$`, `${1}G`}, {`[iu]ang$`, `L`}, {`(.)ang$`, `${1}H`}, {`ian$`, `M`},
		{`(.)an$`, `${1}J`}, {`(.)ou$`, `${1}Z`}, {`[iu]a$`, `X`}, {`iao$`, `N`},
		{`(.)ao$`, `${1}C`}, {`ui$`, `V`}, {`in$`, `B`},
	})},
	{ID: "ziranma", Name: "自然码", rules: compile([][2]string{
		{`^([aoe])([ioun])$`, `${1}${1}${2}`}, {`^([aoe])(ng)?$`, `${1}${1}${2}`},
		{`iu$`, `Q`}, {`[iu]a$`, `W`}, {`[uv]an$`, `R`}, {`[uv]e$`, `T`}, {`ing$|uai$`, `Y`},
		{`^sh`, `U`}, {`^ch`, `I`}, {`^zh`, `V`}, {`uo$`, `O`}, {`[uv]n$`, `P`},
		{`i?ong$`, `S`}, {`[iu]ang$`, `D`}, {`(.)en$`, `${1}F`}, {`(.)eng$`, `${1}G`},
		{`(.)ang$`, `${1}H`}, {`ian$`, `M`}, {`(.)an$`, `${1}J`}, {`iao$`, `C`},
		{`(.)ao$`, `${1}K`}, {`(.)ai$`, `${1}L`}, {`(.)ei$`, `${1}Z`}, {`ie$`, `X`},
		{`ui$`, `V`}, {`(.)ou$`, `${1}B`}, {`in$`, `N`},
	})},
	{ID: "mspy", Name: "微软", rules: compile([][2]string{
		{`^([ae])(.*)$`, `${1}${1}${2}`},
		{`iu$`, `Q`}, {`[iu]a$`, `W`}, {`er$|[uv]an$`, `R`}, {`[uv]e$`, `T`}, {`v$|uai$`, `Y`},
		{`^sh`, `U`}, {`^ch`, `I`}, {`^zh`, `V`}, {`uo$`, `O`}, {`[uv]n$`, `P`},
		{`i?ong$`, `S`}, {`[iu]ang$`, `D`}, {`(.)en$`, `${1}F`}, {`(.)eng$`, `${1}G`},
		{`(.)ang$`, `${1}H`}, {`ian$`, `M`}, {`(.)an$`, `${1}J`}, {`iao$`, `C`},
		{`(.)ao$`, `${1}K`}, {`(.)ai$`, `${1}L`}, {`(.)ei$`, `${1}Z`}, {`ie$`, `X`},
		{`ui$`, `V`}, {`(.)ou$`, `${1}B`}, {`in$`, `N`}, {`ing$`, `;`},
	})},
	{ID: "abc", Name: "智能ABC", rules: compile([][2]string{
		{`^zh`, `A`}, {`^ch`, `E`}, {`^sh`, `V`}, {`^([aoe].*)$`, `O${1}`},
		{`ei$`, `Q`}, {`ian$`, `W`}, {`er$|iu$`, `R`}, {`[iu]ang$`, `T`}, {`ing$`, `Y`},
		{`uo$`, `O`}, {`uan$`, `P`}, {`i?ong$`, `S`}, {`[iu]a$`, `D`}, {`en$`, `F`},
		{`eng$`, `G`}, {`ang$`, `H`}, {`an$`, `J`}, {`iao$`, `Z`}, {`ao$`, `K`},
		{`in$|uai$`, `C`}, {`ai$`, `L`}, {`ie$`, `X`}, {`ou$`, `B`}, {`un$`, `N`},
		{`[uv]e$|ui$`, `M`},
	})},
	{ID: "pyjj", Name: "拼音加加", rules: compile([][2]string{
		{`^([ae])(.*)$`, `${1}${1}${2}`},
		{`iu$`, `N`}, {`[iu]a$`, `B`}, {`er$|ing$`, `Q`}, {`[uv]an$`, `C`}, {`[uv]e$|uai$`, `X`},
		{`^sh`, `I`}, {`^ch`, `U`}, {`^zh`, `V`}, {`uo$`, `O`}, {`[uv]n$`, `Z`},
		{`i?ong$`, `Y`}, {`[iu]ang$`, `H`}, {`(.)en$`, `${1}R`}, {`(.)eng$`, `${1}T`},
		{`(.)ang$`, `${1}G`}, {`ian$`, `J`}, {`(.)an$`, `${1}F`}, {`iao$`, `K`},
		{`(.)ao$`, `${1}D`}, {`(.)ai$`, `${1}S`}, {`(.)ei$`, `${1}W`}, {`ie$`, `M`},
		{`ui$`, `V`}, {`(.)ou$`, `${1}P`}, {`in$`, `L`},
	})},
}

// aliases 把常见别名/搜狗 归到已有方案（搜狗双拼 = 微软布局）。
var aliases = map[string]string{"sogou": "mspy", "搜狗": "mspy", "ms": "mspy", "自然": "ziranma", "加加": "pyjj"}

var byKey = buildIndex()

func buildIndex() map[string]Scheme {
	m := map[string]Scheme{}
	for _, s := range schemes {
		m[s.ID] = s
		m[s.Name] = s
	}
	return m
}

// Get 按 id / 中文名 / 别名 取方案。
func Get(name string) (Scheme, bool) {
	name = strings.TrimSpace(name)
	if s, ok := byKey[name]; ok {
		return s, true
	}
	if a, ok := aliases[name]; ok {
		return byKey[a], true
	}
	return Scheme{}, false
}

// All 返回所有方案（展示顺序）。
func All() []Scheme { return schemes }

// Default 返回默认方案（小鹤）。
func Default() Scheme { return schemes[0] }

// Flypy 返回小鹤方案（供 spt9 复用）。
func Flypy() Scheme { return schemes[0] }

// IDs 返回所有方案 id（供帮助/校验）。
func IDs() []string {
	out := make([]string, len(schemes))
	for i, s := range schemes {
		out[i] = s.ID
	}
	return out
}
