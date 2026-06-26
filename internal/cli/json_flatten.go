package cli

import (
	"github.com/spf13/cobra"

	"github.com/xunull/jdan/internal/jsonx"
)

func newJSONFlattenCommand(deps jsonCmdDeps) *cobra.Command {
	cmd := &cobra.Command{
		Use:           "flatten [file]",
		Short:         "嵌套 JSON → 扁平点分键（a.b / a.c[0]）",
		Args:          cobra.MaximumNArgs(1),
		SilenceUsage:  true,
		SilenceErrors: true,
		RunE: func(cmd *cobra.Command, args []string) error {
			sep, _ := cmd.Flags().GetString("sep")
			indent := indentFromPretty(cmd)
			return runJSONFormat(args, deps.in, deps.out, false, func(data []byte) ([]byte, error) {
				return jsonx.FlattenBytes(data, sep, indent)
			})
		},
	}
	cmd.Flags().String("sep", ".", "对象键连接符（数组始终用 [i]）")
	cmd.Flags().BoolP("pretty", "p", false, "缩进输出（默认 compact 一行）")
	return cmd
}

func newJSONUnflattenCommand(deps jsonCmdDeps) *cobra.Command {
	cmd := &cobra.Command{
		Use:           "unflatten [file]",
		Short:         "扁平点分键 → 嵌套 JSON",
		Args:          cobra.MaximumNArgs(1),
		SilenceUsage:  true,
		SilenceErrors: true,
		RunE: func(cmd *cobra.Command, args []string) error {
			sep, _ := cmd.Flags().GetString("sep")
			indent := indentFromPretty(cmd)
			return runJSONFormat(args, deps.in, deps.out, false, func(data []byte) ([]byte, error) {
				return jsonx.UnflattenBytes(data, sep, indent)
			})
		},
	}
	cmd.Flags().String("sep", ".", "对象键连接符（数组用 [i]）")
	cmd.Flags().BoolP("pretty", "p", false, "缩进输出（默认 compact 一行）")
	return cmd
}

func indentFromPretty(cmd *cobra.Command) int {
	if pretty, _ := cmd.Flags().GetBool("pretty"); pretty {
		return 2
	}
	return 0
}
