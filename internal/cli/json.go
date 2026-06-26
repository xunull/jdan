package cli

import (
	"bytes"
	"encoding/json"
	"fmt"
	"io"
	"os"
	"strings"

	"github.com/spf13/cobra"

	"github.com/xunull/jdan/internal/jsonx"
)

type jsonCmdDeps struct {
	out io.Writer
	in  io.Reader
}

type jsonCmdExitErr struct{ msg string }

func (e *jsonCmdExitErr) Error() string { return e.msg }

func newJSONCommand(deps jsonCmdDeps) *cobra.Command {
	if deps.out == nil {
		deps.out = os.Stdout
	}
	if deps.in == nil {
		deps.in = os.Stdin
	}
	cmd := &cobra.Command{
		Use:   "json",
		Short: "JSON 工具集 (pretty/minify/path/keys/diff/lines)",
		Long: `JSON 工具集。子命令：
  pretty | minify | path | keys | diff | lines

输入约定：file 参数 > stdin。

例：
  jdan json pretty data.json
  jdan json minify data.json --in-place
  jdan json path "users[0].name" data.json
  jdan json path /users/0/name --pointer data.json
  jdan json keys data.json --all
  jdan json diff a.json b.json
  jdan json diff a.json b.json --json --exit-code
  cat logs.jsonl | jdan json lines --count`,
	}
	cmd.AddCommand(newJSONPrettyCommand(deps))
	cmd.AddCommand(newJSONMinifyCommand(deps))
	cmd.AddCommand(newJSONPathCommand(deps))
	cmd.AddCommand(newJSONKeysCommand(deps))
	cmd.AddCommand(newJSONDiffCommand(deps))
	cmd.AddCommand(newJSONLinesCommand(deps))
	cmd.AddCommand(newJSONFromYAMLCommand(deps))
	cmd.AddCommand(newJSONToYAMLCommand(deps))
	cmd.AddCommand(newJSONFromCSVCommand(deps))
	cmd.AddCommand(newJSONToCSVCommand(deps))
	cmd.AddCommand(newJSONFlattenCommand(deps))
	cmd.AddCommand(newJSONUnflattenCommand(deps))
	return cmd
}

func newJSONFromCSVCommand(deps jsonCmdDeps) *cobra.Command {
	cmd := &cobra.Command{
		Use:           "from-csv [file]",
		Short:         "CSV → JSON array of objects (--no-header → array of arrays)",
		Args:          cobra.MaximumNArgs(1),
		SilenceUsage:  true,
		SilenceErrors: true,
		RunE: func(cmd *cobra.Command, args []string) error {
			noHeader, _ := cmd.Flags().GetBool("no-header")
			delimStr, _ := cmd.Flags().GetString("delim")
			pretty, _ := cmd.Flags().GetBool("pretty")
			data, err := readJSONInput(args, deps.in)
			if err != nil {
				return err
			}
			delim, err := pickCSVDelim(delimStr)
			if err != nil {
				return err
			}
			out, err := jsonx.CSVToJSON(data, !noHeader, delim)
			if err != nil {
				return err
			}
			if pretty {
				out, err = jsonx.Pretty(out, 2)
				if err != nil {
					return err
				}
			}
			fmt.Fprintln(deps.out, string(out))
			return nil
		},
	}
	cmd.Flags().Bool("no-header", false, "CSV 没有 header 行（输出 array of arrays）")
	cmd.Flags().String("delim", ",", "field 分隔符（\\t 表 tab）")
	cmd.Flags().Bool("pretty", true, "pretty-print 输出（默认开；--pretty=false 拿紧凑 JSON）")
	return cmd
}

func newJSONToCSVCommand(deps jsonCmdDeps) *cobra.Command {
	cmd := &cobra.Command{
		Use:           "to-csv [file]",
		Short:         "JSON array of objects → CSV",
		Args:          cobra.MaximumNArgs(1),
		SilenceUsage:  true,
		SilenceErrors: true,
		RunE: func(cmd *cobra.Command, args []string) error {
			headerCSV, _ := cmd.Flags().GetString("header")
			delimStr, _ := cmd.Flags().GetString("delim")
			data, err := readJSONInput(args, deps.in)
			if err != nil {
				return err
			}
			var headerOrder []string
			if headerCSV != "" {
				headerOrder = strings.Split(headerCSV, ",")
				for i := range headerOrder {
					headerOrder[i] = strings.TrimSpace(headerOrder[i])
				}
			}
			delim, err := pickCSVDelim(delimStr)
			if err != nil {
				return err
			}
			out, err := jsonx.JSONToCSV(data, headerOrder, delim)
			if err != nil {
				return err
			}
			fmt.Fprint(deps.out, string(out))
			return nil
		},
	}
	cmd.Flags().String("header", "", "指定列序（csv，例：\"name,age,email\"；默认按字母顺序）")
	cmd.Flags().String("delim", ",", "field 分隔符（\\t 表 tab）")
	return cmd
}

