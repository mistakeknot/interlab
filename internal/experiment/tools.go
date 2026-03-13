package experiment

import (
	"context"

	"github.com/mark3labs/mcp-go/mcp"
	"github.com/mark3labs/mcp-go/server"
)

// RegisterAll registers all interlab tools with the MCP server.
func RegisterAll(s *server.MCPServer) {
	s.AddTool(initExperimentTool, handleInitExperiment)
	s.AddTool(runExperimentTool, handleRunExperiment)
	s.AddTool(logExperimentTool, handleLogExperiment)
}

var initExperimentTool = mcp.NewTool("init_experiment",
	mcp.WithDescription("Configure a new experiment campaign: metric, direction, benchmark command, files in scope."),
	mcp.WithString("name", mcp.Required(), mcp.Description("Campaign name (e.g., 'skaffen-test-speed')")),
	mcp.WithString("metric_name", mcp.Required(), mcp.Description("Primary metric to optimize")),
	mcp.WithString("metric_unit", mcp.Required(), mcp.Description("Unit of measurement (ms, bytes, score)")),
	mcp.WithString("direction", mcp.Required(), mcp.Description("'lower_is_better' or 'higher_is_better'")),
	mcp.WithString("benchmark_command", mcp.Required(), mcp.Description("Shell command to run the benchmark")),
	mcp.WithString("working_directory", mcp.Description("Working directory for the benchmark (default: cwd)")),
)

var runExperimentTool = mcp.NewTool("run_experiment",
	mcp.WithDescription("Execute the benchmark command, capture output and timing. Checks circuit breaker before running."),
)

var logExperimentTool = mcp.NewTool("log_experiment",
	mcp.WithDescription("Record experiment result. 'keep' commits changes, 'discard'/'crash' reverts."),
	mcp.WithString("decision", mcp.Required(), mcp.Description("'keep', 'discard', or 'crash'")),
	mcp.WithString("description", mcp.Required(), mcp.Description("What was changed and why")),
)

func handleInitExperiment(ctx context.Context, req mcp.CallToolRequest) (*mcp.CallToolResult, error) {
	return mcp.NewToolResultText("init_experiment: not yet implemented"), nil
}

func handleRunExperiment(ctx context.Context, req mcp.CallToolRequest) (*mcp.CallToolResult, error) {
	return mcp.NewToolResultText("run_experiment: not yet implemented"), nil
}

func handleLogExperiment(ctx context.Context, req mcp.CallToolRequest) (*mcp.CallToolResult, error) {
	return mcp.NewToolResultText("log_experiment: not yet implemented"), nil
}
