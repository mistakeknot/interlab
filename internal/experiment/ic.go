package experiment

import (
	"bytes"
	"encoding/json"
	"fmt"
	"os/exec"
	"strings"
)

// EmitExperimentEvent records an experiment_outcome event via ic CLI.
// Best-effort: returns nil on ic not found.
func EmitExperimentEvent(cfg Config, res Result) error {
	if _, err := exec.LookPath("ic"); err != nil {
		return nil // ic not available, degrade gracefully
	}

	payload := map[string]interface{}{
		"metric_name":       cfg.MetricName,
		"metric_value":      res.MetricValue,
		"direction":         cfg.Direction,
		"decision":          res.Decision,
		"duration_ms":       res.DurationMs,
		"description":       res.Description,
		"secondary_metrics": res.SecondaryMetrics,
	}

	payloadJSON, _ := json.Marshal(payload)

	args := []string{"events", "record",
		"--source=interlab",
		"--type=experiment_outcome",
		fmt.Sprintf("--payload=%s", string(payloadJSON)),
	}
	if cfg.RunID != "" {
		args = append(args, fmt.Sprintf("--run=%s", cfg.RunID))
	}

	cmd := exec.Command("ic", args...)
	cmd.Run() // best-effort: ignore errors from ic
	return nil
}

// CreateRun creates an ic run for the experiment campaign and returns its id.
// The bead id is recorded as the run's scope. When ic is not on PATH it
// returns ("", nil) so campaigns still work without the kernel; once ic is
// found, a failed or empty create is an error, since the caller would
// otherwise carry on with no run to attach events to.
func CreateRun(beadID string) (string, error) {
	if _, err := exec.LookPath("ic"); err != nil {
		return "", nil // ic not available, degrade gracefully
	}
	goal := "interlab experiment campaign"
	args := []string{"run", "create"}
	if beadID != "" {
		goal += " " + beadID
		args = append(args, fmt.Sprintf("--scope-id=%s", beadID))
	}
	args = append(args, fmt.Sprintf("--goal=%s", goal))
	cmd := exec.Command("ic", args...)
	var stderr bytes.Buffer
	cmd.Stderr = &stderr
	out, err := cmd.Output()
	msg := strings.TrimSpace(stderr.String())
	if err != nil {
		if msg != "" {
			return "", fmt.Errorf("ic run create: %w: %s", err, msg)
		}
		return "", fmt.Errorf("ic run create: %w", err)
	}
	id := strings.TrimSpace(string(out))
	if id == "" {
		if msg != "" {
			return "", fmt.Errorf("ic run create: no run id printed: %s", msg)
		}
		return "", fmt.Errorf("ic run create: no run id printed")
	}
	return id, nil
}
