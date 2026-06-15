package cli

import (
	"encoding/json"
	"fmt"
	"io"
	"os"
	"strings"

	"github.com/spf13/cobra"

	"github.com/xunull/jdan/internal/numconv"
)

type numCmdDeps struct {
	out io.Writer
}

func newNumCommand(deps numCmdDeps) *cobra.Command {
	if deps.out == nil {
		deps.out = os.Stdout
	}
	cmd := &cobra.Command{
		Use:   "num <value>",
		Short: "进制转换 + 位运算（dec/hex/bin/oct）",
		Long: `进制转换工具。自动检测输入进制（0x/0b/0o/前导0/十进制），
一次性输出全部进制 + 位信息。uint64 范围，0 新依赖。

例：
  jdan num 255                  # → hex 0xFF / bin 0b11111111 / oct 0o377
  jdan num 0xDEADBEEF           # 十六进制输入
  jdan num 0b10110 --bits       # 位展示（看 flag / mask）
  jdan num 0xFF --width 16      # 二进制零填充到 16 位
  jdan num 255 --json

位运算用子命令：
  jdan num bit "0xFF AND 0x0F"  # → 0x0F
  jdan num bit "1 << 8"         # → 0x100
  jdan num bit "NOT 0xFF"       # 单目取反（--width 控制位宽）`,
		Args:          cobra.ExactArgs(1),
		SilenceUsage:  true,
		SilenceErrors: true,
		RunE: func(cmd *cobra.Command, args []string) error {
			showBits, _ := cmd.Flags().GetBool("bits")
			width, _ := cmd.Flags().GetInt("width")
			asJSON, _ := cmd.Flags().GetBool("json")
			return runNum(args[0], showBits, width, asJSON, deps.out)
		},
	}
	cmd.Flags().Bool("bits", false, "显示位编号 + 值（看 flag / mask）")
	cmd.Flags().Int("width", 0, "二进制输出零填充到 N 位")
	cmd.Flags().Bool("json", false, "结构化 JSON 输出")
	cmd.AddCommand(newNumBitCommand(deps))
	return cmd
}

func runNum(input string, showBits bool, width int, asJSON bool, out io.Writer) error {
	v, base, err := numconv.ParseValue(input)
	if err != nil {
		return err
	}
	res := numconv.Convert(v)

	if asJSON {
		enc := json.NewEncoder(out)
		enc.SetIndent("", "  ")
		return enc.Encode(map[string]any{
			"input":          input,
			"detected_base":  base.String(),
			"decimal":        res.Decimal,
			"hex":            res.Hex,
			"binary":         res.Binary,
			"octal":          res.Octal,
			"bits_set":       res.BitsSet,
			"bit_width":      res.BitWidth,
		})
	}

	binary := res.Binary
	if width > 0 {
		binary = numconv.BinaryPadded(v, width)
	}
	fmt.Fprintf(out, "Decimal:  %d\n", res.Decimal)
	fmt.Fprintf(out, "Hex:      %s\n", res.Hex)
	fmt.Fprintf(out, "Binary:   %s\n", binary)
	fmt.Fprintf(out, "Octal:    %s\n", res.Octal)

	setBits := numconv.SetBits(v)
	setStr := "none"
	if len(setBits) > 0 {
		parts := make([]string, len(setBits))
		for i, b := range setBits {
			parts[i] = fmt.Sprintf("%d", b)
		}
		setStr = strings.Join(parts, ",")
	}
	fmt.Fprintf(out, "Bits:     %d set (%s), width %d\n", res.BitsSet, setStr, res.BitWidth)

	if showBits {
		fmt.Fprintln(out, "          "+strings.ReplaceAll(numconv.BitRows(v, width), "\n", "\n          "))
	}
	return nil
}

// ---- bit 子命令 ----

func newNumBitCommand(deps numCmdDeps) *cobra.Command {
	cmd := &cobra.Command{
		Use:           "bit <expr>",
		Short:         `位运算（"a AND b" / "1 << 8" / "NOT 0xFF"）`,
		Args:          cobra.MinimumNArgs(1),
		SilenceUsage:  true,
		SilenceErrors: true,
		RunE: func(cmd *cobra.Command, args []string) error {
			width, _ := cmd.Flags().GetInt("width")
			asJSON, _ := cmd.Flags().GetBool("json")
			// 允许 "0xFF AND 0x0F" 作为多个 arg 传入，也允许整个加引号
			expr := strings.Join(args, " ")
			return runNumBit(expr, width, asJSON, deps.out)
		},
	}
	cmd.Flags().Int("width", 64, "NOT 取反的位宽")
	cmd.Flags().Bool("json", false, "结构化 JSON 输出")
	return cmd
}

func runNumBit(expr string, width int, asJSON bool, out io.Writer) error {
	r, err := numconv.EvalBitExpr(expr, width)
	if err != nil {
		return err
	}
	res := numconv.Convert(r.Result)
	if asJSON {
		enc := json.NewEncoder(out)
		enc.SetIndent("", "  ")
		payload := map[string]any{
			"op":      string(r.Op),
			"a":       r.A,
			"result":  res.Decimal,
			"hex":     res.Hex,
			"binary":  res.Binary,
		}
		if r.Op != numconv.OpNOT {
			payload["b"] = r.B
		}
		return enc.Encode(payload)
	}
	// 人类输出：0xRESULT  (dec, 0bbin)
	fmt.Fprintf(out, "%s  (%d, %s)\n", res.Hex, res.Decimal, res.Binary)
	return nil
}

func init() {
	rootCmd.AddCommand(newNumCommand(numCmdDeps{}))
}
