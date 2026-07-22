package tradx

import "strings"

// matcher 是可在 rs[i:] 处求最长命中的东西：单词典或词典组。
type matcher interface {
	matchPrefix(rs []rune, i int) (n int, val string, ok bool)
}

// group 是 OpenCC 的词典组，两种 policy 语义不同（见 src/DictGroup.cpp）：
//   - shortCircuit=true ：按序试成员，第一个有命中的成员即返回它自己的最长前缀，后面的不看。
//   - shortCircuit=false（union）：跨所有成员取最长命中，严格 > → 平手取前者。
//
// 成员本身也是 matcher，支持嵌套。
type group struct {
	shortCircuit bool
	members      []matcher
}

func (g *group) matchPrefix(rs []rune, i int) (int, string, bool) {
	if g.shortCircuit {
		for _, m := range g.members {
			if n, v, ok := m.matchPrefix(rs, i); ok {
				return n, v, true
			}
		}
		return 0, "", false
	}
	bn, bv, found := 0, "", false
	for _, m := range g.members {
		if n, v, ok := m.matchPrefix(rs, i); ok && (!found || n > bn) {
			bn, bv, found = n, v, true
		}
	}
	return bn, bv, found
}

// convertOnce 是一趟 conversion：对文本做前向最大匹配重写（对齐 OpenCC
// Conversion.cpp::AppendConverted）。命中输出 value、前进命中长度；否则原样输出 1 字、前进 1。
func convertOnce(rs []rune, m matcher) string {
	var b strings.Builder
	b.Grow(len(rs) * 3)
	for i := 0; i < len(rs); {
		if n, v, ok := m.matchPrefix(rs, i); ok {
			b.WriteString(v)
			i += n
		} else {
			b.WriteRune(rs[i])
			i++
		}
	}
	return b.String()
}

// Converter 持有一条已构建好的 conversion 链，可复用于多行输入。
type Converter struct {
	chain []matcher
}

// NewConverter 按 target（t/tw/twp/hk/s）构建转换器；触发词典懒加载。
func NewConverter(target string) (*Converter, error) {
	chain, err := buildChain(target)
	if err != nil {
		return nil, err
	}
	return &Converter{chain: chain}, nil
}

// Convert 按链顺序叠加各趟，后趟吃前趟输出。
func (c *Converter) Convert(text string) string {
	if text == "" {
		return ""
	}
	out := text
	for _, m := range c.chain {
		out = convertOnce([]rune(out), m)
	}
	return out
}
