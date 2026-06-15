package cli

import (
	"encoding/json"
	"fmt"
	"io"
	"os"

	"github.com/spf13/cobra"

	"github.com/xunull/jdan/internal/dotenv"
)

type envCmdDeps struct {
	out io.Writer
}

type envCmdExitErr struct{ msg string }

func (e *envCmdExitErr) Error() string { return e.msg }

func newEnvCommand(deps envCmdDeps) *cobra.Command {
	if deps.out == nil {
		deps.out = os.Stdout
	}
	cmd := &cobra.Command{
		Use:   "env",
		Short: ".env 文件工具（lint / diff / redact / get）",
		Long: `.env 文件检查工具。偏"检查 / 对比 / 脱敏"，不做加载（那是 direnv / dotenv-cli）。
0 新依赖。

子命令：
  lint <file>          检查问题（重复 key / 未引号空格 / 非法名 / 尾空格 / CRLF / BOM）
  diff <a> <b>         对比两个 .env 的 key 差异（部署前查漏）
  redact <file>        脱敏 value 以便分享（key 保留，value 打码）
  get <file> <key>     取单个 key 的 value（正确处理引号 / export / 行内注释）

例：
  jdan env lint .env
  jdan env diff .env.example .env       # 部署前确保没缺 key
  jdan env redact .env | pbcopy         # 贴 issue 前脱敏
  jdan env get .env DATABASE_URL`,
	}
	cmd.AddCommand(newEnvLintCommand(deps))
	cmd.AddCommand(newEnvDiffCommand(deps))
	cmd.AddCommand(newEnvRedactCommand(deps))
	cmd.AddCommand(newEnvGetCommand(deps))
	return cmd
}

func parseEnvFile(path string) (*dotenv.File, error) {
	fh, err := os.Open(path)
	if err != nil {
		return nil, err
	}
	defer fh.Close()
	return dotenv.Parse(fh)
}

// ---- lint ----

func newEnvLintCommand(deps envCmdDeps) *cobra.Command {
	cmd := &cobra.Command{
		Use:           "lint <file>",
		Short:         "检查 .env 常见问题",
		Args:          cobra.ExactArgs(1),
		SilenceUsage:  true,
		SilenceErrors: true,
		RunE: func(cmd *cobra.Command, args []string) error {
			strict, _ := cmd.Flags().GetBool("strict")
			asJSON, _ := cmd.Flags().GetBool("json")
			return runEnvLint(args[0], strict, asJSON, deps.out)
		},
	}
	cmd.Flags().Bool("strict", false, "warning 也算失败（退出码 1）")
	cmd.Flags().Bool("json", false, "结构化 JSON 输出")
	return cmd
}

func runEnvLint(path string, strict, asJSON bool, out io.Writer) error {
	f, err := parseEnvFile(path)
	if err != nil {
		return err
	}
	issues := dotenv.Lint(f)
	errs, warns := dotenv.CountBySeverity(issues)

	if asJSON {
		enc := json.NewEncoder(out)
		enc.SetIndent("", "  ")
		_ = enc.Encode(map[string]any{
			"file":     path,
			"issues":   issues,
			"errors":   errs,
			"warnings": warns,
		})
	} else {
		for _, i := range issues {
			fmt.Fprintf(out, "%s:%d  %-7s  %s\n", path, i.Line, i.Severity, i.Message)
		}
		if len(issues) == 0 {
			fmt.Fprintln(out, "no issues")
		} else {
			fmt.Fprintf(out, "\n%d issues (%d errors, %d warnings)\n", len(issues), errs, warns)
		}
	}

	if errs > 0 || (strict && warns > 0) {
		return &envCmdExitErr{msg: fmt.Sprintf("%d error(s), %d warning(s)", errs, warns)}
	}
	return nil
}

// ---- diff ----

