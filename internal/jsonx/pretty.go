// Package jsonx 实现 jdan json 命令的核心：pretty/minify/path/keys/diff/lines
// 以及 yaml/csv 互转。设计要点：
//   - 用 json.Decoder.UseNumber() 保留数字精度（不被 float64 损失）
//   - path 同时支持 dot-path（users.0.name）和 bracket（users[0].name）和 JSON Pointer
//   - diff 输出符合 RFC 6902 JSON Patch 标准
//   - lines 走 bufio.Scanner，64KB 起步 buffer，支持 8MB 单行（典型 JSONL 上限）
package jsonx

import (
	"bytes"
	"encoding/json"
	"strings"
)

// Pretty 美化 JSON。stdlib map marshal 默认按 key 字典序，所以 sort 是免费的。
// indent <= 0 时按 2 空格处理。
func Pretty(data []byte, indent int) ([]byte, error) {
	if indent <= 0 {
		indent = 2
	}
	var v any
	dec := json.NewDecoder(bytes.NewReader(data))
	dec.UseNumber()
	if err := dec.Decode(&v); err != nil {
		return nil, err
	}
	return json.MarshalIndent(v, "", strings.Repeat(" ", indent))
}

// Minify 压成单行紧凑 JSON。走 json.Compact 不需要 unmarshal，避免数字精度损失。
func Minify(data []byte) ([]byte, error) {
	var buf bytes.Buffer
	if err := json.Compact(&buf, data); err != nil {
		return nil, err
	}
	return buf.Bytes(), nil
}
