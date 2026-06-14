package jsonx

import (
	"encoding/json"
	"reflect"
	"strings"
	"testing"
)

// ---- Pretty / Minify ----

func TestPretty_BasicIndent(t *testing.T) {
	in := []byte(`{"b":2,"a":1}`)
	out, err := Pretty(in, 2)
	if err != nil {
		t.Fatal(err)
	}
	want := "{\n  \"a\": 1,\n  \"b\": 2\n}"
	if string(out) != want {
		t.Errorf("Pretty = %q, want %q", out, want)
	}
}

func TestPretty_DefaultIndentWhenZero(t *testing.T) {
	out, err := Pretty([]byte(`{"a":1}`), 0)
	if err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(string(out), "  \"a\"") {
		t.Errorf("indent=0 should default to 2 spaces, got: %s", out)
	}
}

func TestPretty_PreservesIntPrecision(t *testing.T) {
	// 大整数应该保留：UseNumber 防止 float64 精度损失
	in := []byte(`{"id":9007199254740993}`) // 2^53 + 1
	out, err := Pretty(in, 2)
	if err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(string(out), "9007199254740993") {
		t.Errorf("pretty lost precision: %s", out)
	}
}

func TestPretty_InvalidJSON_Errors(t *testing.T) {
	if _, err := Pretty([]byte(`{not json`), 2); err == nil {
		t.Error("invalid JSON should error")
	}
}

func TestMinify_Basic(t *testing.T) {
	out, err := Minify([]byte(`{
  "a": 1,
  "b": [1, 2, 3]
}`))
	if err != nil {
		t.Fatal(err)
	}
	want := `{"a":1,"b":[1,2,3]}`
	if string(out) != want {
		t.Errorf("Minify = %q, want %q", out, want)
	}
}

func TestMinify_PreservesNumberPrecision(t *testing.T) {
	// Compact 走 byte 级，不会经过 float64
	in := []byte(`{"big": 9007199254740993}`)
	out, err := Minify(in)
	if err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(string(out), "9007199254740993") {
		t.Errorf("Minify lost precision: %s", out)
	}
}

// ---- ParsePath ----

func TestParsePath_DotPath(t *testing.T) {
	segs, err := ParsePath("users.0.name")
	if err != nil {
		t.Fatal(err)
	}
	if len(segs) != 3 {
		t.Fatalf("want 3 segs, got %d", len(segs))
	}
	// dot-path: 全是 SegKey（"0" 也是 Key，Get 时按 array 自动转 index）
	for i, s := range segs {
		if s.Kind != SegKey {
			t.Errorf("seg %d should be Key, got %v", i, s.Kind)
		}
	}
}

func TestParsePath_BracketIndex(t *testing.T) {
	segs, err := ParsePath("users[0].name")
	if err != nil {
		t.Fatal(err)
	}
	if len(segs) != 3 {
		t.Fatalf("want 3 segs, got %d", len(segs))
	}
	if segs[1].Kind != SegIndex || segs[1].Index != 0 {
		t.Errorf("middle seg should be Index(0), got %+v", segs[1])
	}
}

func TestParsePath_EscapedDot(t *testing.T) {
	segs, err := ParsePath(`foo\.bar.baz`)
	if err != nil {
		t.Fatal(err)
	}
	if len(segs) != 2 {
		t.Fatalf("want 2 segs, got %d: %+v", len(segs), segs)
	}
	if segs[0].Key != "foo.bar" {
		t.Errorf("seg 0 = %q, want foo.bar", segs[0].Key)
	}
}

func TestParsePath_EmptyReturnsNil(t *testing.T) {
	segs, err := ParsePath("")
	if err != nil {
		t.Fatal(err)
	}
	if segs != nil {
		t.Errorf("empty path should be nil, got %+v", segs)
	}
}

func TestParsePath_UnclosedBracket_Errors(t *testing.T) {
	if _, err := ParsePath("users[0"); err == nil {
		t.Error("unclosed [ should error")
	}
}

func TestParsePath_NonIntBracket_Errors(t *testing.T) {
	if _, err := ParsePath("users[abc]"); err == nil {
		t.Error("non-int inside [] should error")
	}
}

// ---- Get ----

