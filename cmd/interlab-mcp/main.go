package main

import (
	"fmt"
	"os"

	"github.com/mark3labs/mcp-go/server"
	"github.com/mistakeknot/interlab/internal/experiment"
)

func main() {
	s := server.NewMCPServer(
		"interlab",
		"0.1.0",
		server.WithToolCapabilities(true),
	)

	experiment.RegisterAll(s)

	if err := server.ServeStdio(s); err != nil {
		fmt.Fprintf(os.Stderr, "interlab-mcp: %v\n", err)
		os.Exit(1)
	}
}
