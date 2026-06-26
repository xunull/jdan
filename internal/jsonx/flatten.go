package jsonx

import (
	"bytes"
	"encoding/json"
	"fmt"
	"strconv"
	"strings"
)

// Flatten 把嵌套 JSON 值压成扁平的点分键对象：对象键用 sep 连接，数组用 [i] 下标。
// 空对象 / 空数组当叶子保留（{"a":{}} → {"a":{}}），保证可被 Unflatten 还原。
func Flatten(node any, sep string) map[string]any {
	out := map[string]any{}
	flattenRec(out, "", node, sep)
	return out
}

func flattenRec(out map[string]any, prefix string, v any, sep string) {
	switch t := v.(type) {
	case map[string]any:
		if len(t) == 0 {
			out[prefix] = t // 空对象当叶子，保住可还原
			return
		}
		for k, vv := range t {
			key := k
			if prefix != "" {
				key = prefix + sep + k
			}
			flattenRec(out, key, vv, sep)
		}
	case []any:
		if len(t) == 0 {
			out[prefix] = t // 空数组当叶子
			return
		}
		for i, vv := range t {
			flattenRec(out, fmt.Sprintf("%s[%d]", prefix, i), vv, sep)
		}
	default:
		out[prefix] = v
	}
}

// Unflatten 把扁平的点分键对象还原成嵌套 JSON 值。键里同一前缀既当对象又当数组会报冲突。
func Unflatten(flat map[string]any, sep string) (any, error) {
	var root any
	for key, val := range flat {
		segs, err := splitFlatKey(key, sep)
		if err != nil {
			return nil, fmt.Errorf("键 %q: %w", key, err)
		}
		root, err = setPath(root, segs, val)
		if err != nil {
			return nil, fmt.Errorf("键 %q: %w", key, err)
		}
	}
	return root, nil
}

type flatSeg struct {
	key   string
	idx   int
	isIdx bool
}

// splitFlatKey 把 "a.c[0][1]" 拆成段。空键返回空段（表示根本身，对应顶层标量/空容器）。
func splitFlatKey(key, sep string) ([]flatSeg, error) {
	if key == "" {
		return nil, nil
	}
	var segs []flatSeg
	for part := range strings.SplitSeq(key, sep) {
		ps, err := parseFlatPart(part)
		if err != nil {
			return nil, err
		}
		segs = append(segs, ps...)
	}
	return segs, nil
}

// parseFlatPart 解析一个被 sep 分隔的段：name?[i][j]...
func parseFlatPart(part string) ([]flatSeg, error) {
	i := strings.IndexByte(part, '[')
	if i < 0 {
		return []flatSeg{{key: part}}, nil
	}
	var segs []flatSeg
	if i > 0 {
		segs = append(segs, flatSeg{key: part[:i]})
	}
	rest := part[i:]
	for len(rest) > 0 {
		if rest[0] != '[' {
			return nil, fmt.Errorf("非法段 %q", part)
		}
		c := strings.IndexByte(rest, ']')
		if c < 0 {
			return nil, fmt.Errorf("下标缺少 ]: %q", part)
		}
		n, err := strconv.Atoi(rest[1:c])
		if err != nil || n < 0 {
			return nil, fmt.Errorf("非法数组下标 %q", rest[1:c])
		}
		segs = append(segs, flatSeg{idx: n, isIdx: true})
		rest = rest[c+1:]
	}
	return segs, nil
}

// setPath 沿 segs 把 val 写进 node，按需新建对象/数组，返回（可能新建的）容器。
func setPath(node any, segs []flatSeg, val any) (any, error) {
	if len(segs) == 0 {
		return val, nil
	}
	s := segs[0]
	if s.isIdx {
		arr, ok := toSlice(node)
		if !ok {
			return nil, fmt.Errorf("路径冲突：此处既要当数组又已是对象/标量")
		}
		for len(arr) <= s.idx {
			arr = append(arr, nil) // 稀疏下标补 null
		}
		child, err := setPath(arr[s.idx], segs[1:], val)
		if err != nil {
			return nil, err
		}
		arr[s.idx] = child
		return arr, nil
	}
	m, ok := toMap(node)
	if !ok {
		return nil, fmt.Errorf("路径冲突：此处既要当对象又已是数组/标量")
	}
	child, err := setPath(m[s.key], segs[1:], val)
	if err != nil {
		return nil, err
	}
	m[s.key] = child
	return m, nil
}

func toSlice(node any) ([]any, bool) {
	if node == nil {
		return []any{}, true
	}
	a, ok := node.([]any)
	return a, ok
}

func toMap(node any) (map[string]any, bool) {
	if node == nil {
		return map[string]any{}, true
	}
	m, ok := node.(map[string]any)
	return m, ok
}

// FlattenBytes 解码 JSON、扁平化、再编码。indent>0 缩进输出，否则 compact。
func FlattenBytes(data []byte, sep string, indent int) ([]byte, error) {
	v, err := decodeJSON(data)
	if err != nil {
		return nil, err
	}
	return marshalJSON(Flatten(v, sep), indent)
}

// UnflattenBytes 解码扁平对象、还原、再编码。输入必须是 JSON 对象。
func UnflattenBytes(data []byte, sep string, indent int) ([]byte, error) {
	v, err := decodeJSON(data)
	if err != nil {
		return nil, err
	}
	m, ok := v.(map[string]any)
	if !ok {
		return nil, fmt.Errorf("unflatten 输入必须是 JSON 对象")
	}
	out, err := Unflatten(m, sep)
	if err != nil {
		return nil, err
	}
	return marshalJSON(out, indent)
}

func decodeJSON(data []byte) (any, error) {
	var v any
	dec := json.NewDecoder(bytes.NewReader(data))
	dec.UseNumber() // 保数字精度，跟其他 json 子命令一致
	if err := dec.Decode(&v); err != nil {
		return nil, err
	}
	return v, nil
}

func marshalJSON(v any, indent int) ([]byte, error) {
	if indent > 0 {
		return json.MarshalIndent(v, "", strings.Repeat(" ", indent))
	}
	return json.Marshal(v)
}
