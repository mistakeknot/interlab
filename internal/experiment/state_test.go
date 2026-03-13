package experiment

import (
	"path/filepath"
	"testing"
)

func TestStateReconstructEmpty(t *testing.T) {
	dir := t.TempDir()
	s, err := ReconstructState(filepath.Join(dir, "interlab.jsonl"))
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if s.SegmentID != 0 {
		t.Errorf("expected segment 0, got %d", s.SegmentID)
	}
}

func TestStateWriteAndReconstruct(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "interlab.jsonl")

	cfg := Config{
		Name:             "test-campaign",
		MetricName:       "duration_ms",
		MetricUnit:       "ms",
		Direction:        "lower_is_better",
		BenchmarkCommand: "echo METRIC duration_ms=100",
	}
	if err := WriteConfigHeader(path, cfg); err != nil {
		t.Fatalf("write config: %v", err)
	}

	result := Result{
		Decision:    "keep",
		Description: "baseline",
		MetricValue: 100.0,
		DurationMs:  1500,
	}
	if err := WriteResult(path, result); err != nil {
		t.Fatalf("write result: %v", err)
	}

	s, err := ReconstructState(path)
	if err != nil {
		t.Fatalf("reconstruct: %v", err)
	}
	if s.SegmentID != 1 {
		t.Errorf("expected segment 1, got %d", s.SegmentID)
	}
	if s.RunCount != 1 {
		t.Errorf("expected 1 run, got %d", s.RunCount)
	}
	if s.BestMetric != 100.0 {
		t.Errorf("expected best 100.0, got %f", s.BestMetric)
	}
	if s.Config.Name != "test-campaign" {
		t.Errorf("expected name test-campaign, got %s", s.Config.Name)
	}
}

func TestStateCircuitBreaker(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "interlab.jsonl")

	cfg := Config{
		Name:             "crash-test",
		MetricName:       "score",
		MetricUnit:       "points",
		Direction:        "higher_is_better",
		BenchmarkCommand: "false",
		MaxExperiments:   5,
		MaxCrashes:       2,
	}
	WriteConfigHeader(path, cfg)

	// Write 2 crashes
	for i := 0; i < 2; i++ {
		WriteResult(path, Result{Decision: "crash", Description: "broken", DurationMs: 100})
	}

	s, _ := ReconstructState(path)
	err := s.CheckCircuitBreaker()
	if err == nil {
		t.Error("expected circuit breaker to trip after 2 consecutive crashes")
	}
}
