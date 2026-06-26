package jsonx

import (
	"encoding/json"
	"testing"
)

// decode 用 UseNumber 解码（跟生产一致），便于和 Flatten/Unflatten 比较。
func decode(t *testing.T, s string) any {
	t.Helper()
	v, err := decodeJSON([]byte(s))
	if err != nil {
		t.Fatal(err)
	}
	return v
}

// canon 把值编码成 compact JSON（map key 自动排序）做规范化比较。
func canon(t *testing.T, v any) string {
	t.Helper()
	b, err := json.Marshal(v)
	if err != nil {
		t.Fatal(err)
	}
	return string(b)
}

// ---- Flatten ----

func TestFlatten_Basic(t *testing.T) {
	got := Flatten(decode(t, `{"a":{"b":1,"c":[10,20]}}`), ".")
	want := map[string]string{"a.b": "1", "a.c[0]": "10", "a.c[1]": "20"}
	if len(got) != len(want) {
		t.Fatalf("got %d keys, want %d: %v", len(got), len(want), got)
	}
	for k, w := range want {
		if canon(t, got[k]) != w {
			t.Errorf("key %q = %v, want %s", k, got[k], w)
		}
	}
}

func TestFlatten_EmptyContainersPreserved(t *testing.T) {
	got := Flatten(decode(t, `{"a":{},"b":[]}`), ".")
	if _, ok := got["a"]; !ok {
		t.Error("empty object should be kept as leaf 'a'")
	}
	if _, ok := got["b"]; !ok {
		t.Error("empty array should be kept as leaf 'b'")
	}
}

func TestFlatten_TopLevelArray(t *testing.T) {
	got := Flatten(decode(t, `[1,2]`), ".")
	if canon(t, got["[0]"]) != "1" || canon(t, got["[1]"]) != "2" {
		t.Errorf("top-level array flatten wrong: %v", got)
	}
}

func TestFlatten_TopLevelScalar(t *testing.T) {
	got := Flatten(decode(t, `5`), ".")
	if canon(t, got[""]) != "5" {
		t.Errorf("top-level scalar should flatten to {\"\":5}, got %v", got)
	}
}

func TestFlatten_CustomSep(t *testing.T) {
	got := Flatten(decode(t, `{"a":{"b":1}}`), "/")
	if _, ok := got["a/b"]; !ok {
		t.Errorf("custom sep wrong: %v", got)
	}
}

func TestFlatten_BigIntPrecision(t *testing.T) {
	got := Flatten(decode(t, `{"n":12345678901234567890}`), ".")
	if canon(t, got["n"]) != "12345678901234567890" {
		t.Errorf("big int precision lost: %v", got["n"])
	}
}

// ---- Unflatten ----

func TestUnflatten_Object(t *testing.T) {
	flat := decode(t, `{"a.b":1,"a.c":2}`).(map[string]any)
	got, err := Unflatten(flat, ".")
	if err != nil {
		t.Fatal(err)
	}
	if canon(t, got) != `{"a":{"b":1,"c":2}}` {
		t.Errorf("got %s", canon(t, got))
	}
}

func TestUnflatten_Array(t *testing.T) {
	flat := decode(t, `{"a[0]":10,"a[1]":20}`).(map[string]any)
	got, _ := Unflatten(flat, ".")
	if canon(t, got) != `{"a":[10,20]}` {
		t.Errorf("got %s", canon(t, got))
	}
}

func TestUnflatten_SparseArray(t *testing.T) {
	flat := decode(t, `{"x[2]":9}`).(map[string]any)
	got, _ := Unflatten(flat, ".")
	if canon(t, got) != `{"x":[null,null,9]}` {
		t.Errorf("sparse array should fill null: %s", canon(t, got))
	}
}

func TestUnflatten_Conflict(t *testing.T) {
	// a 既当对象又当数组 → 冲突
	flat := map[string]any{"a.b": json.Number("1"), "a[0]": json.Number("2")}
	if _, err := Unflatten(flat, "."); err == nil {
		t.Error("conflicting object/array path should error")
	}
}

func TestUnflatten_BadIndex(t *testing.T) {
	flat := map[string]any{"a[x]": json.Number("1")}
	if _, err := Unflatten(flat, "."); err == nil {
		t.Error("non-int array index should error")
	}
}

// ---- round-trip（核心）----

func TestRoundTrip(t *testing.T) {
	cases := []string{
		`{"a":{"b":1,"c":[10,20]}}`,
		`{"users":[{"name":"bob","roles":["a","b"]},{"name":"amy"}]}`,
		`{"a":{},"b":[],"c":null,"d":true,"e":"x"}`,
		`{"n":12345678901234567890}`,
		`[1,2,3]`,
		`{"deep":{"x":{"y":{"z":[{"k":1}]}}}}`,
		`5`,
		`{}`,
		`[]`,
	}
	for _, in := range cases {
		v := decode(t, in)
		flat := Flatten(v, ".")
		got, err := Unflatten(flat, ".")
		if err != nil {
			t.Errorf("%s: unflatten err: %v", in, err)
			continue
		}
		if canon(t, v) != canon(t, got) {
			t.Errorf("round-trip mismatch:\n in:  %s\n out: %s", canon(t, v), canon(t, got))
		}
	}
}

func TestRoundTrip_CustomSep(t *testing.T) {
	v := decode(t, `{"a":{"b":{"c":1}}}`)
	flat := Flatten(v, "/")
	got, err := Unflatten(flat, "/")
	if err != nil {
		t.Fatal(err)
	}
	if canon(t, v) != canon(t, got) {
		t.Errorf("custom-sep round-trip failed: %s vs %s", canon(t, v), canon(t, got))
	}
}

// ---- Bytes wrappers ----

func TestFlattenBytes_Compact(t *testing.T) {
	out, err := FlattenBytes([]byte(`{"a":{"b":1}}`), ".", 0)
	if err != nil {
		t.Fatal(err)
	}
	if string(out) != `{"a.b":1}` {
		t.Errorf("got %s", out)
	}
}

func TestFlattenBytes_Pretty(t *testing.T) {
	out, _ := FlattenBytes([]byte(`{"a":{"b":1}}`), ".", 2)
	if string(out) == `{"a.b":1}` {
		t.Errorf("pretty should be indented: %s", out)
	}
}

func TestUnflattenBytes_NotObject(t *testing.T) {
	if _, err := UnflattenBytes([]byte(`[1,2]`), ".", 0); err == nil {
		t.Error("unflatten of non-object should error")
	}
}

func TestFlattenBytes_InvalidJSON(t *testing.T) {
	if _, err := FlattenBytes([]byte(`{not json`), ".", 0); err == nil {
		t.Error("invalid JSON should error")
	}
}
