package jsonx

import (
	"bytes"
	"encoding/csv"
	"encoding/json"
	"fmt"
	"sort"
)

// stripBOM 去掉 UTF-8 BOM（EF BB BF）。Excel/Numbers 导出的 CSV 常带 BOM，
// 不剥会让第一个 header 多出几个字节。
func stripBOM(data []byte) []byte {
	return bytes.TrimPrefix(data, []byte{0xEF, 0xBB, 0xBF})
}

// CSVToJSON 将 CSV 转 JSON。返回的是紧凑 JSON byte，调用方按需 Pretty。
//   - hasHeader=true:  第一行做 keys，输出 array of {col: value}
//   - hasHeader=false: 输出 array of array of string
//   - delim=0 默认为 ','
//
// 所有 cell 都保持 string 类型（CSV-as-strings）。类型推断留给下游 (jq / 用户)。
func CSVToJSON(data []byte, hasHeader bool, delim rune) ([]byte, error) {
	if delim == 0 {
		delim = ','
	}
	r := csv.NewReader(bytes.NewReader(stripBOM(data)))
	r.Comma = delim
	r.FieldsPerRecord = -1 // ragged 允许（短行用空 string 填）
	rows, err := r.ReadAll()
	if err != nil {
		return nil, fmt.Errorf("csv parse: %w", err)
	}
	if !hasHeader {
		return json.Marshal(rows)
	}
	if len(rows) == 0 {
		return json.Marshal([]map[string]string{})
	}
	header := rows[0]
	objs := make([]map[string]string, 0, len(rows)-1)
	for _, row := range rows[1:] {
		obj := make(map[string]string, len(header))
		for i, h := range header {
			if i < len(row) {
				obj[h] = row[i]
			} else {
				obj[h] = ""
			}
		}
		objs = append(objs, obj)
	}
	return json.Marshal(objs)
}

// JSONToCSV 将 JSON array of objects 转 CSV。
//   - 输入必须是 array；每个 element 必须是 object（嵌套 object/array 会被
//     JSON-encode 成 sub-document 写入单元格）
//   - headerOrder 不空时按它定义列序；空时取所有 key 的并集并字典序排
//   - delim=0 默认为 ','
//   - 缺失字段输出为空字符串（与 pandas to_csv 一致）
func JSONToCSV(data []byte, headerOrder []string, delim rune) ([]byte, error) {
	if delim == 0 {
		delim = ','
	}
	dec := json.NewDecoder(bytes.NewReader(data))
	dec.UseNumber()
	var arr []any
	if err := dec.Decode(&arr); err != nil {
		return nil, fmt.Errorf("json must be array of objects: %w", err)
	}
	rows := make([]map[string]any, 0, len(arr))
	for i, el := range arr {
		obj, ok := el.(map[string]any)
		if !ok {
			return nil, fmt.Errorf("element %d is not an object (got %T)", i, el)
		}
		rows = append(rows, obj)
	}
	var header []string
	if len(headerOrder) > 0 {
		header = headerOrder
	} else {
		keySet := make(map[string]bool)
		for _, r := range rows {
			for k := range r {
				keySet[k] = true
			}
		}
		header = make([]string, 0, len(keySet))
		for k := range keySet {
			header = append(header, k)
		}
		sort.Strings(header)
	}
	var buf bytes.Buffer
	w := csv.NewWriter(&buf)
	w.Comma = delim
	if err := w.Write(header); err != nil {
		return nil, err
	}
	for _, row := range rows {
		record := make([]string, len(header))
		for i, h := range header {
			if v, ok := row[h]; ok {
				record[i] = csvCell(v)
			}
		}
		if err := w.Write(record); err != nil {
			return nil, err
		}
	}
	w.Flush()
	if err := w.Error(); err != nil {
		return nil, err
	}
	return buf.Bytes(), nil
}

// csvCell 把 JSON scalar 转 CSV-friendly string。
//   - null → 空 string（区别于字面 "null"，跟 pandas 行为对齐）
//   - bool → "true" / "false"
//   - json.Number → 原 string 形式（保留精度）
//   - string → 原样
//   - object/array → JSON-encode 成 sub-document
func csvCell(v any) string {
	if v == nil {
		return ""
	}
	switch x := v.(type) {
	case string:
		return x
	case bool:
		if x {
			return "true"
		}
		return "false"
	case json.Number:
		return x.String()
	}
	b, _ := json.Marshal(v)
	return string(b)
}