func TestGet_DotPathThroughArray(t *testing.T) {
	v, _ := DecodeValue([]byte(`{"users":[{"name":"alice"},{"name":"bob"}]}`))
	segs, _ := ParsePath("users.1.name")
	got, err := Get(v, segs)
	if err != nil {
		t.Fatal(err)
	}
	if got != "bob" {
		t.Errorf("got %v, want bob", got)
	}
}

func TestGet_NegativeIndex(t *testing.T) {
	v, _ := DecodeValue([]byte(`{"xs":[10,20,30]}`))
	segs, _ := ParsePath("xs[-1]")
	got, err := Get(v, segs)
	if err != nil {
		t.Fatal(err)
	}
	gotStr := got.(json.Number).String()
	if gotStr != "30" {
		t.Errorf("got %v, want 30", gotStr)
	}
}

func TestGet_OutOfRange_Errors(t *testing.T) {
	v, _ := DecodeValue([]byte(`{"xs":[1,2]}`))
	segs, _ := ParsePath("xs[5]")
	if _, err := Get(v, segs); err == nil {
		t.Error("out-of-range should error")
	}
}

func TestGet_MissingKey_Errors(t *testing.T) {
	v, _ := DecodeValue([]byte(`{"a":1}`))
	segs, _ := ParsePath("nope")
	if _, err := Get(v, segs); err == nil {
		t.Error("missing key should error")
	}
}

// ---- ParsePointer (RFC 6901) ----

func TestParsePointer_RFC6901Examples(t *testing.T) {
	cases := []struct {
		ptr  string
		want []string
	}{
		{"", nil},
		{"/", []string{""}},
		{"/foo", []string{"foo"}},
		{"/foo/0", []string{"foo", "0"}},
		{"/a~1b", []string{"a/b"}}, // ~1 → /
		{"/m~0n", []string{"m~n"}}, // ~0 → ~
		{"/~01", []string{"~1"}},   // 先 ~1 后 ~0 的顺序：~01 → ~1
	}
	for _, c := range cases {
		segs, err := ParsePointer(c.ptr)
		if err != nil {
			t.Errorf("ParsePointer(%q) errored: %v", c.ptr, err)
			continue
		}
		if c.want == nil {
			if segs != nil {
				t.Errorf("ParsePointer(%q) = %+v, want nil", c.ptr, segs)
			}
			continue
		}
		got := make([]string, len(segs))
		for i, s := range segs {
			got[i] = s.Key
		}
		if !reflect.DeepEqual(got, c.want) {
			t.Errorf("ParsePointer(%q) = %v, want %v", c.ptr, got, c.want)
		}
	}
}

func TestParsePointer_MissingLeadingSlash_Errors(t *testing.T) {
	if _, err := ParsePointer("foo/bar"); err == nil {
		t.Error("missing leading / should error")
	}
}

// ---- Keys ----

func TestKeys_TopLevel(t *testing.T) {
	v, _ := DecodeValue([]byte(`{"b":1,"a":2,"c":3}`))
	ks, err := Keys(v, false, 0)
	if err != nil {
		t.Fatal(err)
	}
	want := []string{"a", "b", "c"}
	if !reflect.DeepEqual(ks, want) {
		t.Errorf("got %v, want %v", ks, want)
	}
}

func TestKeys_TopLevel_NotObject_Errors(t *testing.T) {
	v, _ := DecodeValue([]byte(`[1,2,3]`))
	if _, err := Keys(v, false, 0); err == nil {
		t.Error("top-level array should error in non-all mode")
	}
}

func TestKeys_AllPaths(t *testing.T) {
	v, _ := DecodeValue([]byte(`{"a":1,"b":{"c":2,"d":[10,20]}}`))
	ks, _ := Keys(v, true, 0)
	want := []string{"a", "b.c", "b.d[0]", "b.d[1]"}
	if !reflect.DeepEqual(ks, want) {
		t.Errorf("got %v, want %v", ks, want)
	}
}

func TestKeys_AllPathsWithDepth(t *testing.T) {
	v, _ := DecodeValue([]byte(`{"a":1,"b":{"c":2,"d":{"e":3}}}`))
	ks, _ := Keys(v, true, 2)
	// depth=2 截断到 b.c 和 b.d（不下钻到 b.d.e）
	want := []string{"a", "b.c", "b.d"}
	if !reflect.DeepEqual(ks, want) {
		t.Errorf("got %v, want %v", ks, want)
	}
}

