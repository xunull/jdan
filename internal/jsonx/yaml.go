package jsonx

import (
	"bytes"
	"encoding/json"
	"fmt"

	"go.yaml.in/yaml/v3"
)

// YAMLToJSON 将 YAML 文档转为 JSON。中间走 any，所以保留所有数据结构（不需要
// 预定义 schema）。indent <= 0 时输出紧凑 JSON，否则 pretty 输出。
func YAMLToJSON(data []byte, indent int) ([]byte, error) {
	var v any
	if err := yaml.Unmarshal(data, &v); err != nil {
		return nil, err
	}
	v = normalizeYAMLValue(v)
	if indent <= 0 {
		return json.Marshal(v)
	}
	out, err := json.MarshalIndent(v, "", spaces(indent))
	if err != nil {
		return nil, err
	}
	return out, nil
}

// JSONToYAML 将 JSON 文档转为 YAML。json.Number 转为 int64/float64，避免
// yaml.Marshal 把数字误 quote 成 string。indent 给 yaml encoder（默认 2）。
func JSONToYAML(data []byte, indent int) ([]byte, error) {
	var v any
	dec := json.NewDecoder(bytes.NewReader(data))
	dec.UseNumber()
	if err := dec.Decode(&v); err != nil {
		return nil, err
	}
	v = denumberize(v)
	var buf bytes.Buffer
	enc := yaml.NewEncoder(&buf)
	if indent > 0 {
		enc.SetIndent(indent)
	}
	if err := enc.Encode(v); err != nil {
		return nil, err
	}
	if err := enc.Close(); err != nil {
		return nil, err
	}
	return buf.Bytes(), nil
}

// normalizeYAMLValue 把 yaml.v2 风格的 map[any]any 递归转 map[string]any，
// 让 json.Marshal 能吃。yaml.v3 大多数情况已经给 map[string]any，但 mixed-key
// mapping 仍可能产生 interface key，必须 normalize。
func normalizeYAMLValue(v any) any {
	switch x := v.(type) {
	case map[any]any:
		out := make(map[string]any, len(x))
		for k, vv := range x {
			out[fmt.Sprint(k)] = normalizeYAMLValue(vv)
		}
		return out
	case map[string]any:
		out := make(map[string]any, len(x))
		for k, vv := range x {
			out[k] = normalizeYAMLValue(vv)
		}
		return out
	case []any:
		out := make([]any, len(x))
		for i, vv := range x {
			out[i] = normalizeYAMLValue(vv)
		}
		return out
	default:
		return v
	}
}

// denumberize 把 json.Number 递归还原为 int64/float64，让 yaml.Marshal 输出
// 不带引号的数字（json.Number 本质是 string）。Int64 优先，溢出再退到 Float64，
// 都失败保留 string。
func denumberize(v any) any {
	switch x := v.(type) {
	case json.Number:
		if i, err := x.Int64(); err == nil {
			return i
		}
		if f, err := x.Float64(); err == nil {
			return f
		}
		return x.String()
	case map[string]any:
		out := make(map[string]any, len(x))
		for k, vv := range x {
			out[k] = denumberize(vv)
		}
		return out
	case []any:
		out := make([]any, len(x))
		for i, vv := range x {
			out[i] = denumberize(vv)
		}
		return out
	default:
		return v
	}
}

func spaces(n int) string {
	b := make([]byte, n)
	for i := range b {
		b[i] = ' '
	}
	return string(b)
}
