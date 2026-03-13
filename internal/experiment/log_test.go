package experiment

import (
	"context"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/mark3labs/mcp-go/mcp"
)

func TestLogExperimentKeep(t *testing.T) {
	dir := t.TempDir()
	origDir, _ := os.Getwd()
	os.Chdir(dir)
	defer os.Chdir(origDir)

	// Setup git repo
	runCmd(t, "git", "init")
	runCmd(t, "git", "config", "user.email", "test@test.com")
	runCmd(t, "git", "config", "user.name", "Test")

	// Create a file in scope and commit it
	os.WriteFile(filepath.Join(dir, "code.go"), []byte("package main\n"), 0644)
	runCmd(t, "git", "add", "code.go")
	runCmd(t, "git", "commit", "-m", "initial")

	// Init campaign with files_in_scope
	cfg := Config{
		Name:             "test",
		MetricName:       "duration_ms",
		MetricUnit:       "ms",
		Direction:        "lower_is_better",
		BenchmarkCommand: "echo METRIC duration_ms=42",
		WorkingDirectory: dir,
		FilesInScope:     []string{"code.go"},
	}
	jsonlPath := filepath.Join(dir, "interlab.jsonl")
	WriteConfigHeader(jsonlPath, cfg)

	// Simulate a code change
	os.WriteFile(filepath.Join(dir, "code.go"), []byte("package main\n// optimized\n"), 0644)

	// Write run details (normally done by run_experiment)
	writeRunDetails(dir, RunDetails{
		ExitCode:   0,
		DurationMs: 100,
		Metrics:    map[string]float64{"duration_ms": 42.0},
		Output:     "METRIC duration_ms=42",
	})

	req := mcp.CallToolRequest{}
	req.Params.Arguments = map[string]interface{}{
		"decision":    "keep",
		"description": "optimized hot path",
	}

	result, err := handleLogExperiment(context.Background(), req)
	if err != nil {
		t.Fatalf("log_experiment error: %v", err)
	}

	text := result.Content[0].(mcp.TextContent).Text
	if !strings.Contains(strings.ToLower(text), "keep") {
		t.Errorf("expected 'keep' in response, got: %s", text)
	}

	// Verify JSONL has a result entry
	s, _ := ReconstructState(jsonlPath)
	if s.RunCount != 1 {
		t.Errorf("expected 1 run, got %d", s.RunCount)
	}
	if s.KeptCount != 1 {
		t.Errorf("expected 1 kept, got %d", s.KeptCount)
	}
}

func TestLogExperimentDiscard(t *testing.T) {
	dir := t.TempDir()
	origDir, _ := os.Getwd()
	os.Chdir(dir)
	defer os.Chdir(origDir)

	// Setup git repo
	runCmd(t, "git", "init")
	runCmd(t, "git", "config", "user.email", "test@test.com")
	runCmd(t, "git", "config", "user.name", "Test")

	os.WriteFile(filepath.Join(dir, "code.go"), []byte("package main\n"), 0644)
	runCmd(t, "git", "add", "code.go")
	runCmd(t, "git", "commit", "-m", "initial")

	cfg := Config{
		Name:             "test",
		MetricName:       "duration_ms",
		MetricUnit:       "ms",
		Direction:        "lower_is_better",
		BenchmarkCommand: "echo METRIC duration_ms=42",
		WorkingDirectory: dir,
		FilesInScope:     []string{"code.go"},
	}
	jsonlPath := filepath.Join(dir, "interlab.jsonl")
	WriteConfigHeader(jsonlPath, cfg)

	// Simulate a code change
	os.WriteFile(filepath.Join(dir, "code.go"), []byte("package main\n// bad change\n"), 0644)

	writeRunDetails(dir, RunDetails{
		ExitCode:   0,
		DurationMs: 200,
		Metrics:    map[string]float64{"duration_ms": 99.0},
		Output:     "METRIC duration_ms=99",
	})

	req := mcp.CallToolRequest{}
	req.Params.Arguments = map[string]interface{}{
		"decision":    "discard",
		"description": "regression",
	}

	result, _ := handleLogExperiment(context.Background(), req)
	text := result.Content[0].(mcp.TextContent).Text
	if !strings.Contains(strings.ToLower(text), "discard") {
		t.Errorf("expected 'discard' in response, got: %s", text)
	}

	// Verify file was reverted
	content, _ := os.ReadFile(filepath.Join(dir, "code.go"))
	if strings.Contains(string(content), "bad change") {
		t.Error("expected code.go to be reverted after discard")
	}

	// Verify JSONL
	s, _ := ReconstructState(jsonlPath)
	if s.DiscardedCount != 1 {
		t.Errorf("expected 1 discarded, got %d", s.DiscardedCount)
	}
}

func TestLogExperimentNoRunDetails(t *testing.T) {
	dir := t.TempDir()
	origDir, _ := os.Getwd()
	os.Chdir(dir)
	defer os.Chdir(origDir)

	cfg := Config{
		Name:             "test",
		MetricName:       "x",
		Direction:        "lower_is_better",
		BenchmarkCommand: "true",
		WorkingDirectory: dir,
	}
	WriteConfigHeader(filepath.Join(dir, "interlab.jsonl"), cfg)

	// No run details file — should error
	req := mcp.CallToolRequest{}
	req.Params.Arguments = map[string]interface{}{
		"decision":    "keep",
		"description": "no preceding run",
	}

	result, _ := handleLogExperiment(context.Background(), req)
	text := result.Content[0].(mcp.TextContent).Text
	if !strings.Contains(text, "run_experiment") {
		t.Errorf("expected error about missing run_experiment, got: %s", text)
	}
}
