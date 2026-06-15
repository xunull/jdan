package numconv

import (
	"fmt"
	"strings"
)

// BitOp 是支持的位运算符。
type BitOp string

const (
	OpAND BitOp = "AND"
	OpOR  BitOp = "OR"
	OpXOR BitOp = "XOR"
	OpNOT BitOp = "NOT" // 单目
	OpSHL BitOp = "<<"
	OpSHR BitOp = ">>"
)

// normalizeOp 把符号别名（& | ^ ~）和大小写统一到 BitOp。
func normalizeOp(tok string) (BitOp, bool) {
	switch strings.ToUpper(tok) {
	case "AND", "&":
		return OpAND, true
	case "OR", "|":
		return OpOR, true
	case "XOR", "^":
		return OpXOR, true
	case "NOT", "~":
		return OpNOT, true
	case "<<", "SHL":
		return OpSHL, true
	case ">>", "SHR":
		return OpSHR, true
	}
	return "", false
}

// BitExprResult 是位运算的求值结果。
type BitExprResult struct {
	Op     BitOp
	A      uint64
	B      uint64 // NOT 时无意义
	Result uint64
	Width  int // NOT 取反用的位宽（默认 64）
}

// EvalBitExpr 解析并求值一个位表达式。支持两种形态：
//   - 二元: "<a> <op> <b>"   如 "0xFF AND 0x0F"、"1 << 8"、"5 ^ 2"
//   - 单目: "NOT <a>" / "~ <a>"
//
// width 是 NOT 运算的取反位宽（其他运算忽略）；<=0 时默认 64。
// tokenizer 简单按空白分词；符号紧贴操作数（如 "1<<8"）也支持，先做切分。
func EvalBitExpr(expr string, width int) (BitExprResult, error) {
	if width <= 0 || width > 64 {
		width = 64
	}
	tokens := tokenizeBitExpr(expr)
	switch len(tokens) {
	case 2: // 单目：NOT a
		op, ok := normalizeOp(tokens[0])
		if !ok || op != OpNOT {
			return BitExprResult{}, fmt.Errorf("unary form requires NOT/~, got %q", tokens[0])
		}
		a, _, err := ParseValue(tokens[1])
		if err != nil {
			return BitExprResult{}, err
		}
		var mask uint64 = ^uint64(0)
		if width < 64 {
			mask = (uint64(1) << width) - 1
		}
		return BitExprResult{Op: OpNOT, A: a, Result: (^a) & mask, Width: width}, nil
	case 3: // 二元：a op b
		a, _, err := ParseValue(tokens[0])
		if err != nil {
			return BitExprResult{}, err
		}
		op, ok := normalizeOp(tokens[1])
		if !ok {
			return BitExprResult{}, fmt.Errorf("unknown operator %q (want AND/OR/XOR/<</>> or & | ^)", tokens[1])
		}
		if op == OpNOT {
			return BitExprResult{}, fmt.Errorf("NOT is unary; use \"NOT %s\"", tokens[0])
		}
		b, _, err := ParseValue(tokens[2])
		if err != nil {
			return BitExprResult{}, err
		}
		res, err := applyBinary(op, a, b)
		if err != nil {
			return BitExprResult{}, err
		}
		return BitExprResult{Op: op, A: a, B: b, Result: res, Width: width}, nil
	default:
		return BitExprResult{}, fmt.Errorf("expected \"a OP b\" or \"NOT a\", got %d tokens", len(tokens))
	}
}

func applyBinary(op BitOp, a, b uint64) (uint64, error) {
	switch op {
	case OpAND:
		return a & b, nil
	case OpOR:
		return a | b, nil
	case OpXOR:
		return a ^ b, nil
	case OpSHL:
		if b >= 64 {
			return 0, nil // 移出全部位
		}
		return a << b, nil
	case OpSHR:
		if b >= 64 {
			return 0, nil
		}
		return a >> b, nil
	}
	return 0, fmt.Errorf("unsupported binary op %q", op)
}

// tokenizeBitExpr 切分表达式。先在 << >> & | ^ ~ 周围插空格，再按空白分词，
// 这样 "1<<8" 和 "1 << 8" 都能正确切成 3 个 token。
func tokenizeBitExpr(expr string) []string {
	// 多字符运算符先处理，避免被单字符规则拆开
	expr = strings.ReplaceAll(expr, "<<", " << ")
	expr = strings.ReplaceAll(expr, ">>", " >> ")
	for _, sym := range []string{"&", "|", "^", "~"} {
		expr = strings.ReplaceAll(expr, sym, " "+sym+" ")
	}
	return strings.Fields(expr)
}
