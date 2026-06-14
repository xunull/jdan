package jsonx

import (
	"bufio"
	"bytes"
	"encoding/json"
	"errors"
	"fmt"
	"io"
)

// linesScanner 返回一个能处理巨大单行（8MB）的 Scanner。JSONL 实际 spec 没
// 规定上限，但 logging/data warehouse 一行 1-2MB 是常见的，8MB 给余量。
func linesScanner(r io.Reader) *bufio.Scanner {
	s := bufio.NewScanner(r)
	s.Buffer(make([]byte, 64*1024), 8*1024*1024)
	return s
}

// LinesCount 数 JSONL 中合法的非空记录数。遇到非法 JSON 行立即 error。
func LinesCount(r io.Reader) (int, error) {
	n := 0
	lineNo := 0
	s := linesScanner(r)
	for s.Scan() {
		lineNo++
		line := bytes.TrimSpace(s.Bytes())
		if len(line) == 0 {
			continue
		}
		if !json.Valid(line) {
			return 0, fmt.Errorf("line %d: invalid JSON", lineNo)
		}
		n++
	}
	if err := s.Err(); err != nil {
		return 0, err
	}
	return n, nil
}

// LinesGet 返回 JSONL 中第 idx 行（0-based，跳过空行）。
func LinesGet(r io.Reader, idx int) ([]byte, error) {
	if idx < 0 {
		return nil, errors.New("negative index not supported for JSONL")
	}
	s := linesScanner(r)
	i := 0
	for s.Scan() {
		line := bytes.TrimSpace(s.Bytes())
		if len(line) == 0 {
			continue
		}
		if i == idx {
			cp := make([]byte, len(line))
			copy(cp, line)
			return cp, nil
		}
		i++
	}
	if err := s.Err(); err != nil {
		return nil, err
	}
	return nil, fmt.Errorf("index %d out of range (file has %d records)", idx, i)
}

// LinesHead 返回 JSONL 的前 n 条非空记录。
func LinesHead(r io.Reader, n int) ([][]byte, error) {
	if n <= 0 {
		return nil, errors.New("n must be positive")
	}
	out := make([][]byte, 0, n)
	s := linesScanner(r)
	for s.Scan() {
		if len(out) >= n {
			break
		}
		line := bytes.TrimSpace(s.Bytes())
		if len(line) == 0 {
			continue
		}
		cp := make([]byte, len(line))
		copy(cp, line)
		out = append(out, cp)
	}
	if err := s.Err(); err != nil {
		return nil, err
	}
	return out, nil
}
