package main

import (
	"os"
	"time"

	"github.com/rs/zerolog"
	"github.com/rs/zerolog/log"

	"github.com/xunull/jdan/internal/cli"
)

func main() {
	log.Logger = log.Output(zerolog.ConsoleWriter{
		Out:        os.Stdout,
		TimeFormat: time.RFC3339,
	})
	if err := cli.Execute(); err != nil {
		log.Fatal().Err(err).Msg("command failed")
	}
}
