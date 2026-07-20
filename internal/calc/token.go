// Package calc 实现 jdan calc 命令的核心：算术表达式求值。
//
// 手写递归下降解析器（recursive descent），0 依赖。支持 + - * / % ^ 运算、
// 括号、一元负号、进制操作数（0x/0b/0o）、内置函数（sqrt/abs/...）和常量
// （pi/e）。位运算归 jdan num bit，本包不做。
package calc

import (
	"fmt"
	"strconv"
	"strings"
)

// tokKind 是 token 类型。
type tokKind int

const (
	tokNumber tokKind = iota
	tokPlus
	tokMinus
	tokStar
	tokSlash
	tokPercent
	tokCaret // ^ 或 **
	tokLParen
	tokRParen
	tokComma
	tokIdent // 函数名 / 常量名
	tokEOF
)

// token 是一个词法单元。Pos 是在原始输入中的字节位置（用于错误信息）。
type token struct {
	Kind tokKind
	Num  float64 // 仅 tokNumber
	Text string  // 仅 tokIdent
	Pos  int
}

// tokenize 把表达式切成 token 流。
func tokenize(s string) ([]token, error) {
	var toks []token
	i := 0
	for i < len(s) {
		c := s[i]
		switch {
		case c == ' ' || c == '\t' || c == '\n' || c == '\r':
			i++
		case c == '+':
			toks = append(toks, token{Kind: tokPlus, Pos: i})
			i++
		case c == '-':
			toks = append(toks, token{Kind: tokMinus, Pos: i})
			i++
		case c == '*':
			// ** 当作 ^
			if i+1 < len(s) && s[i+1] == '*' {
				toks = append(toks, token{Kind: tokCaret, Pos: i})
				i += 2
			} else {
				toks = append(toks, token{Kind: tokStar, Pos: i})
				i++
			}
		case c == '/':
			toks = append(toks, token{Kind: tokSlash, Pos: i})
			i++
		case c == '%':
			toks = append(toks, token{Kind: tokPercent, Pos: i})
			i++
		case c == '^':
			toks = append(toks, token{Kind: tokCaret, Pos: i})
			i++
		case c == '(':
			toks = append(toks, token{Kind: tokLParen, Pos: i})
			i++
		case c == ')':
			toks = append(toks, token{Kind: tokRParen, Pos: i})
			i++
		case c == ',':
			toks = append(toks, token{Kind: tokComma, Pos: i})
			i++
		case isDigit(c) || c == '.':
			n, consumed, err := scanNumber(s[i:])
			if err != nil {
				return nil, fmt.Errorf("%w at position %d", err, i)
			}
			toks = append(toks, token{Kind: tokNumber, Num: n, Pos: i})
			i += consumed
		case isIdentStart(c):
			j := i + 1
			for j < len(s) && isIdentPart(s[j]) {
				j++
			}
			toks = append(toks, token{Kind: tokIdent, Text: s[i:j], Pos: i})
			i = j
		default:
			return nil, fmt.Errorf("unexpected character %q at position %d", string(c), i)
		}
	}
	toks = append(toks, token{Kind: tokEOF, Pos: len(s)})
	return toks, nil
}

// scanNumber 从开头扫一个数字字面量，支持 0x/0b/0o 整数前缀 + 十进制小数 +
// 科学计数法。返回 (值, 消耗的字节数, err)。
func scanNumber(s string) (float64, int, error) {
	// 进制前缀整数
	if len(s) >= 2 && s[0] == '0' {
		var base int
		switch s[1] {
		case 'x', 'X':
			base = 16
		case 'b', 'B':
			base = 2
		case 'o', 'O':
			base = 8
		}
		if base != 0 {
			j := 2
			for j < len(s) && isBaseDigit(s[j], base) {
				j++
			}
			if j == 2 {
				return 0, 0, fmt.Errorf("no digits after base prefix")
			}
			v, err := strconv.ParseUint(s[2:j], base, 64)
			if err != nil {
				return 0, 0, fmt.Errorf("invalid number %q", s[:j])
			}
			return float64(v), j, nil
		}
	}
	// 十进制（含小数 / 科学计数）
	j := 0
	for j < len(s) && (isDigit(s[j]) || s[j] == '.') {
		j++
	}
	// 科学计数 e / E [+-] digits
	if j < len(s) && (s[j] == 'e' || s[j] == 'E') {
		k := j + 1
		if k < len(s) && (s[k] == '+' || s[k] == '-') {
			k++
		}
		if k < len(s) && isDigit(s[k]) {
			for k < len(s) && isDigit(s[k]) {
				k++
			}
			j = k
		}
	}
	v, err := strconv.ParseFloat(s[:j], 64)
	if err != nil {
		return 0, 0, fmt.Errorf("invalid number %q", s[:j])
	}
	return v, j, nil
}

func isDigit(c byte) bool { return c >= '0' && c <= '9' }

func isBaseDigit(c byte, base int) bool {
	switch base {
	case 2:
		return c == '0' || c == '1'
	case 8:
		return c >= '0' && c <= '7'
	case 16:
		return isDigit(c) || (c >= 'a' && c <= 'f') || (c >= 'A' && c <= 'F')
	}
	return false
}

func isIdentStart(c byte) bool {
	return (c >= 'a' && c <= 'z') || (c >= 'A' && c <= 'Z') || c == '_'
}

func isIdentPart(c byte) bool {
	return isIdentStart(c) || isDigit(c)
}

// kindName 给错误信息用。
func (k tokKind) String() string {
	switch k {
	case tokNumber:
		return "number"
	case tokPlus:
		return "'+'"
	case tokMinus:
		return "'-'"
	case tokStar:
		return "'*'"
	case tokSlash:
		return "'/'"
	case tokPercent:
		return "'%'"
	case tokCaret:
		return "'^'"
	case tokLParen:
		return "'('"
	case tokRParen:
		return "')'"
	case tokComma:
		return "','"
	case tokIdent:
		return "identifier"
	case tokEOF:
		return "end of expression"
	}
	return "?"
}

// normalizeExpr trims 输入（CLI 多 arg 拼接 / stdin 末尾换行）。
func normalizeExpr(s string) string {
	return strings.TrimSpace(s)
}
