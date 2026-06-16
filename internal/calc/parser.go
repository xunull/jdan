package calc

import "fmt"

// parser 是递归下降解析器。文法（优先级低→高）：
//
//	expr   := term  (('+' | '-') term)*
//	term   := unary (('*' | '/' | '%') unary)*
//	unary  := '-' unary | power
//	power  := atom ('^' unary)?        // ^ 右结合
//	atom   := number | const | ident '(' args ')' | '(' expr ')'
type parser struct {
	toks []token
	pos  int
}

// Parse 解析表达式字符串成 AST。
func Parse(input string) (Node, error) {
	input = normalizeExpr(input)
	if input == "" {
		return nil, fmt.Errorf("empty expression")
	}
	toks, err := tokenize(input)
	if err != nil {
		return nil, err
	}
	p := &parser{toks: toks}
	n, err := p.parseExpr()
	if err != nil {
		return nil, err
	}
	if p.cur().Kind != tokEOF {
		return nil, fmt.Errorf("unexpected %s at position %d", p.cur().Kind, p.cur().Pos)
	}
	return n, nil
}

func (p *parser) cur() token  { return p.toks[p.pos] }
func (p *parser) advance()    { p.pos++ }

func (p *parser) parseExpr() (Node, error) {
	left, err := p.parseTerm()
	if err != nil {
		return nil, err
	}
	for {
		k := p.cur().Kind
		if k != tokPlus && k != tokMinus {
			return left, nil
		}
		p.advance()
		right, err := p.parseTerm()
		if err != nil {
			return nil, err
		}
		left = BinNode{Op: k, L: left, R: right}
	}
}

func (p *parser) parseTerm() (Node, error) {
	left, err := p.parseUnary()
	if err != nil {
		return nil, err
	}
	for {
		k := p.cur().Kind
		if k != tokStar && k != tokSlash && k != tokPercent {
			return left, nil
		}
		p.advance()
		right, err := p.parseUnary()
		if err != nil {
			return nil, err
		}
		left = BinNode{Op: k, L: left, R: right}
	}
}

func (p *parser) parseUnary() (Node, error) {
	if p.cur().Kind == tokMinus {
		p.advance()
		x, err := p.parseUnary()
		if err != nil {
			return nil, err
		}
		return UnaryNode{Op: tokMinus, X: x}, nil
	}
	// 一元正号无意义，直接跳过
	if p.cur().Kind == tokPlus {
		p.advance()
		return p.parseUnary()
	}
	return p.parsePower()
}

func (p *parser) parsePower() (Node, error) {
	base, err := p.parseAtom()
	if err != nil {
		return nil, err
	}
	if p.cur().Kind == tokCaret {
		p.advance()
		// ^ 右结合：右侧递归到 unary（让 -2 这种指数也行，且 2^3^2 右结合）
		exp, err := p.parseUnary()
		if err != nil {
			return nil, err
		}
		return BinNode{Op: tokCaret, L: base, R: exp}, nil
	}
	return base, nil
}

func (p *parser) parseAtom() (Node, error) {
	t := p.cur()
	switch t.Kind {
	case tokNumber:
		p.advance()
		return NumNode{Val: t.Num}, nil
	case tokLParen:
		p.advance()
		n, err := p.parseExpr()
		if err != nil {
			return nil, err
		}
		if p.cur().Kind != tokRParen {
			return nil, fmt.Errorf("expected ')' at position %d, got %s", p.cur().Pos, p.cur().Kind)
		}
		p.advance()
		return n, nil
	case tokIdent:
		p.advance()
		// 函数调用？
		if p.cur().Kind == tokLParen {
			p.advance()
			args, err := p.parseArgs()
			if err != nil {
				return nil, err
			}
			return CallNode{Name: t.Text, Args: args}, nil
		}
		// 否则当常量
		return ConstNode{Name: t.Text}, nil
	case tokEOF:
		return nil, fmt.Errorf("unexpected end of expression (expected operand)")
	default:
		return nil, fmt.Errorf("unexpected %s at position %d (expected operand)", t.Kind, t.Pos)
	}
}

// parseArgs 解析 '(' 后的参数列表，直到 ')'。已消耗 '('。
func (p *parser) parseArgs() ([]Node, error) {
	var args []Node
	if p.cur().Kind == tokRParen {
		p.advance()
		return args, nil
	}
	for {
		n, err := p.parseExpr()
		if err != nil {
			return nil, err
		}
		args = append(args, n)
		switch p.cur().Kind {
		case tokComma:
			p.advance()
		case tokRParen:
			p.advance()
			return args, nil
		default:
			return nil, fmt.Errorf("expected ',' or ')' at position %d, got %s", p.cur().Pos, p.cur().Kind)
		}
	}
}