func pickCSVDelim(s string) (rune, error) {
	switch s {
	case "":
		return ',', nil
	case "\\t":
		return '\t', nil
	}
	rs := []rune(s)
	if len(rs) != 1 {
		return 0, fmt.Errorf("delimiter must be 1 char (or \\t), got %q", s)
	}
	return rs[0], nil
}

func newJSONFromYAMLCommand(deps jsonCmdDeps) *cobra.Command {
	cmd := &cobra.Command{
		Use:           "from-yaml [file]",
		Short:         "YAML → JSON",
		Args:          cobra.MaximumNArgs(1),
		SilenceUsage:  true,
		SilenceErrors: true,
		RunE: func(cmd *cobra.Command, args []string) error {
			pretty, _ := cmd.Flags().GetBool("pretty")
			indent, _ := cmd.Flags().GetInt("indent")
			data, err := readJSONInput(args, deps.in)
			if err != nil {
				return err
			}
			useIndent := 0
			if pretty {
				useIndent = indent
			}
			out, err := jsonx.YAMLToJSON(data, useIndent)
			if err != nil {
				return fmt.Errorf("yaml → json: %w", err)
			}
			fmt.Fprintln(deps.out, string(out))
			return nil
		},
	}
	cmd.Flags().Bool("pretty", true, "pretty-print 输出（默认开；--pretty=false 拿紧凑 JSON）")
	cmd.Flags().Int("indent", 2, "缩进空格数")
	return cmd
}

func newJSONToYAMLCommand(deps jsonCmdDeps) *cobra.Command {
	cmd := &cobra.Command{
		Use:           "to-yaml [file]",
		Short:         "JSON → YAML",
		Args:          cobra.MaximumNArgs(1),
		SilenceUsage:  true,
		SilenceErrors: true,
		RunE: func(cmd *cobra.Command, args []string) error {
			indent, _ := cmd.Flags().GetInt("indent")
			data, err := readJSONInput(args, deps.in)
			if err != nil {
				return err
			}
			out, err := jsonx.JSONToYAML(data, indent)
			if err != nil {
				return fmt.Errorf("json → yaml: %w", err)
			}
			// yaml.Marshal 已带末尾换行
			fmt.Fprint(deps.out, string(out))
			return nil
		},
	}
	cmd.Flags().Int("indent", 2, "YAML 缩进空格数")
	return cmd
}

func newJSONPrettyCommand(deps jsonCmdDeps) *cobra.Command {
	cmd := &cobra.Command{
		Use:           "pretty [file]",
		Short:         "Pretty-print JSON",
		Args:          cobra.MaximumNArgs(1),
		SilenceUsage:  true,
		SilenceErrors: true,
		RunE: func(cmd *cobra.Command, args []string) error {
			indent, _ := cmd.Flags().GetInt("indent")
			inPlace, _ := cmd.Flags().GetBool("in-place")
			return runJSONFormat(args, deps.in, deps.out, inPlace, func(data []byte) ([]byte, error) {
				return jsonx.Pretty(data, indent)
			})
		},
	}
	cmd.Flags().Int("indent", 2, "空格缩进数量")
	cmd.Flags().Bool("in-place", false, "原地修改文件（需要 file 参数）")
	return cmd
}

func newJSONMinifyCommand(deps jsonCmdDeps) *cobra.Command {
	cmd := &cobra.Command{
		Use:           "minify [file]",
		Short:         "压成紧凑单行 JSON",
		Args:          cobra.MaximumNArgs(1),
		SilenceUsage:  true,
		SilenceErrors: true,
		RunE: func(cmd *cobra.Command, args []string) error {
			inPlace, _ := cmd.Flags().GetBool("in-place")
			return runJSONFormat(args, deps.in, deps.out, inPlace, jsonx.Minify)
		},
	}
	cmd.Flags().Bool("in-place", false, "原地修改文件（需要 file 参数）")
	return cmd
}

