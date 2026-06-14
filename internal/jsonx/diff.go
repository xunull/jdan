package jsonx

import (
	"fmt"
	"reflect"
	"sort"
)

// DiffOp 是 RFC 6902 中的操作类型。我们只产生 add/remove/replace 三种
// （没有 move/copy/test —— 语义 diff 不需要）。
type DiffOp string

const (
	OpAdd     DiffOp = "add"
	OpRemove  DiffOp = "remove"
	OpReplace DiffOp = "replace"
)

// DiffEntry 是一条 diff 记录。字段名跟 RFC 6902 兼容：
//
//	{"op": "add", "path": "/foo", "value": ...}
//	{"op": "remove", "path": "/foo"}
//	{"op": "replace", "path": "/foo", "value": ...}
//
// OldValue 用 "old" 标签输出，方便人类查看，但 RFC 6902 严格只要 value。
type DiffEntry struct {
	Op       DiffOp `json:"op"`
	Path     string `json:"path"`
	OldValue any    `json:"old,omitempty"`
	NewValue any    `json:"value,omitempty"`
}

// Diff 计算 a→b 的语义差异，返回 RFC 6902 兼容的 op 列表。
// path 用 JSON Pointer 形式（"/users/0/name"）。
func Diff(a, b any) []DiffEntry {
	var out []DiffEntry
	diffWalk(a, b, "", &out)
	return out
}

func diffWalk(a, b any, path string, out *[]DiffEntry) {
	if reflect.DeepEqual(a, b) {
		return
	}
	am, aIsMap := a.(map[string]any)
	bm, bIsMap := b.(map[string]any)
	if aIsMap && bIsMap {
		keys := make(map[string]bool)
		for k := range am {
			keys[k] = true
		}
		for k := range bm {
			keys[k] = true
		}
		sorted := make([]string, 0, len(keys))
		for k := range keys {
			sorted = append(sorted, k)
		}
		sort.Strings(sorted)
		for _, k := range sorted {
			av, ahas := am[k]
			bv, bhas := bm[k]
			child := path + "/" + pointerEscape(k)
			switch {
			case !ahas && bhas:
				*out = append(*out, DiffEntry{Op: OpAdd, Path: child, NewValue: bv})
			case ahas && !bhas:
				*out = append(*out, DiffEntry{Op: OpRemove, Path: child, OldValue: av})
			default:
				diffWalk(av, bv, child, out)
			}
		}
		return
	}
	aa, aIsArr := a.([]any)
	ba, bIsArr := b.([]any)
	if aIsArr && bIsArr {
		n := max(len(aa), len(ba))
		for i := range n {
			child := fmt.Sprintf("%s/%d", path, i)
			switch {
			case i >= len(aa):
				*out = append(*out, DiffEntry{Op: OpAdd, Path: child, NewValue: ba[i]})
			case i >= len(ba):
				*out = append(*out, DiffEntry{Op: OpRemove, Path: child, OldValue: aa[i]})
			default:
				diffWalk(aa[i], ba[i], child, out)
			}
		}
		return
	}
	// 标量替换 or 类型不一致
	*out = append(*out, DiffEntry{Op: OpReplace, Path: path, OldValue: a, NewValue: b})
}
