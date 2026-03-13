package experiment

import (
	"testing"
)

func TestEmitExperimentEventNoIC(t *testing.T) {
	// When ic is not on PATH, should return nil (graceful degradation)
	cfg := Config{
		MetricName: "duration_ms",
		Direction:  "lower_is_better",
	}
	res := Result{
		Decision:    "keep",
		MetricValue: 42.0,
		DurationMs:  100,
		Description: "test",
	}

	// This should not error even if ic is not installed
	err := EmitExperimentEvent(cfg, res)
	if err != nil {
		t.Errorf("expected nil error when ic not found, got: %v", err)
	}
}

func TestCreateRunNoIC(t *testing.T) {
	// When ic is not on PATH, should return empty string and nil
	runID, err := CreateRun("test-bead")
	if err != nil {
		t.Errorf("expected nil error, got: %v", err)
	}
	// runID may be empty if ic is not installed — that's fine
	_ = runID
}
