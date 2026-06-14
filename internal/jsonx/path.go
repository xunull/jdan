package jsonx

import (
	"encoding/json"
	"errors"
	"fmt"
	"strconv"
	"strings"
)

// SegKind 区分一个 path 段是按 key 取还是按 index 取。
type SegKind int

const (
	SegKey SegKind = iota
	SegIndex
)

// Segment 是 path 中的一段。
type Segment struct {
	Kind  SegKind
	Key   string // 仅 SegKey 时有意义
	Index int    // 仅 SegIndex 时有意义
}

func (s Segment) String() string {
	if s.Kind == SegIndex {
		return fmt.Sprintf("[%d]", s.Index)
	}
	return s.Key
}

// ParsePath 解析 dot-path + bracket 形式：
//
//	users.0.name           → key=users, key=0,    key=name
//	users[0].name          → key=users, idx=0,    key=name
//	servers[0].ports[2]    → key=servers, idx=0, key=ports, idx=2
//	foo\.bar               → key=foo.bar  （backslash 转义 dot）
//	空字符串               → 空 slice（指根）
//
// dot-path 模式下数组索引段也可写成 key（"users.0.name"），Get 会在 runtime
// 自动把 string-shaped int 当 index 用。这样用户不需要记 bracket 语法。
func ParsePath(p string) ([]Segment, error) {
	if p == "" {
		return nil, nil
	}
	var segs []Segment
	var cur strings.Builder
	flushKey := func() {
		if cur.Len() > 0 {
			segs = append(segs, Segment{Kind: SegKey, Key: cur.String()})
			cur.Reset()
		}
	}
	i := 0
	for i < len(p) {
		c := p[i]
		switch c {
		case '\\':
			if i+1 >= len(p) {
				return nil, errors.New("trailing backslash in path")
			}
			cur.WriteByte(p[i+1])
			i += 2
		case '.':
			flushKey()
			i++
		case '[':
			flushKey()
			end := strings.IndexByte(p[i+1:], ']')
			if end < 0 {
				return nil, fmt.Errorf("unclosed '[' at position %d", i)
			}
			tok := p[i+1 : i+1+end]
			idx, err := strconv.Atoi(tok)
			if err != nil {
				return nil, fmt.Errorf("non-integer index %q inside brackets", tok)
			}
			segs = append(segs, Segment{Kind: SegIndex, Index: idx})
			i = i + 1 + end + 1
		case ']':
			return nil, fmt.Errorf("unexpected ']' at position %d", i)
		default:
			cur.WriteByte(c)
			i++
		}
	}
	flushKey()
	return segs, nil
}

// Get 按 path 段从 v 中取值。
//   - 在 map 上 Index 段直接报错
//   - 在 array 上 Key 段尝试当作整数索引（让 "users.0.name" 也 work）
//   - 负 Index 从末尾倒数
func Get(v any, segs []Segment) (any, error) {
	cur := v
	for i, s := range segs {
		switch node := cur.(type) {
		case map[string]any:
			if s.Kind == SegIndex {
				return nil, fmt.Errorf("path segment %d: object expects key, got index [%d]", i, s.Index)
			}
			next, ok := node[s.Key]
			if !ok {
				return nil, fmt.Errorf("path segment %d: key %q not found", i, s.Key)
			}
			cur = next
		case []any:
			idx, err := indexFromSeg(s, len(node))
			if err != nil {
				return nil, fmt.Errorf("path segment %d: %w", i, err)
			}
			cur = node[idx]
		case nil:
			return nil, fmt.Errorf("path segment %d: cannot descend into null", i)
		default:
			return nil, fmt.Errorf("path segment %d: cannot descend into %T", i, cur)
		}
	}
	return cur, nil
}

func indexFromSeg(s Segment, length int) (int, error) {
	var idx int
	switch s.Kind {
	case SegIndex:
		idx = s.Index
	case SegKey:
		n, err := strconv.Atoi(s.Key)
		if err != nil {
			return 0, fmt.Errorf("array index must be int, got %q", s.Key)
		}
		idx = n
	}
	if idx < 0 {
		idx = length + idx
	}
	if idx < 0 || idx >= length {
		return 0, fmt.Errorf("index %d out of range [0,%d)", idx, length)
	}
	return idx, nil
}

// DecodeValue parses JSON into any with UseNumber so numbers don't lose precision.
func DecodeValue(data []byte) (any, error) {
	var v any
	dec := json.NewDecoder(strings.NewReader(string(data)))
	dec.UseNumber()
	if err := dec.Decode(&v); err != nil {
		return nil, err
	}
	return v, nil
}
