package cli

import (
	"context"
	"fmt"
	"os"
	"runtime"

	tea "charm.land/bubbletea/v2"
	"github.com/rs/zerolog"
	"github.com/rs/zerolog/log"
	"github.com/spf13/cobra"

	"github.com/xunull/jdan/internal/macgpu"
)

var macgpuCmd = &cobra.Command{
	Use:   "macgpu",
	Short: "监控 Apple Silicon Mac 的 GPU 实时指标（需要 sudo）",
	Long: `监控 Apple Silicon Mac 的 GPU 实时使用率、功耗、频率和散热压力等级。

数据来源：sudo powermetrics（macOS 内置工具）
输出方式：htop/glances 风格的实时 TUI 界面

注意：此命令仅支持 Apple Silicon（arm64）Mac，且需要 sudo 权限运行。
示例：sudo jdan macgpu`,
	RunE: func(cmd *cobra.Command, args []string) error {
		if runtime.GOARCH != "arm64" {
			return fmt.Errorf("jdan macgpu 仅支持 Apple Silicon (arm64) Mac，当前架构: %s", runtime.GOARCH)
		}

		if os.Getuid() != 0 {
			return fmt.Errorf("请使用 sudo 运行此命令：sudo jdan macgpu")
		}

		interval, err := cmd.Flags().GetInt("interval")
		if err != nil {
			return err
		}
		if interval < 500 {
			return fmt.Errorf("采样间隔不得小于 500ms，当前值: %d", interval)
		}

		// TUI 接管 stdout，将 zerolog 重定向到临时文件以避免输出冲突。
		logFile, err := os.CreateTemp("", "jdan-macgpu-*.log")
		if err != nil {
			log.Warn().Err(err).Msg("无法创建日志文件，日志将被丢弃")
			log.Logger = zerolog.Nop()
		} else {
			defer logFile.Close()
			log.Logger = zerolog.New(logFile).With().Timestamp().Logger()
		}

		ctx, cancel := context.WithCancel(context.Background())
		defer cancel()

		model := macgpu.NewModel(interval, cancel)
		program := tea.NewProgram(model)

		collector := macgpu.NewCollector(ctx, interval, program)
		collector.Start()

		_, runErr := program.Run()
		return runErr
	},
}

func init() {
	macgpuCmd.Flags().IntP("interval", "i", 2000, "采样间隔（ms，最小 500）")
	rootCmd.AddCommand(macgpuCmd)
}
