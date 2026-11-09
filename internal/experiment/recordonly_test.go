package experiment

import (
	"context"
	"math"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"testing"

	"github.com/mark3labs/mcp-go/mcp"
)

func gitOut(t *testing.T, dir string, args ...string) string {
	t.Helper()
	out, err := exec.Command("git", append([]string{"-C", dir}, args...)...).Output()
	if err != nil {
		t.Fatalf("git %v: %v", args, err)
	}
	return string(out)
}

// treeSnapshot captures HEAD, branch, tracked-file bytes, diff and index state.
func treeSnapshot(t *testing.T, dir string, files ...string) string {
	t.Helper()
	var b strings.Builder
	b.WriteString(gitOut(t, dir, "rev-parse", "HEAD"))
	b.WriteString(gitOut(t, dir, "rev-parse", "--abbrev-ref", "HEAD"))
	b.WriteString(gitOut(t, dir, "status", "--porcelain", "--", "code.go"))
	b.WriteString(gitOut(t, dir, "diff", "--cached"))
	for _, f := range files {
		data, err := os.ReadFile(filepath.Join(dir, f))
		if err != nil {
			t.Fatal(err)
		}
		b.WriteString(f + ":" + string(data) + "\n")
	}
	return b.String()
}

func recordRepo(t *testing.T) string {
	t.Helper()
	dir := t.TempDir()
	runCmd(t, "git", "-C", dir, "init", "-q")
	runCmd(t, "git", "-C", dir, "config", "user.email", "test@test.com")
	runCmd(t, "git", "-C", dir, "config", "user.name", "Test")
	os.WriteFile(filepath.Join(dir, "code.go"), []byte("package main\n"), 0644)
	runCmd(t, "git", "-C", dir, "add", "code.go")
	runCmd(t, "git", "-C", dir, "commit", "-q", "-m", "initial")
	// Uncommitted edit that keep would commit and discard would revert.
	os.WriteFile(filepath.Join(dir, "code.go"), []byte("package main\n// candidate\n"), 0644)
	return dir
}

func callTool(t *testing.T, h func(context.Context, mcp.CallToolRequest) (*mcp.CallToolResult, error), args map[string]interface{}) string {
	t.Helper()
	req := mcp.CallToolRequest{}
	req.Params.Arguments = args
	res, err := h(context.Background(), req)
	if err != nil {
		t.Fatalf("tool error: %v", err)
	}
	return res.Content[0].(mcp.TextContent).Text
}

func TestRecordOnlyLeavesTreeByteIdentical(t *testing.T) {
	dir := recordRepo(t)
	before := treeSnapshot(t, dir, "code.go")

	callTool(t, handleInitExperiment, map[string]interface{}{
		"name": "skill-eval", "metric_name": "lift", "metric_unit": "score",
		"direction": "higher_is_better", "working_directory": dir,
		"record_only":       true,
		"baseline_metric":   "baseline_pass_rate",
		"benchmark_command": "printf 'METRIC lift=0.2\\nMETRIC lift_sigma=0.05\\nMETRIC bytes=4096\\nMETRIC baseline_pass_rate=0.5\\n'",
	})
	// Scope the files by editing the header config the way a campaign file would.
	jsonl := filepath.Join(dir, "interlab.jsonl")
	st, _ := ReconstructState(jsonl)
	cfg := st.Config
	cfg.FilesInScope = []string{"code.go"}
	WriteConfigHeader(jsonl, cfg)

	for i := 0; i < 2; i++ {
		callTool(t, handleRunExperiment, map[string]interface{}{"working_directory": dir})
		out := callTool(t, handleLogExperiment, map[string]interface{}{
			"working_directory": dir, "decision": "record", "description": "arm pair"})
		if !strings.Contains(out, "working tree untouched") {
			t.Fatalf("unexpected log output: %s", out)
		}
	}
	// keep and discard are refused rather than acted on.
	callTool(t, handleRunExperiment, map[string]interface{}{"working_directory": dir})
	for _, d := range []string{"keep", "discard"} {
		out := callTool(t, handleLogExperiment, map[string]interface{}{
			"working_directory": dir, "decision": d, "description": "x"})
		if !strings.Contains(out, "record-only campaigns accept only") {
			t.Fatalf("%s should be refused in record-only: %s", d, out)
		}
	}

	if after := treeSnapshot(t, dir, "code.go"); after != before {
		t.Fatalf("working tree changed in record-only mode\nbefore:\n%s\nafter:\n%s", before, after)
	}
	if gitOut(t, dir, "branch", "--list", "interlab/*") != "" {
		t.Fatal("record-only must not create an interlab branch")
	}
}

