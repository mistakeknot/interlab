package experiment

import (
	"testing"

	"github.com/mark3labs/mcp-go/server"
)

func TestRegisterAll(t *testing.T) {
	s := server.NewMCPServer("test", "0.0.1",
		server.WithToolCapabilities(true),
	)
	RegisterAll(s)
	// If we get here without panic, registration succeeded
}
