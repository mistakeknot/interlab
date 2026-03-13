package experiment

import (
	"context"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"

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
	// Extract arguments
	name := req.GetString("name", "")
	metricName := req.GetString("metric_name", "")
	metricUnit := req.GetString("metric_unit", "")
	direction := req.GetString("direction", "")
	benchCmd := req.GetString("benchmark_command", "")
	workDir := req.GetString("working_directory", "")

	// Validate required fields
	var missing []string
	if name == "" {
		missing = append(missing, "name")
	}
	if metricName == "" {
		missing = append(missing, "metric_name")
	}
	if direction == "" {
		missing = append(missing, "direction")
	}
	if benchCmd == "" {
		missing = append(missing, "benchmark_command")
	}
	if len(missing) > 0 {
		return mcp.NewToolResultText(fmt.Sprintf("missing required fields: %v", missing)), nil
	}

	// Validate direction
	if direction != "lower_is_better" && direction != "higher_is_better" {
		return mcp.NewToolResultText(fmt.Sprintf("invalid direction %q: must be 'lower_is_better' or 'higher_is_better'", direction)), nil
	}

	// Default working directory to cwd
	if workDir == "" {
		var err error
		workDir, err = os.Getwd()
		if err != nil {
			return nil, fmt.Errorf("get working directory: %w", err)
		}
	}

	// Build config and write header
	cfg := Config{
		Name:             name,
		MetricName:       metricName,
		MetricUnit:       metricUnit,
		Direction:        direction,
		BenchmarkCommand: benchCmd,
		WorkingDirectory: workDir,
	}

	jsonlPath := filepath.Join(workDir, "interlab.jsonl")
	if err := WriteConfigHeader(jsonlPath, cfg); err != nil {
		return nil, fmt.Errorf("write config header: %w", err)
	}

	// Best-effort: create experiment branch
	gitCmd := exec.CommandContext(ctx, "git", "-C", workDir, "checkout", "-b", "interlab/"+name)
	branchMsg := ""
	if err := gitCmd.Run(); err != nil {
		branchMsg = fmt.Sprintf(" (branch creation skipped: %v)", err)
	} else {
		branchMsg = fmt.Sprintf(" on branch interlab/%s", name)
	}

	// Reconstruct state to return summary
	state, err := ReconstructState(jsonlPath)
	if err != nil {
		return nil, fmt.Errorf("reconstruct state: %w", err)
	}

	summary := fmt.Sprintf("Experiment %q initialized%s\n"+
		"  metric: %s (%s, %s)\n"+
		"  command: %s\n"+
		"  limits: %d experiments, %d crashes, %d no-improvement\n"+
		"  segment: %d",
		name, branchMsg,
		state.Config.MetricName, state.Config.MetricUnit, state.Config.Direction,
		state.Config.BenchmarkCommand,
		state.Config.MaxExperiments, state.Config.MaxCrashes, state.Config.MaxNoImprovement,
		state.SegmentID,
	)

	return mcp.NewToolResultText(summary), nil
}

func handleRunExperiment(ctx context.Context, req mcp.CallToolRequest) (*mcp.CallToolResult, error) {
	return mcp.NewToolResultText("run_experiment: not yet implemented"), nil
}

func handleLogExperiment(ctx context.Context, req mcp.CallToolRequest) (*mcp.CallToolResult, error) {
	return mcp.NewToolResultText("log_experiment: not yet implemented"), nil
}
