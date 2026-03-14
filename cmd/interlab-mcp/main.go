package main

import (
	"fmt"
	"os"

	"github.com/mark3labs/mcp-go/server"
	"github.com/mistakeknot/interlab/internal/experiment"
	"github.com/mistakeknot/interlab/internal/orchestration"
)

func main() {
	s := server.NewMCPServer(
		"interlab",
		"0.2.0",
		server.WithToolCapabilities(true),
	)

	experiment.RegisterAll(s)
	orchestration.RegisterAll(s)

	if err := server.ServeStdio(s); err != nil {
		fmt.Fprintf(os.Stderr, "interlab-mcp: %v\n", err)
		os.Exit(1)
	}
}
