package calc

import (
	"math"
	"testing"
)

func evalOK(t *testing.T, expr string, want float64) {
	t.Helper()
	got, err := EvalString(expr)
	if err != nil {
		t.Fatalf("EvalString(%q) errored: %v", expr, err)
	}
	if math.Abs(got-want) > 1e-9 {
		t.Errorf("EvalString(%q) = %v, want %v", expr, got, want)
	}
}

func evalErr(t *testing.T, expr string) {
	t.Helper()
	if _, err := EvalString(expr); err == nil {
		t.Errorf("EvalString(%q) should error", expr)
	}
}

// ---- 基本算术 ----

func TestEval_Basic(t *testing.T) {
	evalOK(t, "1 + 2", 3)
	evalOK(t, "10 - 4", 6)
	evalOK(t, "3 * 4", 12)
	evalOK(t, "10 / 4", 2.5)
	evalOK(t, "10 % 3", 1)
	evalOK(t, "2 ^ 10", 1024)
}

// ---- 优先级 ----

func TestEval_Precedence(t *testing.T) {
	evalOK(t, "2 + 3 * 4", 14)   // * 高于 +
	evalOK(t, "(2 + 3) * 4", 20) // 括号
	evalOK(t, "2 * 3 + 4", 10)
	evalOK(t, "100 / 10 / 2", 5) // / 左结合
	evalOK(t, "2 ^ 3 ^ 2", 512)  // ^ 右结合：2^(3^2)=2^9
	evalOK(t, "2 ^ 2 * 3", 12)   // ^ 高于 *
}

// ---- 一元负号 ----

func TestEval_Unary(t *testing.T) {
	evalOK(t, "-5", -5)
	evalOK(t, "-5 + 3", -2)
	evalOK(t, "2 * -3", -6)
	evalOK(t, "-(2 + 3)", -5)
	evalOK(t, "--5", 5) // 双重负号
	evalOK(t, "2 ^ -1", 0.5)
}

// ---- ** 别名 ----

func TestEval_StarStarPower(t *testing.T) {
	evalOK(t, "2 ** 8", 256)
}

// ---- 进制操作数 ----

func TestEval_BaseOperands(t *testing.T) {
	evalOK(t, "0xFF + 1", 256)
	evalOK(t, "0b1010 * 2", 20)
	evalOK(t, "0o755", 493)
	evalOK(t, "0xFF + 0x01", 256)
}

// ---- 小数 / 科学计数 ----

func TestEval_Decimals(t *testing.T) {
	evalOK(t, "1.5 + 2.5", 4)
	evalOK(t, "0.1 + 0.2", 0.3)
	evalOK(t, "1e3", 1000)
	evalOK(t, "1.5e2", 150)
	evalOK(t, "2e-2", 0.02)
}

// ---- 函数 ----

func TestEval_Functions(t *testing.T) {
	evalOK(t, "sqrt(2)", math.Sqrt2)
	evalOK(t, "sqrt(16)", 4)
	evalOK(t, "abs(-5)", 5)
	evalOK(t, "floor(3.7)", 3)
	evalOK(t, "ceil(3.2)", 4)
	evalOK(t, "round(3.5)", 4)
	evalOK(t, "min(3, 7, 2)", 2)
	evalOK(t, "max(3, 7, 2)", 7)
	evalOK(t, "ln(e)", 1)
	evalOK(t, "log10(1000)", 3)
}

func TestEval_FunctionsCaseInsensitive(t *testing.T) {
	evalOK(t, "SQRT(4)", 2)
	evalOK(t, "Abs(-3)", 3)
}

func TestEval_NestedFunctions(t *testing.T) {
	evalOK(t, "sqrt(abs(-16))", 4)
	evalOK(t, "max(sqrt(4), 1)", 2)
}

// ---- 常量 ----

func TestEval_Constants(t *testing.T) {
	evalOK(t, "pi", math.Pi)
	evalOK(t, "e", math.E)
	evalOK(t, "tau", 2*math.Pi)
	evalOK(t, "pi * 2", 2*math.Pi)
	evalOK(t, "PI", math.Pi) // case-insensitive
}

// ---- 错误 ----

func TestEval_Errors(t *testing.T) {
	evalErr(t, "1 +")        // 缺操作数
	evalErr(t, "1 / 0")      // 除零
	evalErr(t, "1 % 0")      // 模零
	evalErr(t, "2 @ 3")      // 非法字符
	evalErr(t, "(1 + 2")     // 括号不匹配
	evalErr(t, "1 + 2)")     // 多余右括号
	evalErr(t, "sqrt(-1)")   // sqrt 负数
	evalErr(t, "ln(0)")      // ln 非正
	evalErr(t, "foo(2)")     // 未知函数
	evalErr(t, "nope")       // 未知常量
	evalErr(t, "")           // 空表达式
	evalErr(t, "   ")        // 空白
	evalErr(t, "1 2")        // 两个数字无运算符
	evalErr(t, "sqrt(1, 2)") // 单参函数给两个参数
	evalErr(t, "0x")         // 缺进制数字
}

// ---- Parse 独立 ----

func TestParse_BuildsAST(t *testing.T) {
	n, err := Parse("1 + 2")
	if err != nil {
		t.Fatal(err)
	}
	bin, ok := n.(BinNode)
	if !ok {
		t.Fatalf("expected BinNode, got %T", n)
	}
	if bin.Op != tokPlus {
		t.Errorf("op = %v", bin.Op)
	}
}

func TestParse_EmptyErrors(t *testing.T) {
	if _, err := Parse(""); err == nil {
		t.Error("empty should error")
	}
}

// ---- tokenize 边界 ----

func TestTokenize_Whitespace(t *testing.T) {
	evalOK(t, "  1  +  2  ", 3)
	evalOK(t, "1+2", 3) // 无空格
}
