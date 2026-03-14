package orchestration

import (
	"fmt"
	"os/exec"
	"strings"
)

// BeadStatus holds metadata returned by bd show.
type BeadStatus struct {
	ID     string
	Title  string
	Status string
}

// bdAvailable returns true if the bd CLI is on PATH.
func bdAvailable() bool {
	_, err := exec.LookPath("bd")
	return err == nil
}

// bdCreate creates a bead and returns the ID parsed from output.
// Best-effort: returns ("", nil) if bd is not available.
func bdCreate(title, description, beadType string, priority int) (string, error) {
	if !bdAvailable() {
		return "", nil
	}

	args := []string{"create",
		fmt.Sprintf("--title=%s", title),
		fmt.Sprintf("--type=%s", beadType),
		fmt.Sprintf("--priority=%d", priority),
	}
	if description != "" {
		args = append(args, fmt.Sprintf("--description=%s", description))
	}

	out, err := exec.Command("bd", args...).Output()
	if err != nil {
		return "", fmt.Errorf("bd create: %w", err)
	}

	// Parse ID from output line like: "✓ Created issue: ID — title"
	for _, line := range strings.Split(string(out), "\n") {
		if strings.Contains(line, "Created issue:") {
			parts := strings.SplitN(line, "Created issue:", 2)
			if len(parts) == 2 {
				rest := strings.TrimSpace(parts[1])
				// ID is before the em-dash
				if idx := strings.Index(rest, "—"); idx > 0 {
					return strings.TrimSpace(rest[:idx]), nil
				}
				// fallback: entire rest is the ID
				return rest, nil
			}
		}
	}

	return "", fmt.Errorf("bd create: could not parse ID from output: %s", string(out))
}

// bdDepAdd adds a dependency relationship (child depends on parent).
// Best-effort: returns nil if bd is not available.
func bdDepAdd(child, parent string) error {
	if !bdAvailable() {
		return nil
	}

	err := exec.Command("bd", "dep", "add", child, parent).Run()
	if err != nil {
		return fmt.Errorf("bd dep add: %w", err)
	}
	return nil
}

// bdSetState sets a key=value state on a bead. Best-effort: ignores errors.
func bdSetState(beadID, key, value string) error {
	if !bdAvailable() {
		return nil
	}

	// bd set-state <beadID> "key=value"
	cmd := exec.Command("bd", "set-state", beadID, fmt.Sprintf("%s=%s", key, value))
	cmd.Run() // best-effort: ignore errors
	return nil
}

// bdUpdateClaim claims a bead. Best-effort: returns nil if bd is not available.
func bdUpdateClaim(beadID string) error {
	if !bdAvailable() {
		return nil
	}

	err := exec.Command("bd", "update", beadID, "--claim").Run()
	if err != nil {
		return fmt.Errorf("bd update --claim: %w", err)
	}
	return nil
}

// bdClose closes a bead with a reason. Best-effort: returns nil if bd is not available.
func bdClose(beadID, reason string) error {
	if !bdAvailable() {
		return nil
	}

	args := []string{"close", beadID}
	if reason != "" {
		args = append(args, fmt.Sprintf("--reason=%s", reason))
	}

	err := exec.Command("bd", args...).Run()
	if err != nil {
		return fmt.Errorf("bd close: %w", err)
	}
	return nil
}

// bdGetState reads a state value for a bead. Returns "" if bd is not available or on error.
func bdGetState(beadID, key string) string {
	if !bdAvailable() {
		return ""
	}

	// bd state <beadID> <key>
	out, err := exec.Command("bd", "state", beadID, key).Output()
	if err != nil {
		return ""
	}
	result := strings.TrimSpace(string(out))
	// bd returns "(no <key> state set)" when no state exists
	if strings.HasPrefix(result, "(no ") {
		return ""
	}
	return result
}

// bdShow returns bead metadata. Returns (nil, nil) if bd is not available.
func bdShow(beadID string) (*BeadStatus, error) {
	if !bdAvailable() {
		return nil, nil
	}

	out, err := exec.Command("bd", "show", beadID).Output()
	if err != nil {
		return nil, fmt.Errorf("bd show: %w", err)
	}

	bs := &BeadStatus{ID: beadID}
	for _, line := range strings.Split(string(out), "\n") {
		line = strings.TrimSpace(line)
		if strings.HasPrefix(line, "Title:") {
			bs.Title = strings.TrimSpace(strings.TrimPrefix(line, "Title:"))
		} else if strings.HasPrefix(line, "Status:") {
			bs.Status = strings.TrimSpace(strings.TrimPrefix(line, "Status:"))
		}
	}
	return bs, nil
}
