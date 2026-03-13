package experiment

import (
	"context"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/mark3labs/mcp-go/mcp"
)

func TestRunExperiment(t *testing.T) {
	dir := t.TempDir()
	origDir, _ := os.Getwd()
	os.Chdir(dir)
	defer os.Chdir(origDir)

	// Write a benchmark script
	script := "#!/bin/bash\necho 'METRIC duration_ms=42.5'\necho 'METRIC memory_kb=1024'"
	os.WriteFile(filepath.Join(dir, "bench.sh"), []byte(script), 0755)

	// Init campaign via JSONL (not via handler, to isolate this test)
	cfg := Config{
		Name:             "test",
		MetricName:       "duration_ms",
		MetricUnit:       "ms",
		Direction:        "lower_is_better",
		BenchmarkCommand: "bash bench.sh",
		WorkingDirectory: dir,
	}
	WriteConfigHeader(filepath.Join(dir, "interlab.jsonl"), cfg)

	req := mcp.CallToolRequest{}
	result, err := handleRunExperiment(context.Background(), req)
	if err != nil {
		t.Fatalf("run_experiment error: %v", err)
	}

	text := result.Content[0].(mcp.TextContent).Text
	if !strings.Contains(text, "duration_ms") {
		t.Errorf("expected metric in output, got: %s", text)
	}
	if !strings.Contains(text, "42.5") {
		t.Errorf("expected metric value 42.5 in output, got: %s", text)
	}
}

func TestRunExperimentCircuitBreaker(t *testing.T) {
	dir := t.TempDir()
	origDir, _ := os.Getwd()
	os.Chdir(dir)
	defer os.Chdir(origDir)

	cfg := Config{
		Name:             "crash-test",
		MetricName:       "score",
		MetricUnit:       "points",
		Direction:        "higher_is_better",
		BenchmarkCommand: "false",
		MaxCrashes:       2,
		WorkingDirectory: dir,
	}
	path := filepath.Join(dir, "interlab.jsonl")
	WriteConfigHeader(path, cfg)
	WriteResult(path, Result{Decision: "crash", DurationMs: 100})
	WriteResult(path, Result{Decision: "crash", DurationMs: 100})

	req := mcp.CallToolRequest{}
	result, _ := handleRunExperiment(context.Background(), req)
	text := result.Content[0].(mcp.TextContent).Text
	if !strings.Contains(strings.ToLower(text), "circuit breaker") {
		t.Errorf("expected circuit breaker error, got: %s", text)
	}
}

func TestRunExperimentNoCampaign(t *testing.T) {
	dir := t.TempDir()
	origDir, _ := os.Getwd()
	os.Chdir(dir)
	defer os.Chdir(origDir)

	req := mcp.CallToolRequest{}
	result, _ := handleRunExperiment(context.Background(), req)
	text := result.Content[0].(mcp.TextContent).Text
	if !strings.Contains(text, "init_experiment") {
		t.Errorf("expected 'call init_experiment first' message, got: %s", text)
	}
}
