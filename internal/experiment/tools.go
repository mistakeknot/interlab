package experiment

import (
	"context"
	"encoding/json"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"regexp"
	"strconv"
	"strings"
	"time"

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
	mcp.WithString("working_directory", mcp.Description("Directory containing interlab.jsonl (default: cwd)")),
)

var logExperimentTool = mcp.NewTool("log_experiment",
	mcp.WithDescription("Record experiment result. 'keep' commits changes, 'discard'/'crash' reverts."),
	mcp.WithString("decision", mcp.Required(), mcp.Description("'keep', 'discard', or 'crash'")),
	mcp.WithString("description", mcp.Required(), mcp.Description("What was changed and why")),
	mcp.WithString("working_directory", mcp.Description("Directory containing interlab.jsonl (default: cwd)")),
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
	// Resolve campaign directory: explicit param > cwd
	cwd := req.GetString("working_directory", "")
	if cwd == "" {
		var err error
		cwd, err = os.Getwd()
		if err != nil {
			return nil, fmt.Errorf("get working directory: %w", err)
		}
	}

	jsonlPath := filepath.Join(cwd, "interlab.jsonl")

	// Reconstruct state
	state, err := ReconstructState(jsonlPath)
	if err != nil {
		return nil, fmt.Errorf("reconstruct state: %w", err)
	}

	// No active campaign?
	if state.SegmentID == 0 {
		return mcp.NewToolResultText("no active campaign — call init_experiment first"), nil
	}

	// Check circuit breaker
	if cbErr := state.CheckCircuitBreaker(); cbErr != nil {
		return mcp.NewToolResultText(cbErr.Error()), nil
	}

	// Determine working directory for benchmark
	workDir := state.Config.WorkingDirectory
	if workDir == "" {
		workDir = cwd
	}

	// Execute benchmark
	start := time.Now()
	cmd := exec.CommandContext(ctx, "bash", "-c", state.Config.BenchmarkCommand)
	cmd.Dir = workDir
	output, cmdErr := cmd.CombinedOutput()
	durationMs := time.Since(start).Milliseconds()

	exitCode := 0
	if cmdErr != nil {
		if exitErr, ok := cmdErr.(*exec.ExitError); ok {
			exitCode = exitErr.ExitCode()
		} else {
			return nil, fmt.Errorf("execute benchmark: %w", cmdErr)
		}
	}

	outputStr := string(output)

	// Parse METRIC lines
	metrics := parseMetrics(outputStr)

	// Build response
	expNum := state.RunCount + 1
	var b strings.Builder
	fmt.Fprintf(&b, "## Run #%d (segment %d)\n", expNum, state.SegmentID)
	fmt.Fprintf(&b, "- duration: %dms\n", durationMs)
	fmt.Fprintf(&b, "- exit code: %d\n", exitCode)

	// Primary metric
	primaryName := state.Config.MetricName
	if val, ok := metrics[primaryName]; ok {
		fmt.Fprintf(&b, "- %s: %.4g %s\n", primaryName, val, state.Config.MetricUnit)
		if state.HasBaseline {
			delta := val - state.BestMetric
			sign := "+"
			if delta < 0 {
				sign = ""
			}
			fmt.Fprintf(&b, "- delta vs best: %s%.4g %s\n", sign, delta, state.Config.MetricUnit)
		}
	}

	// All metrics
	if len(metrics) > 0 {
		fmt.Fprintf(&b, "\n### Metrics\n")
		for name, val := range metrics {
			fmt.Fprintf(&b, "- %s = %.4g\n", name, val)
		}
	}

	// Truncated output tail
	tail := truncateTail(outputStr, 20)
	if tail != "" {
		fmt.Fprintf(&b, "\n### Output (last 20 lines)\n```\n%s\n```\n", tail)
	}

	// Save run details for log_experiment
	details := RunDetails{
		ExitCode:   exitCode,
		DurationMs: durationMs,
		Metrics:    metrics,
		Output:     outputStr,
	}
	if wErr := writeRunDetails(cwd, details); wErr != nil {
		fmt.Fprintf(&b, "\n(warning: failed to save run details: %v)\n", wErr)
	}

	return mcp.NewToolResultText(b.String()), nil
}

// RunDetails holds data from a benchmark run for log_experiment to consume.
type RunDetails struct {
	ExitCode   int                `json:"exit_code"`
	DurationMs int64              `json:"duration_ms"`
	Metrics    map[string]float64 `json:"metrics"`
	Output     string             `json:"output"`
}

var metricPattern = regexp.MustCompile(`^METRIC\s+(\S+)=(\S+)$`)

// parseMetrics scans output lines for METRIC name=value patterns.
func parseMetrics(output string) map[string]float64 {
	metrics := make(map[string]float64)
	for _, line := range strings.Split(output, "\n") {
		line = strings.TrimSpace(line)
		m := metricPattern.FindStringSubmatch(line)
		if m == nil {
			continue
		}
		val, err := strconv.ParseFloat(m[2], 64)
		if err != nil {
			continue
		}
		metrics[m[1]] = val
	}
	return metrics
}

// truncateTail returns the last n lines of s.
func truncateTail(s string, n int) string {
	lines := strings.Split(strings.TrimRight(s, "\n"), "\n")
	if len(lines) <= n {
		return strings.Join(lines, "\n")
	}
	return strings.Join(lines[len(lines)-n:], "\n")
}

const runDetailsFile = ".interlab-run.json"

func writeRunDetails(dir string, d RunDetails) error {
	data, err := json.Marshal(d)
	if err != nil {
		return err
	}
	return os.WriteFile(filepath.Join(dir, runDetailsFile), data, 0644)
}

