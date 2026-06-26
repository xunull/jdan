package jsonx

import (
	"strings"
	"testing"
)

// mergeStr 解码两段 JSON、合并、再规范化成 compact JSON，便于断言。
func mergeStr(t *testing.T, a, b string, strat ArrayStrategy) string {
	t.Helper()
	got := Merge(decode(t, a), decode(t, b), strat)
	return canon(t, got)
}

// ---- Merge ----

func TestMerge_ObjectUnion(t *testing.T) {
	if got := mergeStr(t, `{"a":1}`, `{"b":2}`, ArrayReplace); got != `{"a":1,"b":2}` {
		t.Errorf("got %s", got)
	}
}

func TestMerge_RecursiveNested(t *testing.T) {
	// nest 应递归合并，而不是整体替换
	got := mergeStr(t, `{"nest":{"x":1}}`, `{"nest":{"y":2}}`, ArrayReplace)
	if got != `{"nest":{"x":1,"y":2}}` {
		t.Errorf("nested merge wrong: %s", got)
	}
}

func TestMerge_ScalarOverride(t *testing.T) {
	if got := mergeStr(t, `{"a":1}`, `{"a":9}`, ArrayReplace); got != `{"a":9}` {
		t.Errorf("scalar override wrong: %s", got)
	}
}

func TestMerge_TypeMismatchBWins(t *testing.T) {
	// 对象被标量替换
	if got := mergeStr(t, `{"a":{"x":1}}`, `{"a":5}`, ArrayReplace); got != `{"a":5}` {
		t.Errorf("type mismatch should let b win: %s", got)
	}
	// 标量被对象替换
	if got := mergeStr(t, `{"a":5}`, `{"a":{"x":1}}`, ArrayReplace); got != `{"a":{"x":1}}` {
		t.Errorf("type mismatch should let b win: %s", got)
	}
}

func TestMerge_ArrayReplace(t *testing.T) {
	if got := mergeStr(t, `{"l":[1,2]}`, `{"l":[3,4]}`, ArrayReplace); got != `{"l":[3,4]}` {
		t.Errorf("array replace wrong: %s", got)
	}
}

func TestMerge_ArrayAppend(t *testing.T) {
	if got := mergeStr(t, `{"l":[1,2]}`, `{"l":[3,4]}`, ArrayAppend); got != `{"l":[1,2,3,4]}` {
		t.Errorf("array append wrong: %s", got)
	}
}

func TestMerge_TopLevelArrayAppend(t *testing.T) {
	if got := mergeStr(t, `[1,2]`, `[3]`, ArrayAppend); got != `[1,2,3]` {
		t.Errorf("top-level array append wrong: %s", got)
	}
}

func TestMerge_NullOverrides(t *testing.T) {
	if got := mergeStr(t, `{"a":1}`, `{"a":null}`, ArrayReplace); got != `{"a":null}` {
		t.Errorf("null should override: %s", got)
	}
}

func TestMerge_BOnlyKeyAdded_AOnlyKept(t *testing.T) {
	got := mergeStr(t, `{"a":1,"only_a":true}`, `{"b":2,"only_b":true}`, ArrayReplace)
	for _, want := range []string{`"a":1`, `"b":2`, `"only_a":true`, `"only_b":true`} {
		if !strings.Contains(got, want) {
			t.Errorf("missing %s in %s", want, got)
		}
	}
}

func TestMerge_DeepNested(t *testing.T) {
	got := mergeStr(t, `{"a":{"b":{"c":1}}}`, `{"a":{"b":{"d":2}}}`, ArrayReplace)
	if got != `{"a":{"b":{"c":1,"d":2}}}` {
		t.Errorf("deep nested merge wrong: %s", got)
	}
}

func TestMerge_DoesNotMutateInputs(t *testing.T) {
	a := decode(t, `{"nest":{"x":1}}`)
	b := decode(t, `{"nest":{"y":2}}`)
	_ = Merge(a, b, ArrayReplace)
	if canon(t, a) != `{"nest":{"x":1}}` {
		t.Errorf("Merge mutated a: %s", canon(t, a))
	}
	if canon(t, b) != `{"nest":{"y":2}}` {
		t.Errorf("Merge mutated b: %s", canon(t, b))
	}
}

// ---- MergeAll ----

func TestMergeAll_LeftToRight(t *testing.T) {
	docs := [][]byte{[]byte(`{"k":1,"a":1}`), []byte(`{"k":2}`), []byte(`{"k":3}`)}
	out, err := MergeAll(docs, ArrayReplace, 0)
	if err != nil {
		t.Fatal(err)
	}
	if string(out) != `{"a":1,"k":3}` {
		t.Errorf("left-to-right wrong: %s", out)
	}
}

func TestMergeAll_BigIntPrecision(t *testing.T) {
	docs := [][]byte{[]byte(`{"n":12345678901234567890}`), []byte(`{"k":1}`)}
	out, _ := MergeAll(docs, ArrayReplace, 0)
	if !strings.Contains(string(out), "12345678901234567890") {
		t.Errorf("big int precision lost: %s", out)
	}
}

func TestMergeAll_InvalidNthDoc(t *testing.T) {
	docs := [][]byte{[]byte(`{"a":1}`), []byte(`{bad`)}
	_, err := MergeAll(docs, ArrayReplace, 0)
	if err == nil {
		t.Error("invalid nth doc should error")
	}
	if !strings.Contains(err.Error(), "2") {
		t.Errorf("error should name the offending input index: %v", err)
	}
}

// ---- ParseArrayStrategy ----

func TestParseArrayStrategy(t *testing.T) {
	if s, _ := ParseArrayStrategy("replace"); s != ArrayReplace {
		t.Error("replace")
	}
	if s, _ := ParseArrayStrategy(""); s != ArrayReplace {
		t.Error("empty → replace")
	}
	if s, _ := ParseArrayStrategy("append"); s != ArrayAppend {
		t.Error("append")
	}
	if _, err := ParseArrayStrategy("union"); err == nil {
		t.Error("unknown strategy should error")
	}
}