func TestKeys_AllPathsEscapesDotsInKeys(t *testing.T) {
	v, _ := DecodeValue([]byte(`{"a.b":1}`))
	ks, _ := Keys(v, true, 0)
	if len(ks) != 1 || ks[0] != `a\.b` {
		t.Errorf("got %v, want [a\\.b]", ks)
	}
}

// ---- Diff ----

func TestDiff_Identical(t *testing.T) {
	a, _ := DecodeValue([]byte(`{"a":1,"b":[1,2]}`))
	b, _ := DecodeValue([]byte(`{"a":1,"b":[1,2]}`))
	if d := Diff(a, b); len(d) != 0 {
		t.Errorf("identical should be empty diff, got %+v", d)
	}
}

func TestDiff_KeyAdded(t *testing.T) {
	a, _ := DecodeValue([]byte(`{"a":1}`))
	b, _ := DecodeValue([]byte(`{"a":1,"b":2}`))
	d := Diff(a, b)
	if len(d) != 1 || d[0].Op != OpAdd || d[0].Path != "/b" {
		t.Errorf("got %+v", d)
	}
}

func TestDiff_KeyRemoved(t *testing.T) {
	a, _ := DecodeValue([]byte(`{"a":1,"b":2}`))
	b, _ := DecodeValue([]byte(`{"a":1}`))
	d := Diff(a, b)
	if len(d) != 1 || d[0].Op != OpRemove || d[0].Path != "/b" {
		t.Errorf("got %+v", d)
	}
}

func TestDiff_ScalarReplace(t *testing.T) {
	a, _ := DecodeValue([]byte(`{"a":1}`))
	b, _ := DecodeValue([]byte(`{"a":2}`))
	d := Diff(a, b)
	if len(d) != 1 || d[0].Op != OpReplace {
		t.Errorf("got %+v", d)
	}
}

func TestDiff_ArrayLengthDiff(t *testing.T) {
	a, _ := DecodeValue([]byte(`{"xs":[1,2]}`))
	b, _ := DecodeValue([]byte(`{"xs":[1,2,3]}`))
	d := Diff(a, b)
	if len(d) != 1 || d[0].Op != OpAdd || d[0].Path != "/xs/2" {
		t.Errorf("got %+v", d)
	}
}

func TestDiff_PointerEscapesSlashInKey(t *testing.T) {
	a, _ := DecodeValue([]byte(`{"a/b":1}`))
	b, _ := DecodeValue([]byte(`{"a/b":2}`))
	d := Diff(a, b)
	if len(d) != 1 || d[0].Path != "/a~1b" {
		t.Errorf("path should be /a~1b, got %+v", d)
	}
}

// ---- Lines (JSONL) ----

func TestLinesCount_BasicAndBlanks(t *testing.T) {
	in := `{"a":1}
{"b":2}

{"c":3}
`
	n, err := LinesCount(strings.NewReader(in))
	if err != nil {
		t.Fatal(err)
	}
	if n != 3 {
		t.Errorf("got %d, want 3", n)
	}
}

func TestLinesCount_InvalidLineErrors(t *testing.T) {
	in := "{\"a\":1}\nnotjson\n"
	if _, err := LinesCount(strings.NewReader(in)); err == nil {
		t.Error("invalid line should error")
	}
}

func TestLinesGet_HappyPath(t *testing.T) {
	in := `{"a":1}
{"b":2}
{"c":3}
`
	line, err := LinesGet(strings.NewReader(in), 1)
	if err != nil {
		t.Fatal(err)
	}
	if string(line) != `{"b":2}` {
		t.Errorf("got %q", line)
	}
}

func TestLinesGet_OutOfRange_Errors(t *testing.T) {
	in := `{"a":1}` + "\n"
	if _, err := LinesGet(strings.NewReader(in), 5); err == nil {
		t.Error("out-of-range should error")
	}
}

func TestLinesHead_LimitsAndSkipsBlanks(t *testing.T) {
	in := `{"a":1}

{"b":2}
{"c":3}
{"d":4}
`
	out, err := LinesHead(strings.NewReader(in), 2)
	if err != nil {
		t.Fatal(err)
	}
	if len(out) != 2 {
		t.Errorf("got %d, want 2", len(out))
	}
	if string(out[0]) != `{"a":1}` || string(out[1]) != `{"b":2}` {
		t.Errorf("got %q, %q", out[0], out[1])
	}
}
