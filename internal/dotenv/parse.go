// Package dotenv 实现 jdan env 命令的核心：.env 文件解析 + lint + diff + redact。
// 全部基于 stdlib，0 新依赖。偏"检查 / 对比 / 脱敏"，不做加载（那是 direnv /
// dotenv-cli 的范畴）。
package dotenv

import (
	"bufio"
	"bytes"
	"io"
	"strings"
)

// Quote 表示 value 的引号类型。
type Quote int

const (
	QuoteNone   Quote = iota // KEY=value
	QuoteDouble              // KEY="value"
	QuoteSingle              // KEY='value'
)

// Entry 是 .env 里的一行有效赋值（注释/空行不产生 Entry）。
type Entry struct {
	Line      int    // 1-based 行号
	Key       string // 去掉 export 前缀后的 key
	Value     string // 去引号、去行内注释后的值
	Quote     Quote  // value 的引号类型
	HadExport bool   // 原行是否带 export 前缀
	Raw       string // 原始整行（不含换行）
	RawValue  string // 去引号前、= 右侧的原始内容（保留尾空格用于 lint）
	HasEquals bool   // 是否含 '='（无 = 的孤立 token 也记一条，Key=整行）
}

// File 是解析后的整个 .env。
type File struct {
	Entries []Entry
	// HasCRLF / HasBOM 供 lint 检查 Windows 编辑器留下的问题
	HasCRLF bool
	HasBOM  bool
}

// bomPrefix 是 UTF-8 BOM（U+FEFF），用 escape 避免源码里出现裸 BOM。
const bomPrefix = "\ufeff"

// Parse 解析 .env 内容。注释行和空行被跳过（不进 Entries），但行号正确保留。
//
// 先读全部内容：bufio.ScanLines 会静默剥掉行尾 \r，无法据此检测 CRLF，所以
// 在分行前直接扫原始字节判断 \r\n / BOM。
func Parse(r io.Reader) (*File, error) {
	f := &File{}
	data, err := io.ReadAll(r)
	if err != nil {
		return nil, err
	}
	if bytes.HasPrefix(data, []byte(bomPrefix)) {
		f.HasBOM = true
		data = bytes.TrimPrefix(data, []byte(bomPrefix))
	}
	if bytes.Contains(data, []byte("\r\n")) {
		f.HasCRLF = true
	}

	sc := bufio.NewScanner(bytes.NewReader(data))
	sc.Buffer(make([]byte, 64*1024), 4*1024*1024)
	lineNo := 0
	for sc.Scan() {
		lineNo++
		raw := sc.Text() // ScanLines 已剥掉行尾 \r

		trimmed := strings.TrimSpace(raw)
		if trimmed == "" || strings.HasPrefix(trimmed, "#") {
			continue
		}

		e := parseLine(lineNo, raw)
		f.Entries = append(f.Entries, e)
	}
	if err := sc.Err(); err != nil {
		return nil, err
	}
	return f, nil
}

// parseLine 解析单行赋值。保留足够信息让 lint 能检出未引号空格 / 尾空格。
func parseLine(lineNo int, raw string) Entry {
	e := Entry{Line: lineNo, Raw: raw}
	work := strings.TrimLeft(raw, " \t")

	// export 前缀
	if rest, ok := strings.CutPrefix(work, "export "); ok {
		e.HadExport = true
		work = strings.TrimLeft(rest, " \t")
	}

	eq := strings.IndexByte(work, '=')
	if eq < 0 {
		// 无 '='：孤立 token，Key 记原始 trim，lint 会报
		e.Key = strings.TrimSpace(work)
		e.HasEquals = false
		return e
	}
	e.HasEquals = true
	e.Key = strings.TrimSpace(work[:eq])
	rawVal := work[eq+1:]
	e.RawValue = rawVal
	e.Value, e.Quote = unquoteValue(rawVal)
	return e
}

// unquoteValue 处理 value 的引号 + 行内注释。
//   - "..." / '...' → 剥引号，引号内保留原样（含空格），引号后的内容忽略
//   - 无引号 → trim 两侧空格；首个未转义 ` #`（空格+#）起为行内注释
func unquoteValue(s string) (string, Quote) {
	trimmed := strings.TrimLeft(s, " \t")
	if len(trimmed) > 0 && (trimmed[0] == '"' || trimmed[0] == '\'') {
		q := trimmed[0]
		if end := strings.IndexByte(trimmed[1:], q); end >= 0 {
			val := trimmed[1 : 1+end]
			if q == '"' {
				return val, QuoteDouble
			}
			return val, QuoteSingle
		}
		// 引号未闭合：当无引号处理
	}
	// 无引号：先去行内注释（" #" 之前），再 trim
	val := stripInlineComment(s)
	return strings.TrimSpace(val), QuoteNone
}

// stripInlineComment 去掉无引号 value 的行内注释（空格 + #）。
func stripInlineComment(s string) string {
	for i := 0; i < len(s)-1; i++ {
		if (s[i] == ' ' || s[i] == '\t') && s[i+1] == '#' {
			return s[:i]
		}
	}
	return s
}