func runJSONFormat(args []string, stdin io.Reader, stdout io.Writer, inPlace bool, fn func([]byte) ([]byte, error)) error {
	var (
		data []byte
		path string
		err  error
	)
	if len(args) > 0 {
		path = args[0]
		data, err = os.ReadFile(path)
	} else {
		data, err = io.ReadAll(stdin)
	}
	if err != nil {
		return err
	}
	out, err := fn(data)
	if err != nil {
		return err
	}
	if inPlace {
		if path == "" {
			return fmt.Errorf("--in-place requires a file argument")
		}
		return os.WriteFile(path, append(out, '\n'), 0o644)
	}
	fmt.Fprintln(stdout, string(out))
	return nil
}

func newJSONPathCommand(deps jsonCmdDeps) *cobra.Command {
	cmd := &cobra.Command{
		Use:           "path <expr> [file]",
		Short:         "按 path 取值 (dot-path 默认；--pointer 切到 RFC 6901)",
		Args:          cobra.RangeArgs(1, 2),
		SilenceUsage:  true,
		SilenceErrors: true,
		RunE: func(cmd *cobra.Command, args []string) error {
			usePtr, _ := cmd.Flags().GetBool("pointer")
			raw, _ := cmd.Flags().GetBool("raw")
			expr := args[0]
			data, err := readJSONInput(args[1:], deps.in)
			if err != nil {
				return err
			}
			v, err := jsonx.DecodeValue(data)
			if err != nil {
				return fmt.Errorf("parse JSON: %w", err)
			}
			var segs []jsonx.Segment
			if usePtr {
				segs, err = jsonx.ParsePointer(expr)
			} else {
				segs, err = jsonx.ParsePath(expr)
			}
			if err != nil {
				return err
			}
			got, err := jsonx.Get(v, segs)
			if err != nil {
				return err
			}
			return printJSONValue(deps.out, got, raw)
		},
	}
	cmd.Flags().Bool("pointer", false, "用 RFC 6901 JSON Pointer 语法（/foo/0/bar）")
	cmd.Flags().BoolP("raw", "r", false, "字符串结果不带引号输出")
	return cmd
}

func readJSONInput(fileArgs []string, stdin io.Reader) ([]byte, error) {
	if len(fileArgs) > 0 {
		return os.ReadFile(fileArgs[0])
	}
	return io.ReadAll(stdin)
}

func printJSONValue(w io.Writer, v any, raw bool) error {
	if raw {
		if s, ok := v.(string); ok {
			fmt.Fprintln(w, s)
			return nil
		}
	}
	// 标量类型：单行紧凑输出（数字/布尔/null）
	switch v.(type) {
	case map[string]any, []any:
		out, err := json.MarshalIndent(v, "", "  ")
		if err != nil {
			return err
		}
		fmt.Fprintln(w, string(out))
	default:
		out, err := json.Marshal(v)
		if err != nil {
			return err
		}
		fmt.Fprintln(w, string(out))
	}
	return nil
}

func newJSONKeysCommand(deps jsonCmdDeps) *cobra.Command {
	cmd := &cobra.Command{
		Use:           "keys [file]",
		Short:         "列出 key（默认顶层；--all 递归所有路径）",
		Args:          cobra.MaximumNArgs(1),
		SilenceUsage:  true,
		SilenceErrors: true,
		RunE: func(cmd *cobra.Command, args []string) error {
			all, _ := cmd.Flags().GetBool("all")
			depth, _ := cmd.Flags().GetInt("depth")
			data, err := readJSONInput(args, deps.in)
			if err != nil {
				return err
			}
			v, err := jsonx.DecodeValue(data)
			if err != nil {
				return fmt.Errorf("parse JSON: %w", err)
			}
			ks, err := jsonx.Keys(v, all, depth)
			if err != nil {
				return err
			}
			for _, k := range ks {
				fmt.Fprintln(deps.out, k)
			}
			return nil
		},
	}
	cmd.Flags().Bool("all", false, "递归列出所有叶子 path（dot-path 风格）")
	cmd.Flags().Int("depth", 0, "最大深度（0 = 无限；仅 --all 生效）")
	return cmd
}