func newEnvDiffCommand(deps envCmdDeps) *cobra.Command {
	cmd := &cobra.Command{
		Use:           "diff <a> <b>",
		Short:         "对比两个 .env 的 key 差异（默认只比 key）",
		Args:          cobra.ExactArgs(2),
		SilenceUsage:  true,
		SilenceErrors: true,
		RunE: func(cmd *cobra.Command, args []string) error {
			withValues, _ := cmd.Flags().GetBool("values")
			asJSON, _ := cmd.Flags().GetBool("json")
			exitCode, _ := cmd.Flags().GetBool("exit-code")
			return runEnvDiff(args[0], args[1], withValues, asJSON, exitCode, deps.out)
		},
	}
	cmd.Flags().Bool("values", false, "也对比公共 key 的 value（默认只比 key）")
	cmd.Flags().Bool("json", false, "结构化 JSON 输出")
	cmd.Flags().Bool("exit-code", false, "有差异时退出码 1（CI gate）")
	return cmd
}

func runEnvDiff(aPath, bPath string, withValues, asJSON, exitCode bool, out io.Writer) error {
	a, err := parseEnvFile(aPath)
	if err != nil {
		return err
	}
	b, err := parseEnvFile(bPath)
	if err != nil {
		return err
	}
	res := dotenv.Diff(a, b, withValues)

	if asJSON {
		enc := json.NewEncoder(out)
		enc.SetIndent("", "  ")
		_ = enc.Encode(res)
	} else {
		if len(res.OnlyInA) > 0 {
			fmt.Fprintf(out, "Only in %s (%d):\n", aPath, len(res.OnlyInA))
			for _, k := range res.OnlyInA {
				fmt.Fprintf(out, "  + %s\n", k)
			}
		}
		if len(res.OnlyInB) > 0 {
			fmt.Fprintf(out, "Only in %s (%d):\n", bPath, len(res.OnlyInB))
			for _, k := range res.OnlyInB {
				fmt.Fprintf(out, "  - %s\n", k)
			}
		}
		if withValues && len(res.ValueDiff) > 0 {
			fmt.Fprintf(out, "Value differs (%d):\n", len(res.ValueDiff))
			for _, d := range res.ValueDiff {
				fmt.Fprintf(out, "  ~ %s\n", d.Key)
			}
		}
		fmt.Fprintf(out, "Common keys: %d\n", len(res.Common))
	}

	if exitCode && res.HasDifferences() {
		return &envCmdExitErr{msg: "env files differ"}
	}
	return nil
}

// ---- redact ----

func newEnvRedactCommand(deps envCmdDeps) *cobra.Command {
	cmd := &cobra.Command{
		Use:           "redact <file>",
		Short:         "脱敏 value 以便安全分享",
		Args:          cobra.ExactArgs(1),
		SilenceUsage:  true,
		SilenceErrors: true,
		RunE: func(cmd *cobra.Command, args []string) error {
			full, _ := cmd.Flags().GetBool("full")
			keepShort, _ := cmd.Flags().GetBool("keep-short")
			return runEnvRedact(args[0], full, keepShort, deps.out)
		},
	}
	cmd.Flags().Bool("full", false, "完全打码（****，不保留首尾）")
	cmd.Flags().Bool("keep-short", false, "短值（<=4）/ 布尔类不打码")
	return cmd
}

func runEnvRedact(path string, full, keepShort bool, out io.Writer) error {
	f, err := parseEnvFile(path)
	if err != nil {
		return err
	}
	opts := dotenv.RedactOpts{Full: full, KeepShort: keepShort}
	for _, e := range f.Entries {
		fmt.Fprintln(out, dotenv.RedactLine(e, opts))
	}
	return nil
}

// ---- get ----

func newEnvGetCommand(deps envCmdDeps) *cobra.Command {
	cmd := &cobra.Command{
		Use:           "get <file> <key>",
		Short:         "取单个 key 的 value",
		Args:          cobra.ExactArgs(2),
		SilenceUsage:  true,
		SilenceErrors: true,
		RunE: func(cmd *cobra.Command, args []string) error {
			f, err := parseEnvFile(args[0])
			if err != nil {
				return err
			}
			val, err := dotenv.Get(f, args[1])
			if err != nil {
				return err
			}
			fmt.Fprintln(deps.out, val)
			return nil
		},
	}
	return cmd
}

func init() {
	rootCmd.AddCommand(newEnvCommand(envCmdDeps{}))
}
