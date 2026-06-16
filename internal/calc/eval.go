package calc

import (
	"fmt"
	"math"
	"strings"
)

// constants 是内置常量表。
var constants = map[string]float64{
	"pi":  math.Pi,
	"e":   math.E,
	"tau": 2 * math.Pi,
}

// funcs1 是单参数函数表。
var funcs1 = map[string]func(float64) (float64, error){
	"sqrt": func(x float64) (float64, error) {
		if x < 0 {
			return 0, fmt.Errorf("sqrt of negative number")
		}
		return math.Sqrt(x), nil
	},
	"abs":   func(x float64) (float64, error) { return math.Abs(x), nil },
	"floor": func(x float64) (float64, error) { return math.Floor(x), nil },
	"ceil":  func(x float64) (float64, error) { return math.Ceil(x), nil },
	"round": func(x float64) (float64, error) { return math.Round(x), nil },
	"ln":    func(x float64) (float64, error) {
		if x <= 0 {
			return 0, fmt.Errorf("ln of non-positive number")
		}
		return math.Log(x), nil
	},
	"log10": func(x float64) (float64, error) {
		if x <= 0 {
			return 0, fmt.Errorf("log10 of non-positive number")
		}
		return math.Log10(x), nil
	},
	"sin": func(x float64) (float64, error) { return math.Sin(x), nil },
	"cos": func(x float64) (float64, error) { return math.Cos(x), nil },
	"tan": func(x float64) (float64, error) { return math.Tan(x), nil },
}

// funcsVar 是可变参数函数表（min/max）。
var funcsVar = map[string]func([]float64) (float64, error){
	"min": func(xs []float64) (float64, error) {
		if len(xs) == 0 {
			return 0, fmt.Errorf("min requires at least 1 argument")
		}
		m := xs[0]
		for _, x := range xs[1:] {
			m = math.Min(m, x)
		}
		return m, nil
	},
	"max": func(xs []float64) (float64, error) {
		if len(xs) == 0 {
			return 0, fmt.Errorf("max requires at least 1 argument")
		}
		m := xs[0]
		for _, x := range xs[1:] {
			m = math.Max(m, x)
		}
		return m, nil
	},
}

// Eval 求值一个 AST。
func Eval(n Node) (float64, error) {
	switch x := n.(type) {
	case NumNode:
		return x.Val, nil
	case ConstNode:
		v, ok := constants[strings.ToLower(x.Name)]
		if !ok {
			return 0, fmt.Errorf("unknown constant or variable %q", x.Name)
		}
		return v, nil
	case UnaryNode:
		v, err := Eval(x.X)
		if err != nil {
			return 0, err
		}
		if x.Op == tokMinus {
			return -v, nil
		}
		return v, nil
	case BinNode:
		l, err := Eval(x.L)
		if err != nil {
			return 0, err
		}
		r, err := Eval(x.R)
		if err != nil {
			return 0, err
		}
		return applyBin(x.Op, l, r)
	case CallNode:
		return evalCall(x)
	}
	return 0, fmt.Errorf("unknown node type")
}

func applyBin(op tokKind, l, r float64) (float64, error) {
	switch op {
	case tokPlus:
		return l + r, nil
	case tokMinus:
		return l - r, nil
	case tokStar:
		return l * r, nil
	case tokSlash:
		if r == 0 {
			return 0, fmt.Errorf("division by zero")
		}
		return l / r, nil
	case tokPercent:
		if r == 0 {
			return 0, fmt.Errorf("modulo by zero")
		}
		return math.Mod(l, r), nil
	case tokCaret:
		return math.Pow(l, r), nil
	}
	return 0, fmt.Errorf("unknown operator")
}

func evalCall(c CallNode) (float64, error) {
	name := strings.ToLower(c.Name)
	args := make([]float64, len(c.Args))
	for i, a := range c.Args {
		v, err := Eval(a)
		if err != nil {
			return 0, err
		}
		args[i] = v
	}
	if fn, ok := funcs1[name]; ok {
		if len(args) != 1 {
			return 0, fmt.Errorf("%s expects 1 argument, got %d", name, len(args))
		}
		return fn(args[0])
	}
	if fn, ok := funcsVar[name]; ok {
		return fn(args)
	}
	return 0, fmt.Errorf("unknown function %q", c.Name)
}

// EvalString 是便捷入口：解析 + 求值。
func EvalString(expr string) (float64, error) {
	n, err := Parse(expr)
	if err != nil {
		return 0, err
	}
	return Eval(n)
}
