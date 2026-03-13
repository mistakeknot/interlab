package experiment

import (
	"context"
	"os"
	"os/exec"
	"path/filepath"
	"testing"

	"github.com/mark3labs/mcp-go/mcp"
)

func runCmd(t *testing.T, name string, args ...string) {
	t.Helper()
	cmd := exec.Command(name, args...)
	cmd.Stdout = os.Stdout
	cmd.Stderr = os.Stderr
	if err := cmd.Run(); err != nil {
		t.Fatalf("command %s %v failed: %v", name, args, err)
	}
}

func TestInitExperiment(t *testing.T) {
	dir := t.TempDir()
	origDir, _ := os.Getwd()
	os.Chdir(dir)
	defer os.Chdir(origDir)

	// Init a git repo for branch creation
	runCmd(t, "git", "init")
	runCmd(t, "git", "config", "user.email", "test@test.com")
	runCmd(t, "git", "config", "user.name", "Test")
	runCmd(t, "git", "commit", "--allow-empty", "-m", "initial")

	req := mcp.CallToolRequest{}
	req.Params.Arguments = map[string]interface{}{
		"name":              "test-speed",
		"metric_name":       "duration_ms",
		"metric_unit":       "ms",
		"direction":         "lower_is_better",
		"benchmark_command": "echo METRIC duration_ms=42",
	}

	result, err := handleInitExperiment(context.Background(), req)
	if err != nil {
		t.Fatalf("init_experiment error: %v", err)
	}

	text := result.Content[0].(mcp.TextContent).Text
	if text == "" {
		t.Error("expected non-empty response")
	}

	// Verify JSONL was written
	data, err := os.ReadFile(filepath.Join(dir, "interlab.jsonl"))
	if err != nil {
		t.Fatalf("JSONL not created: %v", err)
	}
	if len(data) == 0 {
		t.Error("JSONL is empty")
	}
}

func TestInitExperimentValidation(t *testing.T) {
	dir := t.TempDir()
	origDir, _ := os.Getwd()
	os.Chdir(dir)
	defer os.Chdir(origDir)

	// Missing required fields
	req := mcp.CallToolRequest{}
	req.Params.Arguments = map[string]interface{}{
		"name": "test",
	}

	result, err := handleInitExperiment(context.Background(), req)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	text := result.Content[0].(mcp.TextContent).Text
	if text == "" {
		t.Error("expected error message for missing fields")
	}
}
