package main

import (
	"os"

	"github.com/futuretea/rancher-mcp-server/pkg/rancher-mcp-server/cmd"
)

func main() {
	command := cmd.NewMCPServer(cmd.IOStreams{
		In:     os.Stdin,
		Out:    os.Stdout,
		ErrOut: os.Stderr,
	})

	if err := command.Execute(); err != nil {
		os.Exit(1)
	}
}