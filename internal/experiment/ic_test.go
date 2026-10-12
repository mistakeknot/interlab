package experiment

import (
	"encoding/json"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
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
	t.Setenv("PATH", t.TempDir())
	runID, err := CreateRun("test-bead")
	if err != nil {
		t.Errorf("expected nil error, got: %v", err)
	}
	if runID != "" {
		t.Errorf("expected no run id without ic, got %q", runID)
	}
}

// fakeIC installs an executable named ic that runs body, alone on PATH.
func fakeIC(t *testing.T, body string) {
	t.Helper()
	dir := t.TempDir()
	script := "#!/bin/sh\n" + body + "\n"
	if err := os.WriteFile(filepath.Join(dir, "ic"), []byte(script), 0o755); err != nil {
		t.Fatal(err)
	}
	t.Setenv("PATH", dir)
}

func TestCreateRunReportsIcFailure(t *testing.T) {
	fakeIC(t, `echo "run create: --goal is required" >&2; exit 3`)
	runID, err := CreateRun("test-bead")
	if err == nil {
		t.Fatalf("expected an error when ic run create fails, got run id %q", runID)
	}
	if !strings.Contains(err.Error(), "--goal is required") {
		t.Errorf("error should carry ic's stderr, got: %v", err)
	}
	if runID != "" {
		t.Errorf("expected no run id on failure, got %q", runID)
	}
}

func TestCreateRunRejectsEmptyOutput(t *testing.T) {
	fakeIC(t, `echo "warning: nothing created" >&2; exit 0`)
	runID, err := CreateRun("test-bead")
	if err == nil {
		t.Fatalf("expected an error when ic prints no run id, got %q", runID)
	}
	if !strings.Contains(err.Error(), "nothing created") {
		t.Errorf("error should carry ic's stderr, got: %v", err)
	}
}

func TestCreateRunPassesWhatIcAccepts(t *testing.T) {
	// The flags ic run create accepts: --goal is required; the bead id is the scope.
	fakeIC(t, `[ "$1 $2" = "run create" ] || exit 4
shift 2
goal= scope=
for a in "$@"; do
  case "$a" in
    --goal=*) goal=${a#--goal=} ;;
    --scope-id=*) scope=${a#--scope-id=} ;;
    *) echo "unexpected flag $a" >&2; exit 3 ;;
  esac
done
[ -n "$goal" ] || { echo "--goal is required" >&2; exit 3; }
echo "run-for-$scope"`)
	runID, err := CreateRun("bead-7")
	if err != nil {
		t.Fatalf("CreateRun: %v", err)
	}
	if runID != "run-for-bead-7" {
		t.Errorf("run id = %q, want run-for-bead-7", runID)
	}
	if runID, err = CreateRun(""); err != nil || runID != "run-for-" {
		t.Errorf("empty bead id: got (%q, %v)", runID, err)
	}
}

func TestCreateRunAgainstRealIC(t *testing.T) {
	icPath, err := exec.LookPath("ic")
	if err != nil {
		t.Skip("ic not on PATH")
	}
	dir := t.TempDir()
	// A database in the working directory is found before any ancestor's.
	init := exec.Command(icPath, "--db=.clavain/intercore.db", "init")
	init.Dir = dir
	if out, err := init.CombinedOutput(); err != nil {
		t.Fatalf("cannot initialise a scratch ic database: %v: %s", err, out)
	}
	t.Chdir(dir)
	runID, err := CreateRun("test-bead")
	if err != nil {
		t.Fatalf("CreateRun against real ic: %v", err)
	}
	if runID == "" {
		t.Fatal("CreateRun returned no run id")
	}
	show, err := exec.Command(icPath, "--json", "run", "status", runID).Output()
	if err != nil {
		t.Fatalf("ic run status %s: %v", runID, err)
	}
	var status struct {
		ScopeID string `json:"scope_id"`
	}
	if err := json.Unmarshal(show, &status); err != nil {
		t.Fatalf("ic run status output is not JSON: %v: %s", err, show)
	}
	if status.ScopeID != "test-bead" {
		t.Errorf("run %s scope_id = %q, want test-bead", runID, status.ScopeID)
	}
}
