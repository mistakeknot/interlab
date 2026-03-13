package experiment

import (
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

// CreateRun creates an ic run for the experiment campaign. Best-effort.
func CreateRun(beadID string) (string, error) {
	if _, err := exec.LookPath("ic"); err != nil {
		return "", nil
	}
	out, err := exec.Command("ic", "run", "create",
		fmt.Sprintf("--bead=%s", beadID),
		"--phase=Experiment",
	).Output()
	if err != nil {
		return "", nil // degrade gracefully
	}
	return strings.TrimSpace(string(out)), nil
}
