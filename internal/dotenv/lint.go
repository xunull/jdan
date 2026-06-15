package dotenv

import (
	"fmt"
	"strings"
)

// Severity 是 lint issue 的级别。
type Severity string

const (
	SevError   Severity = "error"
	SevWarning Severity = "warning"
)

// Issue 是一条 lint 发现。
type Issue struct {
	Line     int      `json:"line"`
	Severity Severity `json:"severity"`
	Message  string   `json:"message"`
}

// Lint 检查 .env 的常见问题，返回按行号排序的 issue 列表。
//
// 检查项：
//   - 重复 key（error 级别可选，默认 warning）
//   - 含空格但未引号的 value（shell 加载会截断 → warning）
//   - 非法 key 名（数字开头 / 含非法字符 → error）
//   - 缺 '=' 的孤立 token（error）
//   - value 尾随空格（复制粘贴 bug → warning）
//   - 文件级 CRLF / BOM（warning，各报一次）
func Lint(f *File) []Issue {
	var issues []Issue

	if f.HasBOM {
		issues = append(issues, Issue{Line: 1, Severity: SevWarning, Message: "file has UTF-8 BOM (may break shell parsing)"})
	}
	if f.HasCRLF {
		issues = append(issues, Issue{Line: 1, Severity: SevWarning, Message: "file uses CRLF line endings (use LF)"})
	}

	firstSeen := map[string]int{}
	for _, e := range f.Entries {
		if !e.HasEquals {
			issues = append(issues, Issue{
				Line: e.Line, Severity: SevError,
				Message: fmt.Sprintf("line has no '=' (orphan token %q)", e.Key),
			})
			continue
		}
		if e.Key == "" {
			issues = append(issues, Issue{
				Line: e.Line, Severity: SevError, Message: "empty key before '='",
			})
			continue
		}
		if !validKeyName(e.Key) {
			issues = append(issues, Issue{
				Line: e.Line, Severity: SevError,
				Message: fmt.Sprintf("invalid key name %q (must match [A-Za-z_][A-Za-z0-9_]*)", e.Key),
			})
		}
		if prev, ok := firstSeen[e.Key]; ok {
			issues = append(issues, Issue{
				Line: e.Line, Severity: SevWarning,
				Message: fmt.Sprintf("duplicate key %s (first at line %d)", e.Key, prev),
			})
		} else {
			firstSeen[e.Key] = e.Line
		}
		// 未引号但含空格：shell 加载会截断
		if e.Quote == QuoteNone && strings.ContainsAny(strings.TrimSpace(e.RawValue), " \t") {
			// RawValue trim 后仍含内部空格才算（"hello world"）
			inner := strings.TrimSpace(stripInlineComment(e.RawValue))
			if strings.ContainsAny(inner, " \t") {
				issues = append(issues, Issue{
					Line: e.Line, Severity: SevWarning,
					Message: fmt.Sprintf("unquoted value with spaces: %s=%s", e.Key, inner),
				})
			}
		}
		// value 尾随空格（仅无引号时有意义；引号内空格是有意的）
		if e.Quote == QuoteNone && e.RawValue != "" {
			noComment := stripInlineComment(e.RawValue)
			if noComment != strings.TrimRight(noComment, " \t") && strings.TrimSpace(noComment) != "" {
				issues = append(issues, Issue{
					Line: e.Line, Severity: SevWarning,
					Message: fmt.Sprintf("trailing whitespace in value for %s", e.Key),
				})
			}
		}
	}
	return issues
}

func validKeyName(k string) bool {
	if k == "" {
		return false
	}
	for i, r := range k {
		isLetter := (r >= 'a' && r <= 'z') || (r >= 'A' && r <= 'Z') || r == '_'
		isDigit := r >= '0' && r <= '9'
		if i == 0 {
			if !isLetter {
				return false
			}
		} else if !isLetter && !isDigit {
			return false
		}
	}
	return true
}

// CountBySeverity 返回 (errors, warnings)。
func CountBySeverity(issues []Issue) (errors, warnings int) {
	for _, i := range issues {
		switch i.Severity {
		case SevError:
			errors++
		case SevWarning:
			warnings++
		}
	}
	return
}
