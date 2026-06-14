package cli

import (
	"io"
	"net/url"
	"os"
	"strings"

	"github.com/spf13/cobra"
)

type urlCmdDeps struct {
	out io.Writer
	in  io.Reader
}

func newURLCommand(deps urlCmdDeps) *cobra.Command {
	if deps.out == nil {
		deps.out = os.Stdout
	}
	if deps.in == nil {
		deps.in = os.Stdin
	}
	cmd := &cobra.Command{
		Use:   "url",
		Short: "URL percent-encoding / decoding",
		Long: `URL 编码/解码（RFC 3986 percent-encoding）。

例：
  jdan url enc "hello world"           # → hello%20world
  jdan url dec "hello%20world"         # → hello world
  echo "$VAR" | jdan url enc           # stdin
  jdan url enc "a&b=c" --query         # query string 编码（+号代空格）
  jdan url enc "/path with space" --path # path 编码（保留 /）`,
	}
	cmd.AddCommand(newURLEncodeCommand(deps))
	cmd.AddCommand(newURLDecodeCommand(deps))
	return cmd
}

func newURLEncodeCommand(deps urlCmdDeps) *cobra.Command {
	cmd := &cobra.Command{
		Use:   "enc [text]",
		Short: "URL percent-encode",
		Args:  cobra.MaximumNArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			return runURLEncode(cmd, args, deps.out, deps.in)
		},
	}
	cmd.Flags().Bool("query", false, "query string 编码（+ 代空格而不是 %20）")
	cmd.Flags().Bool("path", false, "path 编码（保留 /，比 query 编码更宽松）")
	cmd.Flags().StringP("input", "i", "", "从文件读输入")
	cmd.Flags().StringP("output", "o", "", "写到文件而不是 stdout")
	cmd.Flags().Bool("no-newline", false, "不在输出末尾加换行")
	return cmd
}

func newURLDecodeCommand(deps urlCmdDeps) *cobra.Command {
	cmd := &cobra.Command{
		Use:   "dec [text]",
		Short: "URL percent-decode",
		Args:  cobra.MaximumNArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			return runURLDecode(cmd, args, deps.out, deps.in)
		},
	}
	cmd.Flags().Bool("query", false, "query string 解码（+ 解成空格）")
	cmd.Flags().StringP("input", "i", "", "从文件读输入")
	cmd.Flags().StringP("output", "o", "", "写到文件而不是 stdout")
	cmd.Flags().Bool("no-newline", false, "不在输出末尾加换行")
	return cmd
}

func runURLEncode(cmd *cobra.Command, args []string, out io.Writer, stdin io.Reader) error {
	queryMode, _ := cmd.Flags().GetBool("query")
	pathMode, _ := cmd.Flags().GetBool("path")
	inputFile, _ := cmd.Flags().GetString("input")
	outputFile, _ := cmd.Flags().GetString("output")
	noNewline, _ := cmd.Flags().GetBool("no-newline")

	data, err := readInput(args, inputFile, stdin)
	if err != nil {
		return err
	}
	input := string(data)
	// stdin 末尾换行常常是无意的，trim 掉
	if len(args) == 0 && inputFile == "" {
		input = strings.TrimRight(input, "\r\n")
	}

	var encoded string
	switch {
	case queryMode:
		encoded = url.QueryEscape(input)
	case pathMode:
		encoded = url.PathEscape(input)
	default:
		encoded = url.PathEscape(input)
	}

	return writeOutputString(encoded, outputFile, out, !noNewline)
}

func runURLDecode(cmd *cobra.Command, args []string, out io.Writer, stdin io.Reader) error {
	queryMode, _ := cmd.Flags().GetBool("query")
	inputFile, _ := cmd.Flags().GetString("input")
	outputFile, _ := cmd.Flags().GetString("output")
	noNewline, _ := cmd.Flags().GetBool("no-newline")

	data, err := readInput(args, inputFile, stdin)
	if err != nil {
		return err
	}
	input := strings.TrimRight(string(data), "\r\n")

	var decoded string
	if queryMode {
		decoded, err = url.QueryUnescape(input)
	} else {
		decoded, err = url.PathUnescape(input)
	}
	if err != nil {
		return err
	}

	return writeOutputString(decoded, outputFile, out, !noNewline)
}

func init() {
	rootCmd.AddCommand(newURLCommand(urlCmdDeps{}))
}
