package cli

import (
	"fmt"
	"io"
	"os"
	"strings"

	"github.com/spf13/cobra"

	"github.com/xunull/jdan/internal/unixtime"
)

var unixTimeCmd = &cobra.Command{
	Use:   "unix-time [timestamp]",
	Short: "将 Unix 时间戳（秒/毫秒）转换为可读时间",
	Args:  cobra.MaximumNArgs(1),
	RunE: func(cmd *cobra.Command, args []string) error {
		input, err := resolveUnixTimeInput(args)
		if err != nil {
			return err
		}

		out, err := unixtime.Convert(input)
		if err != nil {
			return err
		}

		fmt.Print(out)
		return nil
	},
}

func resolveUnixTimeInput(args []string) (string, error) {
	if len(args) == 1 {
		return args[0], nil
	}

	info, err := os.Stdin.Stat()
	if err != nil {
		return "", err
	}
	if (info.Mode() & os.ModeCharDevice) != 0 {
		return "", fmt.Errorf("请提供时间戳参数，或通过 stdin 传入一个值")
	}

	data, err := io.ReadAll(os.Stdin)
	if err != nil {
		return "", err
	}
	input := strings.TrimSpace(string(data))
	if input == "" {
		return "", fmt.Errorf("stdin 输入为空，请提供一个 Unix 时间戳")
	}
	return input, nil
}

func init() {
	rootCmd.AddCommand(unixTimeCmd)
}
