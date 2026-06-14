package jsonx

import (
	"errors"
	"strings"
)

// ParsePointer 解析 RFC 6901 JSON Pointer：
//
//	""             → root（空 slice）
//	"/"            → key=""
//	"/foo/0/bar"   → key=foo, key=0, key=bar
//	"~0" → "~", "~1" → "/" （RFC 6901 4.1：先 ~1 后 ~0）
//
// 所有段都返回 SegKey；Get 在 array 上会自动把 string-int 当 index。
func ParsePointer(p string) ([]Segment, error) {
	if p == "" {
		return nil, nil
	}
	if !strings.HasPrefix(p, "/") {
		return nil, errors.New("JSON Pointer must start with /")
	}
	parts := strings.Split(p[1:], "/")
	segs := make([]Segment, 0, len(parts))
	for _, part := range parts {
		// RFC 6901: first ~1 → /, then ~0 → ~ （顺序很重要）
		unescaped := strings.ReplaceAll(part, "~1", "/")
		unescaped = strings.ReplaceAll(unescaped, "~0", "~")
		segs = append(segs, Segment{Kind: SegKey, Key: unescaped})
	}
	return segs, nil
}

// pointerEscape 反向：把 key 编码成 JSON Pointer token。
// RFC 6901: 先 ~ → ~0 再 / → ~1（顺序很重要，否则会双重转义）。
func pointerEscape(s string) string {
	s = strings.ReplaceAll(s, "~", "~0")
	s = strings.ReplaceAll(s, "/", "~1")
	return s
}
