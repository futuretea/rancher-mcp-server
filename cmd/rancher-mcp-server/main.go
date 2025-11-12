package main

import (
	"os"

	"github.com/rs/zerolog"
	"github.com/rs/zerolog/log"

	"github.com/futuretea/rancher-mcp-server/pkg/rancher-mcp-server/cmd"
)

func init() {
	zerolog.TimeFieldFormat = zerolog.TimeFormatUnix
}

func main() {
	command := cmd.NewMCPServer(cmd.IOStreams{
		In:     os.Stdin,
		Out:    os.Stdout,
		ErrOut: os.Stderr,
	})

	if err := command.Execute(); err != nil {
		log.Fatal().Err(err).Msg("Failed to execute command")
	}
}