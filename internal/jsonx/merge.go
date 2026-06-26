package jsonx

import (
	"fmt"
	"maps"
)

// ArrayStrategy 决定深合并时数组怎么处理。
type ArrayStrategy int

const (
	ArrayReplace ArrayStrategy = iota // 后者整段替换前者（默认）
	ArrayAppend                       // 两段拼接
)

// ParseArrayStrategy 把 --arrays 的值解析成策略。
func ParseArrayStrategy(s string) (ArrayStrategy, error) {
	switch s {
	case "replace", "":
		return ArrayReplace, nil
	case "append":
		return ArrayAppend, nil
	default:
		return 0, fmt.Errorf("非法 --arrays %q（可选 replace / append）", s)
	}
}

// Merge 深度合并 a 和 b，b 覆盖 a：
//   - 两边都是对象 → 递归合并（键取并集）
//   - 两边都是数组且 strat=append → 拼接；否则 b 替换
//   - 其余（标量 / 类型不一致 / null）→ b 覆盖
func Merge(a, b any, strat ArrayStrategy) any {
	am, aok := a.(map[string]any)
	bm, bok := b.(map[string]any)
	if aok && bok {
		out := make(map[string]any, len(am)+len(bm))
		maps.Copy(out, am)
		for k, bv := range bm {
			if av, exists := out[k]; exists {
				out[k] = Merge(av, bv, strat)
			} else {
				out[k] = bv
			}
		}
		return out
	}
	if strat == ArrayAppend {
		aa, aok2 := a.([]any)
		ba, bok2 := b.([]any)
		if aok2 && bok2 {
			out := make([]any, 0, len(aa)+len(ba))
			out = append(out, aa...)
			out = append(out, ba...)
			return out
		}
	}
	return b // 标量 / 类型不一致 / null / 数组 replace → 后者覆盖
}

// MergeAll 把多个 JSON 文档从左到右依次深合并，返回编码结果（indent>0 缩进）。
func MergeAll(docs [][]byte, strat ArrayStrategy, indent int) ([]byte, error) {
	if len(docs) == 0 {
		return nil, fmt.Errorf("没有输入")
	}
	var acc any
	for i, d := range docs {
		v, err := decodeJSON(d)
		if err != nil {
			return nil, fmt.Errorf("第 %d 个输入: %w", i+1, err)
		}
		if i == 0 {
			acc = v
		} else {
			acc = Merge(acc, v, strat)
		}
	}
	return marshalJSON(acc, indent)
}
