package cli

import (
	"fmt"
	"io"
	"os"
	"runtime"

	"github.com/spf13/cobra"
)

// 通过 -ldflags "-X github.com/xunull/jdan/internal/cli.buildVersion=..." 注入
var (
	buildVersion = "dev"
	buildCommit  = "none"
	buildDate    = "unknown"
)

type versionDeps struct {
	out     io.Writer
	version string
	commit  string
	date    string
	goos    string
	goarch  string
}

func newVersionCommand(deps versionDeps) *cobra.Command {
	if deps.out == nil {
		deps.out = os.Stdout
	}
	if deps.goos == "" {
		deps.goos = runtime.GOOS
	}
	if deps.goarch == "" {
		deps.goarch = runtime.GOARCH
	}
	cmd := &cobra.Command{
		Use:   "version",
		Short: "显示版本、commit、构建时间",
		RunE: func(cmd *cobra.Command, args []string) error {
			short, _ := cmd.Flags().GetBool("short")
			if short {
				fmt.Fprintln(deps.out, deps.version)
				return nil
			}
			fmt.Fprintf(deps.out,
				"jdan %s (commit %s, built %s, %s/%s)\n",
				deps.version, deps.commit, deps.date,
				deps.goos, deps.goarch,
			)
			return nil
		},
	}
	cmd.Flags().Bool("short", false, "只输出版本号")
	return cmd
}

func init() {
	rootCmd.AddCommand(newVersionCommand(versionDeps{
		version: buildVersion,
		commit:  buildCommit,
		date:    buildDate,
	}))
}
