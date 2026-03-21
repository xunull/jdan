package cli

import "github.com/spf13/cobra"

// Execute runs the root command.
func Execute() error {
	return rootCmd.Execute()
}

var rootCmd = &cobra.Command{
	Use:   "jdan",
	Short: "常用小工具集合",
}
