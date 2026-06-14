package cli

import (
	"encoding/base64"
	"errors"
	"fmt"
	"io"
	"os"
	"strings"

	"github.com/spf13/cobra"
)

type b64CmdDeps struct {
	out io.Writer
	in  io.Reader
}

func newB64Command(deps b64CmdDeps) *cobra.Command {
	if deps.out == nil {
		deps.out = os.Stdout
	}
	if deps.in == nil {
		deps.in = os.Stdin
	}
	cmd := &cobra.Command{
		Use:   "b64",
		Short: "base64 编码/解码（standard / URL-safe / no-padding）",
		Long: `base64 编码/解码工具。支持 standard / URL-safe 字母表 + 可选 padding。

例：
  jdan b64 enc "hello"                # standard base64
  jdan b64 dec "aGVsbG8="
  echo "hi" | jdan b64 enc            # stdin
  jdan b64 enc "data" --url           # URL-safe (-_  替换 +/)
  jdan b64 enc "data" --no-pad        # 去掉 = padding
  jdan b64 enc -i input.bin -o out.b64 # file → file`,
	}
	cmd.AddCommand(newB64EncodeCommand(deps))
	cmd.AddCommand(newB64DecodeCommand(deps))
	return cmd
}

func newB64EncodeCommand(deps b64CmdDeps) *cobra.Command {
	cmd := &cobra.Command{
		Use:   "enc [text]",
		Short: "base64 编码（text arg / stdin / -i file）",
		Args:  cobra.MaximumNArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			return runB64Encode(cmd, args, deps.out, deps.in)
		},
	}
	cmd.Flags().Bool("url", false, "URL-safe 字母表（-_ 替换 +/）")
	cmd.Flags().Bool("no-pad", false, "去掉末尾的 = padding")
	cmd.Flags().StringP("input", "i", "", "从文件读输入")
	cmd.Flags().StringP("output", "o", "", "写到文件而不是 stdout")
	cmd.Flags().Bool("no-newline", false, "不在输出末尾加换行")
	return cmd
}

func newB64DecodeCommand(deps b64CmdDeps) *cobra.Command {
	cmd := &cobra.Command{
		Use:   "dec [text]",
		Short: "base64 解码（text arg / stdin / -i file）",
		Args:  cobra.MaximumNArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			return runB64Decode(cmd, args, deps.out, deps.in)
		},
	}
	cmd.Flags().Bool("url", false, "URL-safe 字母表（-_ 替换 +/）")
	cmd.Flags().StringP("input", "i", "", "从文件读输入")
	cmd.Flags().StringP("output", "o", "", "写到文件而不是 stdout")
	return cmd
}

func runB64Encode(cmd *cobra.Command, args []string, out io.Writer, stdin io.Reader) error {
	useURL, _ := cmd.Flags().GetBool("url")
	noPad, _ := cmd.Flags().GetBool("no-pad")
	inputFile, _ := cmd.Flags().GetString("input")
	outputFile, _ := cmd.Flags().GetString("output")
	noNewline, _ := cmd.Flags().GetBool("no-newline")

	data, err := readInput(args, inputFile, stdin)
	if err != nil {
		return err
	}

	enc := pickB64Encoding(useURL, noPad)
	encoded := enc.EncodeToString(data)

	return writeOutputString(encoded, outputFile, out, !noNewline)
}

func runB64Decode(cmd *cobra.Command, args []string, out io.Writer, stdin io.Reader) error {
	useURL, _ := cmd.Flags().GetBool("url")
	inputFile, _ := cmd.Flags().GetString("input")
	outputFile, _ := cmd.Flags().GetString("output")

	data, err := readInput(args, inputFile, stdin)
	if err != nil {
		return err
	}
	input := strings.TrimSpace(string(data))

	// Auto-detect padding: 如果含 `=` 用 standard，否则用 raw
	hasPad := strings.Contains(input, "=")
	enc := pickB64DecodeEncoding(useURL, hasPad)
	decoded, err := enc.DecodeString(input)
	if err != nil {
		// 重试用另一种 padding
		var alt *base64.Encoding
		if useURL {
			alt = base64.RawURLEncoding
			if hasPad {
				alt = base64.URLEncoding
			}
		} else {
			alt = base64.RawStdEncoding
			if hasPad {
				alt = base64.StdEncoding
			}
		}
		if decoded, err2 := alt.DecodeString(input); err2 == nil {
			return writeOutputBytes(decoded, outputFile, out, false)
		}
		_ = decoded
		return fmt.Errorf("base64 decode: %w", err)
	}

	return writeOutputBytes(decoded, outputFile, out, false)
}

func pickB64Encoding(useURL, noPad bool) *base64.Encoding {
	switch {
	case useURL && noPad:
		return base64.RawURLEncoding
	case useURL:
		return base64.URLEncoding
	case noPad:
		return base64.RawStdEncoding
	}
	return base64.StdEncoding
}

func pickB64DecodeEncoding(useURL, hasPad bool) *base64.Encoding {
	switch {
	case useURL && hasPad:
		return base64.URLEncoding
	case useURL:
		return base64.RawURLEncoding
	case hasPad:
		return base64.StdEncoding
	}
	return base64.RawStdEncoding
}

// readInput 决定输入来自 arg / file / stdin。
// 优先级：args[0] > -i file > stdin
func readInput(args []string, inputFile string, stdin io.Reader) ([]byte, error) {
	if len(args) > 0 {
		return []byte(args[0]), nil
	}
	if inputFile != "" {
		return os.ReadFile(inputFile)
	}
	if stdin == nil {
		return nil, errors.New("no input")
	}
	return io.ReadAll(stdin)
}

// writeOutputString 决定输出到 file / stdout。string 路径会去掉末尾换行的歧义。
func writeOutputString(s, outputFile string, out io.Writer, addNewline bool) error {
	if outputFile != "" {
		return os.WriteFile(outputFile, []byte(s), 0o644)
	}
	if addNewline {
		fmt.Fprintln(out, s)
	} else {
		fmt.Fprint(out, s)
	}
	return nil
}

// writeOutputBytes 写二进制——decode 输出可能是 binary。
func writeOutputBytes(b []byte, outputFile string, out io.Writer, _ bool) error {
	if outputFile != "" {
		return os.WriteFile(outputFile, b, 0o644)
	}
	_, err := out.Write(b)
	return err
}

func init() {
	rootCmd.AddCommand(newB64Command(b64CmdDeps{}))
}
