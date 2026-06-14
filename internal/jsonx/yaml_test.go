package jsonx

import (
	"encoding/json"
	"reflect"
	"strings"
	"testing"
)

func TestYAMLToJSON_Basic(t *testing.T) {
	in := []byte("name: alice\nage: 30\n")
	out, err := YAMLToJSON(in, 0)
	if err != nil {
		t.Fatal(err)
	}
	var got map[string]any
	if err := json.Unmarshal(out, &got); err != nil {
		t.Fatalf("YAML→JSON output not valid JSON: %v\n%s", err, out)
	}
	if got["name"] != "alice" {
		t.Errorf("name = %v, want alice", got["name"])
	}
	if got["age"].(float64) != 30 {
		t.Errorf("age = %v, want 30", got["age"])
	}
}

func TestYAMLToJSON_Nested(t *testing.T) {
	in := []byte(`
tags:
  - a
  - b
nested:
  count: 42
  enabled: true
`)
	out, err := YAMLToJSON(in, 0)
	if err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(string(out), `"tags":["a","b"]`) {
		t.Errorf("array unmarshal wrong:\n%s", out)
	}
	if !strings.Contains(string(out), `"count":42`) {
		t.Errorf("nested int not preserved:\n%s", out)
	}
	if !strings.Contains(string(out), `"enabled":true`) {
		t.Errorf("nested bool not preserved:\n%s", out)
	}
}

func TestYAMLToJSON_NumbersStayNumeric(t *testing.T) {
	// 关键回归：YAML 数字不能变成 JSON string
	in := []byte("port: 8080\nratio: 1.5\n")
	out, _ := YAMLToJSON(in, 0)
	s := string(out)
	if !strings.Contains(s, `"port":8080`) {
		t.Errorf("int got quoted:\n%s", s)
	}
	if !strings.Contains(s, `"ratio":1.5`) {
		t.Errorf("float got quoted:\n%s", s)
	}
}

func TestYAMLToJSON_PrettyIndent(t *testing.T) {
	in := []byte("a: 1\n")
	out, _ := YAMLToJSON(in, 2)
	if !strings.Contains(string(out), "  \"a\":") {
		t.Errorf("pretty=2 should give 2-space indent:\n%s", out)
	}
}

func TestYAMLToJSON_InvalidYAML_Errors(t *testing.T) {
	if _, err := YAMLToJSON([]byte("a: b\n  c: d\n bad indent\n"), 0); err == nil {
		t.Error("invalid YAML should error")
	}
}

func TestJSONToYAML_Basic(t *testing.T) {
	in := []byte(`{"name":"alice","age":30}`)
	out, err := JSONToYAML(in, 2)
	if err != nil {
		t.Fatal(err)
	}
	s := string(out)
	if !strings.Contains(s, "name: alice") || !strings.Contains(s, "age: 30") {
		t.Errorf("got:\n%s", s)
	}
}

func TestJSONToYAML_NumbersStayUnquoted(t *testing.T) {
	// 关键回归：json.Number 不能被 yaml 当成 string quote
	in := []byte(`{"port":8080,"ratio":1.5}`)
	out, _ := JSONToYAML(in, 2)
	s := string(out)
	if !strings.Contains(s, "port: 8080") {
		t.Errorf("int got quoted in yaml:\n%s", s)
	}
	if !strings.Contains(s, "ratio: 1.5") {
		t.Errorf("float got quoted in yaml:\n%s", s)
	}
	// 检查没出现 "8080" 或 "1.5"（quoted form）
	if strings.Contains(s, `"8080"`) || strings.Contains(s, `"1.5"`) {
		t.Errorf("number was quoted:\n%s", s)
	}
}

func TestJSONToYAML_LargeInt(t *testing.T) {
	// 大 int 应当保留为整数（int64 范围内）
	in := []byte(`{"id":9007199254740993}`) // 2^53 + 1
	out, _ := JSONToYAML(in, 2)
	if !strings.Contains(string(out), "id: 9007199254740993") {
		t.Errorf("large int lost: %s", out)
	}
}

func TestRoundTrip_YAMLJSONYAML(t *testing.T) {
	original := []byte(`name: alice
age: 30
tags:
  - admin
  - active
`)
	asJSON, err := YAMLToJSON(original, 0)
	if err != nil {
		t.Fatal(err)
	}
	backToYAML, err := JSONToYAML(asJSON, 2)
	if err != nil {
		t.Fatal(err)
	}
	// 比 yaml 解析结果，不比文本（field 顺序可能变）
	var origDecoded, rtDecoded any
	if _, err := YAMLToJSON(original, 0); err != nil {
		t.Fatal(err)
	}
	// 用 json round-trip 间接比较
	rtJSON, _ := YAMLToJSON(backToYAML, 0)
	_ = json.Unmarshal(asJSON, &origDecoded)
	_ = json.Unmarshal(rtJSON, &rtDecoded)
	if !reflect.DeepEqual(origDecoded, rtDecoded) {
		t.Errorf("round-trip differs:\norig: %v\nrt:   %v", origDecoded, rtDecoded)
	}
}

func TestJSONToYAML_NestedStructure(t *testing.T) {
	in := []byte(`{"a":{"b":{"c":1}},"xs":[1,2,3]}`)
	out, err := JSONToYAML(in, 2)
	if err != nil {
		t.Fatal(err)
	}
	s := string(out)
	for _, want := range []string{"a:", "b:", "c: 1", "- 1", "- 2", "- 3"} {
		if !strings.Contains(s, want) {
			t.Errorf("missing %q in:\n%s", want, s)
		}
	}
}
