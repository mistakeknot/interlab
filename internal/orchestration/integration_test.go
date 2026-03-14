package orchestration

import (
	"os"
	"path/filepath"
	"testing"

	"github.com/mistakeknot/interlab/internal/experiment"
)

func TestPlanValidationAndConflicts(t *testing.T) {
	// Two campaigns with non-overlapping files should pass validation and have 0 conflicts.
	input := PlanInput{
		Goal: "optimize build pipeline",
		Campaigns: []CampaignSpec{
			{
				Name:             "speed",
				MetricName:       "latency",
				MetricUnit:       "ms",
				Direction:        "lower_is_better",
				BenchmarkCommand: "go test -bench .",
				FilesInScope:     []string{"engine.go", "engine_test.go"},
			},
			{
				Name:             "memory",
				MetricName:       "heap_bytes",
				MetricUnit:       "bytes",
				Direction:        "lower_is_better",
				BenchmarkCommand: "go test -bench BenchmarkMem",
				FilesInScope:     []string{"alloc.go", "pool.go"},
			},
		},
	}

	if err := validatePlan(input); err != nil {
		t.Fatalf("expected valid plan, got error: %v", err)
	}

	conflicts := detectFileConflicts(input.Campaigns)
	if len(conflicts) != 0 {
		t.Errorf("expected 0 conflicts for non-overlapping files, got %d: %+v", len(conflicts), conflicts)
	}

	// Add an overlapping file to campaign 2 — should produce 1 conflict.
	input.Campaigns[1].FilesInScope = append(input.Campaigns[1].FilesInScope, "engine.go")

	conflicts = detectFileConflicts(input.Campaigns)
	if len(conflicts) != 1 {
		t.Fatalf("expected 1 conflict after adding overlap, got %d: %+v", len(conflicts), conflicts)
	}
	if conflicts[0].File != "engine.go" {
		t.Errorf("expected conflict on engine.go, got %q", conflicts[0].File)
	}
	if len(conflicts[0].Campaigns) != 2 {
		t.Errorf("expected 2 campaigns in conflict, got %d", len(conflicts[0].Campaigns))
	}
}

func TestCampaignDirectoryCreation(t *testing.T) {
	root := t.TempDir()

	// Create a campaign subdirectory and write a config header into it.
	campaignDir := filepath.Join(root, "speed")
	if err := os.MkdirAll(campaignDir, 0755); err != nil {
		t.Fatalf("mkdir campaign dir: %v", err)
	}

	cfg := experiment.Config{
		Name:             "speed",
		MetricName:       "latency",
		MetricUnit:       "ms",
		Direction:        "lower_is_better",
		BenchmarkCommand: "go test -bench .",
	}

	jsonlPath := filepath.Join(campaignDir, "interlab.jsonl")
	if err := experiment.WriteConfigHeader(jsonlPath, cfg); err != nil {
		t.Fatalf("write config header: %v", err)
	}

	// Reconstruct state and verify.
	state, err := experiment.ReconstructState(jsonlPath)
	if err != nil {
		t.Fatalf("reconstruct state: %v", err)
	}
	if state.SegmentID != 1 {
		t.Errorf("expected segment 1, got %d", state.SegmentID)
	}
	if state.Config.Name != "speed" {
		t.Errorf("expected config name %q, got %q", "speed", state.Config.Name)
	}
}

func TestMultipleCampaignStateReconstruction(t *testing.T) {
	root := t.TempDir()

	// --- Campaign "speed": 2 keep results ---
	speedDir := filepath.Join(root, "speed")
	os.MkdirAll(speedDir, 0755)
	speedPath := filepath.Join(speedDir, "interlab.jsonl")

	experiment.WriteConfigHeader(speedPath, experiment.Config{
		Name:             "speed",
		MetricName:       "latency",
		MetricUnit:       "ms",
		Direction:        "lower_is_better",
		BenchmarkCommand: "bench.sh",
	})
	experiment.WriteResult(speedPath, experiment.Result{
		Decision: "keep", Description: "baseline", MetricValue: 1000, DurationMs: 500,
	})
	experiment.WriteResult(speedPath, experiment.Result{
		Decision: "keep", Description: "optimized hot path", MetricValue: 800, DurationMs: 450,
	})

	speedState, err := experiment.ReconstructState(speedPath)
	if err != nil {
		t.Fatalf("speed reconstruct: %v", err)
	}
	if speedState.RunCount != 2 {
		t.Errorf("speed: expected 2 runs, got %d", speedState.RunCount)
	}
	if speedState.KeptCount != 2 {
		t.Errorf("speed: expected 2 kept, got %d", speedState.KeptCount)
	}
	if speedState.BestMetric != 800 {
		t.Errorf("speed: expected best 800, got %f", speedState.BestMetric)
	}

	// --- Campaign "memory": 1 keep + 1 discard ---
	memDir := filepath.Join(root, "memory")
	os.MkdirAll(memDir, 0755)
	memPath := filepath.Join(memDir, "interlab.jsonl")

	experiment.WriteConfigHeader(memPath, experiment.Config{
		Name:             "memory",
		MetricName:       "heap_bytes",
		MetricUnit:       "bytes",
		Direction:        "lower_is_better",
		BenchmarkCommand: "mem-bench.sh",
	})
	experiment.WriteResult(memPath, experiment.Result{
		Decision: "keep", Description: "baseline", MetricValue: 5000, DurationMs: 200,
	})
	experiment.WriteResult(memPath, experiment.Result{
		Decision: "discard", Description: "regression", MetricValue: 6000, DurationMs: 210,
	})

	memState, err := experiment.ReconstructState(memPath)
	if err != nil {
		t.Fatalf("memory reconstruct: %v", err)
	}
	if memState.RunCount != 2 {
		t.Errorf("memory: expected 2 runs, got %d", memState.RunCount)
	}
	if memState.KeptCount != 1 {
		t.Errorf("memory: expected 1 kept, got %d", memState.KeptCount)
	}
	if memState.DiscardedCount != 1 {
		t.Errorf("memory: expected 1 discarded, got %d", memState.DiscardedCount)
	}

	// --- Campaign "startup": config header only, no results ---
	startupDir := filepath.Join(root, "startup")
	os.MkdirAll(startupDir, 0755)
	startupPath := filepath.Join(startupDir, "interlab.jsonl")

	experiment.WriteConfigHeader(startupPath, experiment.Config{
		Name:             "startup",
		MetricName:       "boot_ms",
		MetricUnit:       "ms",
		Direction:        "lower_is_better",
		BenchmarkCommand: "startup-bench.sh",
	})

	startupState, err := experiment.ReconstructState(startupPath)
	if err != nil {
		t.Fatalf("startup reconstruct: %v", err)
	}
	if startupState.RunCount != 0 {
		t.Errorf("startup: expected 0 runs, got %d", startupState.RunCount)
	}
	if startupState.SegmentID != 1 {
		t.Errorf("startup: expected segment 1, got %d", startupState.SegmentID)
	}
}
