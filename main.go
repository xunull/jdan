package main

import (
	"os"
	"time"

	"github.com/rs/zerolog"
	"github.com/rs/zerolog/log"

	"github.com/xunull/jdan/internal/cli"
)

func main() {
	// 日志走 stderr。stdout 是命令的数据输出（--json、管道下游要读的东西），
	// 把日志和错误混进去会污染它 —— 之前 `jdan size /nope | jq` 会把一行
	// zerolog FTL 喂给 jq。
	log.Logger = log.Output(zerolog.ConsoleWriter{
		Out:        os.Stderr,
		TimeFormat: time.RFC3339,
	})
	if err := cli.Execute(); err != nil {
		// 错误已经由 cli.Execute 打印过了。这里再 log.Fatal 一次会出现重复：
		// cobra 先打「Error: unknown command … Did you mean this? size」，
		// 紧接着又来一行 FTL 把同一条错误转义成机器格式。只留退出码。
		os.Exit(1)
	}
}
