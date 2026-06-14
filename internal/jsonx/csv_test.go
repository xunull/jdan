package jsonx

import (
	"encoding/json"
	"reflect"
	"strings"
	"testing"
)

func TestCSVToJSON_WithHeader(t *testing.T) {
	in := []byte("name,age\nalice,30\nbob,25\n")
	out, err := CSVToJSON(in, true, ',')
	if err != nil {
		t.Fatal(err)
	}
	var got []map[string]string
	if err := json.Unmarshal(out, &got); err != nil {
		t.Fatal(err)
	}
	want := []map[string]string{
		{"name": "alice", "age": "30"},
		{"name": "bob", "age": "25"},
	}
	if !reflect.DeepEqual(got, want) {
		t.Errorf("got %v, want %v", got, want)
	}
}

func TestCSVToJSON_NoHeader(t *testing.T) {
	in := []byte("alice,30\nbob,25\n")
	out, err := CSVToJSON(in, false, ',')
	if err != nil {
		t.Fatal(err)
	}
	var got [][]string
	_ = json.Unmarshal(out, &got)
	want := [][]string{{"alice", "30"}, {"bob", "25"}}
	if !reflect.DeepEqual(got, want) {
		t.Errorf("got %v, want %v", got, want)
	}
}

func TestCSVToJSON_TabDelim(t *testing.T) {
	in := []byte("a\tb\n1\t2\n")
	out, err := CSVToJSON(in, true, '\t')
	if err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(string(out), `"a":"1"`) {
		t.Errorf("got %s", out)
	}
}

func TestCSVToJSON_BOMStripped(t *testing.T) {
	// Excel-exported CSV 经常带 UTF-8 BOM；不剥会让第一个 key 多出
	in := append([]byte{0xEF, 0xBB, 0xBF}, []byte("name,age\nalice,30\n")...)
	out, err := CSVToJSON(in, true, ',')
	if err != nil {
		t.Fatal(err)
	}
	var got []map[string]string
	_ = json.Unmarshal(out, &got)
	if got[0]["name"] != "alice" {
		t.Errorf("BOM not stripped, got keys %v", got[0])
	}
	// 防御性：检查 key 里没出现 BOM 字符（U+FEFF）
	for k := range got[0] {
		if strings.ContainsRune(k, '\uFEFF') {
			t.Errorf("BOM leaked into key %q", k)
		}
	}
}

func TestCSVToJSON_QuotedFields(t *testing.T) {
	// csv 标准：quoted field 内允许 comma 和 embedded newline
	in := []byte("name,bio\n\"alice\",\"line1\nline2\"\n\"bob, jr\",\"plain\"\n")
	out, err := CSVToJSON(in, true, ',')
	if err != nil {
		t.Fatal(err)
	}
	var got []map[string]string
	_ = json.Unmarshal(out, &got)
	if got[0]["bio"] != "line1\nline2" {
		t.Errorf("embedded newline lost: %q", got[0]["bio"])
	}
	if got[1]["name"] != "bob, jr" {
		t.Errorf("comma in quoted lost: %q", got[1]["name"])
	}
}

func TestCSVToJSON_RaggedRows(t *testing.T) {
	// 短行：缺失字段填空 string
	in := []byte("a,b,c\n1,2\n")
	out, _ := CSVToJSON(in, true, ',')
	var got []map[string]string
	_ = json.Unmarshal(out, &got)
	if got[0]["c"] != "" {
		t.Errorf("missing field should be empty string, got %q", got[0]["c"])
	}
}

func TestJSONToCSV_HeaderInferred(t *testing.T) {
	in := []byte(`[{"name":"alice","age":30},{"name":"bob","age":25}]`)
	out, err := JSONToCSV(in, nil, ',')
	if err != nil {
		t.Fatal(err)
	}
	s := string(out)
	// 字母序：age,name
	if !strings.HasPrefix(s, "age,name\n") {
		t.Errorf("header should be alphabetical, got first line of: %q", s)
	}
	if !strings.Contains(s, "30,alice") || !strings.Contains(s, "25,bob") {
		t.Errorf("data missing:\n%s", s)
	}
}

func TestJSONToCSV_ExplicitHeader(t *testing.T) {
	in := []byte(`[{"name":"alice","age":30,"role":"admin"}]`)
	out, _ := JSONToCSV(in, []string{"name", "role", "age"}, ',')
	want := "name,role,age\nalice,admin,30\n"
	if string(out) != want {
		t.Errorf("got %q, want %q", out, want)
	}
}

func TestJSONToCSV_MissingKeyEmpty(t *testing.T) {
	in := []byte(`[{"name":"alice","age":30},{"name":"bob"}]`)
	out, _ := JSONToCSV(in, []string{"name", "age"}, ',')
	want := "name,age\nalice,30\nbob,\n"
	if string(out) != want {
		t.Errorf("got %q, want %q", out, want)
	}
}

func TestJSONToCSV_LargeIntPreserved(t *testing.T) {
	// 关键：UseNumber 让大 int 不被 float64 损失
	in := []byte(`[{"id":9007199254740993}]`)
	out, _ := JSONToCSV(in, nil, ',')
	if !strings.Contains(string(out), "9007199254740993") {
		t.Errorf("large int lost: %s", out)
	}
}

func TestJSONToCSV_NestedSubDocument(t *testing.T) {
	// 嵌套 object 编码成 JSON 子文档
	in := []byte(`[{"name":"alice","meta":{"k":"v"}}]`)
	out, _ := JSONToCSV(in, []string{"name", "meta"}, ',')
	s := string(out)
	if !strings.Contains(s, `"{""k"":""v""}"`) { // CSV escape: " 变 ""
		t.Errorf("nested object should JSON-encode + CSV-escape; got:\n%s", s)
	}
}

func TestJSONToCSV_NotArray_Errors(t *testing.T) {
	if _, err := JSONToCSV([]byte(`{"a":1}`), nil, ','); err == nil {
		t.Error("non-array JSON should error")
	}
}

func TestJSONToCSV_ElementNotObject_Errors(t *testing.T) {
	if _, err := JSONToCSV([]byte(`[1,2,3]`), nil, ','); err == nil {
		t.Error("array of scalars should error")
	}
}

func TestRoundTrip_CSVJSONCSV(t *testing.T) {
	// CSV → JSON → CSV，header 顺序需要显式（JSON object key 顺序不保证）
	orig := "name,age,role\nalice,30,admin\nbob,25,user\n"
	asJSON, err := CSVToJSON([]byte(orig), true, ',')
	if err != nil {
		t.Fatal(err)
	}
	back, err := JSONToCSV(asJSON, []string{"name", "age", "role"}, ',')
	if err != nil {
		t.Fatal(err)
	}
	if string(back) != orig {
		t.Errorf("round-trip differs:\norig: %q\nback: %q", orig, back)
	}
}

func TestCSVToJSON_NullCellStayString(t *testing.T) {
	// 空字段保持空字符串（不变 null）
	in := []byte("a,b\n,\n")
	out, _ := CSVToJSON(in, true, ',')
	var got []map[string]string
	_ = json.Unmarshal(out, &got)
	if got[0]["a"] != "" || got[0]["b"] != "" {
		t.Errorf("empty cells: %v", got[0])
	}
}
