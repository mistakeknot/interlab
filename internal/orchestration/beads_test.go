package orchestration

import (
	"testing"
)

func TestBdAvailableNoPanic(t *testing.T) {
	// bdAvailable should never panic regardless of whether bd is installed.
	avail := bdAvailable()
	// Just log the result — we can't assert either way since bd may or may not be on PATH.
	t.Logf("bdAvailable() = %v", avail)
}
