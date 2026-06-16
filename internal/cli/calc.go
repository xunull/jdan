package cli

import (
	"encoding/json"
	"fmt"
	"io"
	"math"
	"os"
	"strconv"
	"strings"

	"github.com/spf13/cobra"

	"github.com/xunull/jdan/internal/calc"
)

type calcCmdDeps struct {
	out io.Writer
	in  io.Reader
}

// calcOpts 是手动解析出的 calc flag。
type calcOpts struct {
	hex       bool
	bin       bool
	json      bool
	precision int // -1 = 智能
}

func newCalcCommand(deps calcCmdDeps) *cobra.Command {
	if deps.out == nil {
		deps.out = os.Stdout
	}
	if deps.in == nil {
		deps.in = os.Stdin
	}
	cmd := &cobra.Command{
		Use:   "calc [expr]",
		Short: "算术表达式计算器（+ - * / % ^ + 函数）",
		Long: `命令行算术表达式求值。0 新依赖（手写递归下降 parser）。

运算：+ - * / %（取模）^（幂，右结合，也接受 **）+ 括号 + 一元负号
函数：sqrt abs floor ceil round ln log10 sin cos tan min max
常量：pi e tau
进制操作数：0xFF / 0b1010 / 0o755

位运算（AND/OR/XOR/shift）归 jdan num bit，本命令不做。

flags: --hex / --bin（整数结果）/ --precision N / --json

例：
  jdan calc "3 * (4 + 5) / 2"      # 13.5
  jdan calc "2 ^ 10"               # 1024
  jdan calc "-5 + 3"               # 表达式可以负号开头
  jdan calc "sqrt(2)"              # 1.4142135623730951
  jdan calc "255 + 1" --hex        # 0x100
  echo "1 + 2 * 3" | jdan calc     # stdin`,
		// 表达式常以 '-' 开头（一元负号），会被 cobra 误当 flag，所以
		// 关掉 cobra 的 flag 解析，自己分离 flag 和表达式。
		DisableFlagParsing: true,
		SilenceUsage:       true,
		SilenceErrors:      true,
		RunE: func(cmd *cobra.Command, args []string) error {
			// help 仍然要工作
			for _, a := range args {
				if a == "-h" || a == "--help" {
					return cmd.Help()
				}
			}
			opts, exprParts, err := parseCalcArgs(args)
			if err != nil {
				return err
			}
			return runCalc(exprParts, deps, opts)
		},
	}
	return cmd
}

// parseCalcArgs 手动分离 flag 和表达式 token。识别的 flag：
// --hex --bin --json --precision N（或 --precision=N）。其余全是表达式。
func parseCalcArgs(args []string) (calcOpts, []string, error) {
	opts := calcOpts{precision: -1}
	var expr []string
	for i := 0; i < len(args); i++ {
		a := args[i]
		switch {
		case a == "--hex":
			opts.hex = true
		case a == "--bin":
			opts.bin = true
		case a == "--json":
			opts.json = true
		case a == "--precision":
			if i+1 >= len(args) {
				return opts, nil, fmt.Errorf("--precision requires a value")
			}
			i++
			n, err := strconv.Atoi(args[i])
			if err != nil || n < 0 {
				return opts, nil, fmt.Errorf("--precision must be a non-negative integer")
			}
			opts.precision = n
		case strings.HasPrefix(a, "--precision="):
			n, err := strconv.Atoi(strings.TrimPrefix(a, "--precision="))
			if err != nil || n < 0 {
				return opts, nil, fmt.Errorf("--precision must be a non-negative integer")
			}
			opts.precision = n
		default:
			expr = append(expr, a)
		}
	}
	if opts.hex && opts.bin {
		return opts, nil, fmt.Errorf("--hex and --bin are mutually exclusive")
	}
	return opts, expr, nil
}

func runCalc(exprParts []string, deps calcCmdDeps, opts calcOpts) error {
	var expr string
	if len(exprParts) > 0 {
		expr = strings.Join(exprParts, " ")
	} else {
		data, err := io.ReadAll(deps.in)
		if err != nil {
			return err
		}
		expr = string(data)
	}
	expr = strings.TrimSpace(expr)
	if expr == "" {
		return fmt.Errorf("no expression (pass as args or stdin)")
	}

	result, err := calc.EvalString(expr)
	if err != nil {
		return err
	}

	if opts.json {
		enc := json.NewEncoder(deps.out)
		enc.SetIndent("", "  ")
		return enc.Encode(map[string]any{
			"expr":   expr,
			"result": result,
		})
	}

	out, err := formatCalcResult(result, opts.hex, opts.bin, opts.precision)
	if err != nil {
		return err
	}
	fmt.Fprintln(deps.out, out)
	return nil
}

// formatCalcResult 智能格式化结果。
//   - --hex / --bin：要求结果是非负整数（在 uint64 范围）
//   - --precision N：固定小数位
//   - 默认：整数显示成整数，小数用最短往返表示
func formatCalcResult(v float64, hexOut, binOut bool, precision int) (string, error) {
	if hexOut || binOut {
		if v != math.Trunc(v) || math.IsInf(v, 0) || math.IsNaN(v) {
			return "", fmt.Errorf("hex/bin output requires an integer result, got %v", v)
		}
		if v < 0 {
			return "", fmt.Errorf("hex/bin output requires a non-negative result, got %v", v)
		}
		if v > math.MaxUint64 {
			return "", fmt.Errorf("result %v exceeds uint64 range for hex/bin output", v)
		}
		u := uint64(v)
		if hexOut {
			return "0x" + strings.ToUpper(strconv.FormatUint(u, 16)), nil
		}
		return "0b" + strconv.FormatUint(u, 2), nil
	}

	if precision >= 0 {
		return strconv.FormatFloat(v, 'f', precision, 64), nil
	}

	// 智能：整数值显示成整数（不带 .0）
	if v == math.Trunc(v) && !math.IsInf(v, 0) && math.Abs(v) < 1e15 {
		return strconv.FormatInt(int64(v), 10), nil
	}
	return strconv.FormatFloat(v, 'g', -1, 64), nil
}

func init() {
	rootCmd.AddCommand(newCalcCommand(calcCmdDeps{}))
}