func TestRecordOnlyRecordsPairedMetrics(t *testing.T) {
	dir := recordRepo(t)
	callTool(t, handleInitExperiment, map[string]interface{}{
		"name": "skill-eval", "metric_name": "lift", "metric_unit": "score",
		"direction": "higher_is_better", "working_directory": dir,
		"record_only": true, "baseline_metric": "baseline_pass_rate",
		"benchmark_command": "printf 'METRIC lift=0.2\\nMETRIC lift_sigma=0.05\\nMETRIC bytes=4096\\nMETRIC baseline_pass_rate=0.5\\n'",
	})
	out := callTool(t, handleRunExperiment, map[string]interface{}{"working_directory": dir})
	if !strings.Contains(out, "delta vs same-run baseline") {
		t.Fatalf("run output lacks same-run baseline delta: %s", out)
	}
	callTool(t, handleLogExperiment, map[string]interface{}{
		"working_directory": dir, "decision": "record", "description": "pair 1"})

	data, _ := os.ReadFile(filepath.Join(dir, "interlab.jsonl"))
	line := string(data)
	for _, want := range []string{`"decision":"record"`, `"metric_value":0.2`, `"lift_sigma":0.05`, `"bytes":4096`, `"baseline_value":0.5`, `"delta_vs_baseline":-0.3`} {
		if !strings.Contains(line, want) {
			t.Errorf("jsonl missing %s:\n%s", want, line)
		}
	}
	s, _ := ReconstructState(filepath.Join(dir, "interlab.jsonl"))
	if s.RecordedCount != 1 || s.RunCount != 1 || s.HasBaseline || s.KeptCount != 0 {
		t.Errorf("unexpected state: %+v", s)
	}
}

func TestRecordOnlyNeverTripsNoImprovementBreaker(t *testing.T) {
	dir := recordRepo(t)
	jsonl := filepath.Join(dir, "interlab.jsonl")
	WriteConfigHeader(jsonl, Config{Name: "n", MetricName: "lift", Direction: "higher_is_better",
		BenchmarkCommand: "echo METRIC lift=0.1", RecordOnly: true, MaxNoImprovement: 2})
	for i := 0; i < 5; i++ {
		WriteResult(jsonl, Result{Decision: "record", MetricValue: 0.1})
	}
	s, _ := ReconstructState(jsonl)
	if err := s.CheckCircuitBreaker(); err != nil {
		t.Fatalf("breaker tripped: %v", err)
	}
}

func TestRecordDecisionRejectedOutsideRecordOnly(t *testing.T) {
	dir := recordRepo(t)
	jsonl := filepath.Join(dir, "interlab.jsonl")
	WriteConfigHeader(jsonl, Config{Name: "n", MetricName: "m", Direction: "lower_is_better", BenchmarkCommand: "echo METRIC m=1", WorkingDirectory: dir})
	writeRunDetails(dir, RunDetails{Metrics: map[string]float64{"m": 1}})
	out := callTool(t, handleLogExperiment, map[string]interface{}{"working_directory": dir, "decision": "record", "description": "x"})
	if !strings.Contains(out, "must be 'keep', 'discard', or 'crash'") {
		t.Fatalf("default mode must reject record: %s", out)
	}
}

func TestParseMetricsSkipsNonFinite(t *testing.T) {
	m := parseMetrics("METRIC lift=NaN\nMETRIC lift_sigma=Inf\nMETRIC lift=-0.25\nMETRIC bytes=1e3\n")
	if m["lift"] != -0.25 || m["bytes"] != 1000 {
		t.Errorf("metrics = %v", m)
	}
	if _, ok := m["lift_sigma"]; ok || math.IsNaN(m["lift"]) {
		t.Errorf("non-finite value kept: %v", m)
	}
}
