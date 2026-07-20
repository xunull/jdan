package dotenv

import "sort"

// DiffResult 是两个 .env 的对比结果。
type DiffResult struct {
	OnlyInA   []string     `json:"only_in_a"`            // a 有 b 没有的 key
	OnlyInB   []string     `json:"only_in_b"`            // b 有 a 没有的 key
	Common    []string     `json:"common"`               // 两边都有的 key
	ValueDiff []ValueDelta `json:"value_diff,omitempty"` // 仅 withValues 时填充
}

// ValueDelta 是同一 key 在两文件中 value 不同的记录。
type ValueDelta struct {
	Key string `json:"key"`
	A   string `json:"a"`
	B   string `json:"b"`
}

// keyMap 把 File 折成 key→value（重复 key 取最后一个，跟 shell 加载语义一致）。
func keyMap(f *File) map[string]string {
	m := map[string]string{}
	for _, e := range f.Entries {
		if e.HasEquals && e.Key != "" {
			m[e.Key] = e.Value
		}
	}
	return m
}

// Diff 对比两个 .env 的 key 集合。withValues=true 时额外对比公共 key 的 value。
func Diff(a, b *File, withValues bool) DiffResult {
	ma, mb := keyMap(a), keyMap(b)
	var res DiffResult
	for k := range ma {
		if _, ok := mb[k]; ok {
			res.Common = append(res.Common, k)
			if withValues && ma[k] != mb[k] {
				res.ValueDiff = append(res.ValueDiff, ValueDelta{Key: k, A: ma[k], B: mb[k]})
			}
		} else {
			res.OnlyInA = append(res.OnlyInA, k)
		}
	}
	for k := range mb {
		if _, ok := ma[k]; !ok {
			res.OnlyInB = append(res.OnlyInB, k)
		}
	}
	sort.Strings(res.OnlyInA)
	sort.Strings(res.OnlyInB)
	sort.Strings(res.Common)
	sort.Slice(res.ValueDiff, func(i, j int) bool { return res.ValueDiff[i].Key < res.ValueDiff[j].Key })
	return res
}

// HasDifferences 判断是否有 key 差异（用于退出码）。
func (d DiffResult) HasDifferences() bool {
	return len(d.OnlyInA) > 0 || len(d.OnlyInB) > 0 || len(d.ValueDiff) > 0
}