func newJSONDiffCommand(deps jsonCmdDeps) *cobra.Command {
	cmd := &cobra.Command{
		Use:           "diff <a.json> <b.json>",
		Short:         "语义 JSON diff（--json 输出 RFC 6902 patch）",
		Args:          cobra.ExactArgs(2),
		SilenceUsage:  true,
		SilenceErrors: true,
		RunE: func(cmd *cobra.Command, args []string) error {
			asJSON, _ := cmd.Flags().GetBool("json")
			exitCode, _ := cmd.Flags().GetBool("exit-code")
			ad, err := os.ReadFile(args[0])
			if err != nil {
				return err
			}
			bd, err := os.ReadFile(args[1])
			if err != nil {
				return err
			}
			a, err := jsonx.DecodeValue(ad)
			if err != nil {
				return fmt.Errorf("parse %s: %w", args[0], err)
			}
			b, err := jsonx.DecodeValue(bd)
			if err != nil {
				return fmt.Errorf("parse %s: %w", args[1], err)
			}
			entries := jsonx.Diff(a, b)
			if asJSON {
				// RFC 6902 不要 old 字段，但保留 old 便于人类阅读 + IDE。
				out, err := json.MarshalIndent(entries, "", "  ")
				if err != nil {
					return err
				}
				fmt.Fprintln(deps.out, string(out))
			} else {
				for _, e := range entries {
					renderDiffEntry(deps.out, e)
				}
				if len(entries) == 0 {
					fmt.Fprintln(deps.out, "(identical)")
				}
			}
			if exitCode && len(entries) > 0 {
				return &jsonCmdExitErr{msg: fmt.Sprintf("%d difference(s)", len(entries))}
			}
			return nil
		},
	}
	cmd.Flags().Bool("json", false, "输出 RFC 6902 JSON Patch 格式")
	cmd.Flags().Bool("exit-code", false, "有差异时非零退出码（CI gate）")
	return cmd
}

func renderDiffEntry(w io.Writer, e jsonx.DiffEntry) {
	switch e.Op {
	case jsonx.OpAdd:
		fmt.Fprintf(w, "+ %s = %s\n", e.Path, jsonInline(e.NewValue))
	case jsonx.OpRemove:
		fmt.Fprintf(w, "- %s = %s\n", e.Path, jsonInline(e.OldValue))
	case jsonx.OpReplace:
		fmt.Fprintf(w, "~ %s: %s -> %s\n", e.Path, jsonInline(e.OldValue), jsonInline(e.NewValue))
	}
}

func jsonInline(v any) string {
	b, _ := json.Marshal(v)
	return string(b)
}

func newJSONLinesCommand(deps jsonCmdDeps) *cobra.Command {
	cmd := &cobra.Command{
		Use:           "lines [file]",
		Short:         "JSONL（一行一个 JSON）工具",
		Args:          cobra.MaximumNArgs(1),
		SilenceUsage:  true,
		SilenceErrors: true,
		RunE: func(cmd *cobra.Command, args []string) error {
			count, _ := cmd.Flags().GetBool("count")
			get, _ := cmd.Flags().GetInt("get")
			head, _ := cmd.Flags().GetInt("head")
			// 三个模式互斥
			modes := 0
			if count {
				modes++
			}
			if cmd.Flags().Changed("get") {
				modes++
			}
			if cmd.Flags().Changed("head") {
				modes++
			}
			if modes > 1 {
				return fmt.Errorf("--count / --get / --head 互斥，只能传一个")
			}
			r, closer, err := openJSONLReader(args, deps.in)
			if err != nil {
				return err
			}
			if closer != nil {
				defer closer.Close()
			}
			switch {
			case count:
				n, err := jsonx.LinesCount(r)
				if err != nil {
					return err
				}
				fmt.Fprintln(deps.out, n)
			case cmd.Flags().Changed("get"):
				line, err := jsonx.LinesGet(r, get)
				if err != nil {
					return err
				}
				fmt.Fprintln(deps.out, string(line))
			case cmd.Flags().Changed("head"):
				lines, err := jsonx.LinesHead(r, head)
				if err != nil {
					return err
				}
				for _, l := range lines {
					fmt.Fprintln(deps.out, string(l))
				}
			default:
				n, err := jsonx.LinesCount(r)
				if err != nil {
					return err
				}
				fmt.Fprintf(deps.out, "%d valid JSONL records\n", n)
			}
			return nil
		},
	}
	cmd.Flags().Bool("count", false, "只计数（校验每行 valid）")
	cmd.Flags().Int("get", -1, "返回第 N 行（0-based）")
	cmd.Flags().Int("head", 0, "返回前 N 行")
	return cmd
}

func openJSONLReader(args []string, stdin io.Reader) (io.Reader, io.Closer, error) {
	if len(args) > 0 {
		f, err := os.Open(args[0])
		if err != nil {
			return nil, nil, err
		}
		return f, f, nil
	}
	// 用 bytes.NewReader 读完整 stdin —— LinesCount 等只 scan 一遍
	data, err := io.ReadAll(stdin)
	if err != nil {
		return nil, nil, err
	}
	return bytes.NewReader(data), nil, nil
}

func init() {
	rootCmd.AddCommand(newJSONCommand(jsonCmdDeps{}))
}
