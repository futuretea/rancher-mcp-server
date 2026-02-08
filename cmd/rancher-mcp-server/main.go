package main

import (
	"os"

	"github.com/rs/zerolog"
	"github.com/rs/zerolog/log"

	"github.com/futuretea/rancher-mcp-server/internal/cmd"
	"github.com/futuretea/rancher-mcp-server/pkg/core/logging"
)

func init() {
	zerolog.TimeFieldFormat = zerolog.TimeFormatUnix
	// Initialize basic logging for early error handling
	// This will be reconfigured in runServer based on mode (stdio/HTTP)
	logging.Initialize(0, os.Stderr)
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