func readRunDetails(dir string) (*RunDetails, error) {
	data, err := os.ReadFile(filepath.Join(dir, runDetailsFile))
	if err != nil {
		return nil, err
	}
	var d RunDetails
	if err := json.Unmarshal(data, &d); err != nil {
		return nil, err
	}
	return &d, nil
}

func handleLogExperiment(ctx context.Context, req mcp.CallToolRequest) (*mcp.CallToolResult, error) {
	// Resolve campaign directory: explicit param > cwd
	cwd := req.GetString("working_directory", "")
	if cwd == "" {
		var err error
		cwd, err = os.Getwd()
		if err != nil {
			return nil, fmt.Errorf("get working directory: %w", err)
		}
	}

	jsonlPath := filepath.Join(cwd, "interlab.jsonl")

	// Reconstruct state
	state, err := ReconstructState(jsonlPath)
	if err != nil {
		return nil, fmt.Errorf("reconstruct state: %w", err)
	}

	if state.SegmentID == 0 {
		return mcp.NewToolResultText("no active campaign — call init_experiment first"), nil
	}

	// Read run details from previous run_experiment call
	details, err := readRunDetails(cwd)
	if err != nil {
		return mcp.NewToolResultText("no preceding run_experiment call — run_experiment must be called before log_experiment"), nil
	}

	// Extract arguments
	decision := req.GetString("decision", "")
	description := req.GetString("description", "")

	// Validate decision
	if decision != "keep" && decision != "discard" && decision != "crash" {
		return mcp.NewToolResultText(fmt.Sprintf("invalid decision %q: must be 'keep', 'discard', or 'crash'", decision)), nil
	}

	cfg := state.Config
	workDir := cfg.WorkingDirectory
	if workDir == "" {
		workDir = cwd
	}

	// Get primary metric value
	primaryValue := details.Metrics[cfg.MetricName]

	// Build secondary metrics (everything except primary)
	var secondary map[string]float64
	for k, v := range details.Metrics {
		if k != cfg.MetricName {
			if secondary == nil {
				secondary = make(map[string]float64)
			}
			secondary[k] = v
		}
	}

	// Build result
	result := Result{
		Decision:         decision,
		Description:      description,
		MetricValue:      primaryValue,
		DurationMs:       details.DurationMs,
		ExitCode:         details.ExitCode,
		SecondaryMetrics: secondary,
	}

	var b strings.Builder

	switch decision {
	case "keep":
		// Stage and commit files in scope
		if len(cfg.FilesInScope) > 0 {
			// Path-scoped git add — NEVER git add -A
			for _, f := range cfg.FilesInScope {
				addCmd := exec.CommandContext(ctx, "git", "-C", workDir, "add", f)
				addCmd.Run() // best-effort
			}

			// Build commit message with trailers
			commitMsg := fmt.Sprintf("interlab: %s\n\nMetric-Name: %s\nMetric-Value: %.4g",
				description, cfg.MetricName, primaryValue)
			if cfg.BeadID != "" {
				commitMsg += fmt.Sprintf("\nBead-ID: %s", cfg.BeadID)
			}

			commitCmd := exec.CommandContext(ctx, "git", "-C", workDir, "commit", "-m", commitMsg)
			commitCmd.Run() // best-effort

			// Capture commit hash
			hashCmd := exec.CommandContext(ctx, "git", "-C", workDir, "rev-parse", "HEAD")
			hashOut, hashErr := hashCmd.Output()
			if hashErr == nil {
				result.CommitHash = strings.TrimSpace(string(hashOut))
			}
		}

		fmt.Fprintf(&b, "## Decision: keep — %s\n", description)
		fmt.Fprintf(&b, "- %s: %.4g %s\n", cfg.MetricName, primaryValue, cfg.MetricUnit)
		if result.CommitHash != "" {
			fmt.Fprintf(&b, "- commit: %s\n", result.CommitHash[:min(12, len(result.CommitHash))])
		}

	case "discard", "crash":
		// Revert files in scope
		if len(cfg.FilesInScope) > 0 {
			for _, f := range cfg.FilesInScope {
				checkoutCmd := exec.CommandContext(ctx, "git", "-C", workDir, "checkout", "--", f)
				checkoutCmd.Run() // best-effort
			}
		}

		fmt.Fprintf(&b, "## Decision: %s — %s\n", decision, description)
		fmt.Fprintf(&b, "- %s: %.4g %s\n", cfg.MetricName, primaryValue, cfg.MetricUnit)
		fmt.Fprintf(&b, "- changes reverted\n")
	}

	// Write result to JSONL
	if wErr := WriteResult(jsonlPath, result); wErr != nil {
		return nil, fmt.Errorf("write result: %w", wErr)
	}

	// Cleanup run details
	os.Remove(filepath.Join(cwd, runDetailsFile))

	// Emit ic event (best-effort)
	EmitExperimentEvent(cfg, result)

	// Summary stats
	updated, _ := ReconstructState(jsonlPath)
	fmt.Fprintf(&b, "\n### Campaign: %s (segment %d)\n", cfg.Name, updated.SegmentID)
	fmt.Fprintf(&b, "- runs: %d | kept: %d | discarded: %d | crashes: %d\n",
		updated.RunCount, updated.KeptCount, updated.DiscardedCount, updated.CrashCount)
	if updated.HasBaseline {
		fmt.Fprintf(&b, "- best %s: %.4g %s\n", cfg.MetricName, updated.BestMetric, cfg.MetricUnit)
	}

	return mcp.NewToolResultText(b.String()), nil
}
