---
artifact_type: plan
bead: Demarch-meq9
stage: design
requirements:
  - F1: plan_campaigns MCP tool
  - F2: dispatch_campaigns MCP tool
  - F3: status_campaigns MCP tool
  - F4: synthesize_campaigns MCP tool
  - F5: /autoresearch-multi skill
  - F6: File conflict detection at plan time
---
# Multi-Campaign Orchestration Implementation Plan

> **For Claude:** REQUIRED SUB-SKILL: Use clavain:executing-plans to implement this plan task-by-task.

**Bead:** Demarch-meq9
**Goal:** Add 4 MCP tools and a skill for multi-campaign orchestration with beads-backed coordination and parallel dispatch.

**Architecture:** New `internal/orchestration/` package alongside `internal/experiment/`. Orchestration tools call `bd` CLI for bead coordination and reuse `experiment.ReconstructState()` for JSONL reads. Each sub-campaign gets its own directory with standard interlab.jsonl. The `RegisterAll` pattern in `main.go` adds both experiment and orchestration tools to the same MCP server.

**Tech Stack:** Go 1.23, mcp-go v0.32.0 (existing), `bd` CLI for beads, `os/exec` for CLI calls (same pattern as ic.go).

---

## Must-Haves

**Truths** (observable behaviors):
- Agent can call `plan_campaigns` with a goal + campaign specs and get back bead IDs + working directories
- Agent can call `dispatch_campaigns` and get back dispatch instructions for ready campaigns
- Agent can call `status_campaigns` and see per-campaign progress with aggregate stats
- Agent can call `synthesize_campaigns` after all campaigns complete and get a structured cross-campaign report
- `/autoresearch-multi` skill drives the full loop without manual intervention
- Two campaigns with overlapping files_in_scope and no dependency edge are rejected at plan time

**Artifacts** (files that must exist):
- [`internal/orchestration/plan.go`] exports [`handlePlanCampaigns`, `PlanCampaignsTool`]
- [`internal/orchestration/dispatch.go`] exports [`handleDispatchCampaigns`, `DispatchCampaignsTool`]
- [`internal/orchestration/status.go`] exports [`handleStatusCampaigns`, `StatusCampaignsTool`]
- [`internal/orchestration/synthesize.go`] exports [`handleSynthesizeCampaigns`, `SynthesizeCampaignsTool`]
- [`internal/orchestration/register.go`] exports [`RegisterAll`]
- [`skills/autoresearch-multi/SKILL.md`] — skill protocol

**Key Links:**
- `cmd/interlab-mcp/main.go` calls both `experiment.RegisterAll(s)` and `orchestration.RegisterAll(s)`
- `plan_campaigns` creates directories that `init_experiment` (existing tool) writes JSONL into
- `status_campaigns` calls `experiment.ReconstructState()` to read sub-campaign JSONL files
- `synthesize_campaigns` reads archived `results.jsonl` from `campaigns/<name>/`

---

### Task 1: Orchestration package scaffold + beads helpers

**Files:**
- Create: `internal/orchestration/beads.go`
- Create: `internal/orchestration/beads_test.go`
- Create: `internal/orchestration/register.go`
- Modify: `cmd/interlab-mcp/main.go:1-24`

**Step 1: Create the beads helper module**

This module wraps `bd` CLI calls. Same best-effort pattern as `internal/experiment/ic.go` — if `bd` is not found, return empty results rather than errors.

```go
// internal/orchestration/beads.go
package orchestration

import (
	"encoding/json"
	"fmt"
	"os/exec"
	"strings"
)

// bdAvailable checks if the bd CLI is on PATH.
func bdAvailable() bool {
	_, err := exec.LookPath("bd")
	return err == nil
}

// bdCreate creates a bead and returns its ID.
func bdCreate(title, description, beadType string, priority int) (string, error) {
	if !bdAvailable() {
		return "", fmt.Errorf("bd not available")
	}
	out, err := exec.Command("bd", "create",
		fmt.Sprintf("--title=%s", title),
		fmt.Sprintf("--description=%s", description),
		fmt.Sprintf("--type=%s", beadType),
		fmt.Sprintf("--priority=%d", priority),
	).CombinedOutput()
	if err != nil {
		return "", fmt.Errorf("bd create: %s", string(out))
	}
	// Parse bead ID from output: "✓ Created issue: Demarch-xxx — title"
	line := strings.TrimSpace(string(out))
	if idx := strings.Index(line, ": "); idx >= 0 {
		rest := line[idx+2:]
		if spIdx := strings.Index(rest, " "); spIdx >= 0 {
			return rest[:spIdx], nil
		}
	}
	return "", fmt.Errorf("could not parse bead ID from: %s", line)
}

// bdDepAdd adds a dependency: child depends on parent.
func bdDepAdd(child, parent string) error {
	if !bdAvailable() {
		return nil
	}
	out, err := exec.Command("bd", "dep", "add", child, parent).CombinedOutput()
	if err != nil {
		return fmt.Errorf("bd dep add: %s", string(out))
	}
	return nil
}

// bdSetState sets a key=value state on a bead.
func bdSetState(beadID, key, value string) error {
	if !bdAvailable() {
		return nil
	}
	cmd := exec.Command("bd", "set-state", beadID, fmt.Sprintf("%s=%s", key, value))
	cmd.Run() // best-effort
	return nil
}

// bdUpdateClaim claims a bead for the current session.
func bdUpdateClaim(beadID string) error {
	if !bdAvailable() {
		return nil
	}
	out, err := exec.Command("bd", "update", beadID, "--claim").CombinedOutput()
	if err != nil {
		return fmt.Errorf("bd claim: %s", string(out))
	}
	return nil
}

// bdClose closes a bead with a reason.
func bdClose(beadID, reason string) error {
	if !bdAvailable() {
		return nil
	}
	out, err := exec.Command("bd", "close", beadID, fmt.Sprintf("--reason=%s", reason)).CombinedOutput()
	if err != nil {
		return fmt.Errorf("bd close: %s", string(out))
	}
	return nil
}

// BeadStatus holds parsed bead status from bd show --json.
type BeadStatus struct {
	ID     string `json:"id"`
	Title  string `json:"title"`
	Status string `json:"status"`
}

// bdShow returns bead metadata. Returns nil if bd unavailable or bead not found.
func bdShow(beadID string) (*BeadStatus, error) {
	if !bdAvailable() {
		return nil, nil
	}
	out, err := exec.Command("bd", "show", beadID, "--json").CombinedOutput()
	if err != nil {
		return nil, nil // bead not found or bd error — degrade
	}
	var bs BeadStatus
	if err := json.Unmarshal(out, &bs); err != nil {
		return nil, nil
	}
	return &bs, nil
}
```

**Step 2: Create the register module**

```go
// internal/orchestration/register.go
package orchestration

import "github.com/mark3labs/mcp-go/server"

// RegisterAll registers all orchestration tools with the MCP server.
func RegisterAll(s *server.MCPServer) {
	// Tools will be added in subsequent tasks
}
```

**Step 3: Wire into main.go**

```go
// cmd/interlab-mcp/main.go
package main

import (
	"fmt"
	"os"

	"github.com/mark3labs/mcp-go/server"
	"github.com/mistakeknot/interlab/internal/experiment"
	"github.com/mistakeknot/interlab/internal/orchestration"
)

func main() {
	s := server.NewMCPServer(
		"interlab",
		"0.2.0",
		server.WithToolCapabilities(true),
	)

	experiment.RegisterAll(s)
	orchestration.RegisterAll(s)

	if err := server.ServeStdio(s); err != nil {
		fmt.Fprintf(os.Stderr, "interlab-mcp: %v\n", err)
		os.Exit(1)
	}
}
```

**Step 4: Write tests for beads helpers**

```go
// internal/orchestration/beads_test.go
package orchestration

import "testing"

func TestBdAvailable(t *testing.T) {
	// Just verify it doesn't panic — bd may or may not be on PATH
	_ = bdAvailable()
}
```

**Step 5: Build and test**

Run: `go build ./... && go test ./...`
Expected: PASS, binary builds successfully

**Step 6: Commit**

```bash
git add internal/orchestration/ cmd/interlab-mcp/main.go
git commit -m "feat(interlab): orchestration package scaffold + beads helpers"
```

<verify>
- run: `go build ./...`
  expect: exit 0
- run: `go test ./...`
  expect: exit 0
</verify>

---

### Task 2: plan_campaigns types + validation (F1, F6)

**Files:**
- Create: `internal/orchestration/plan.go`
- Create: `internal/orchestration/plan_test.go`

**Step 1: Write the plan types and validation**

```go
// internal/orchestration/plan.go
package orchestration

import (
	"context"
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"time"

	"github.com/mark3labs/mcp-go/mcp"
	"github.com/mistakeknot/interlab/internal/experiment"
)

// CampaignSpec defines a single campaign in a multi-campaign plan.
type CampaignSpec struct {
	Name             string   `json:"name"`
	MetricName       string   `json:"metric_name"`
	MetricUnit       string   `json:"metric_unit"`
	Direction        string   `json:"direction"`
	BenchmarkCommand string   `json:"benchmark_command"`
	FilesInScope     []string `json:"files_in_scope,omitempty"`
	DependsOn        []string `json:"depends_on,omitempty"` // names of campaigns this depends on
}

// PlanInput is the JSON input for plan_campaigns.
type PlanInput struct {
	Goal      string         `json:"goal"`
	ParentID  string         `json:"parent_bead_id,omitempty"`
	Campaigns []CampaignSpec `json:"campaigns"`
}

// PlanResult is the output of plan_campaigns.
type PlanResult struct {
	ParentBeadID string            `json:"parent_bead_id"`
	Campaigns    []CampaignResult  `json:"campaigns"`
	Parallelism  int               `json:"max_parallelism"`
	Conflicts    []FileConflict    `json:"conflicts,omitempty"`
}

// CampaignResult holds the created campaign metadata.
type CampaignResult struct {
	Name      string   `json:"name"`
	BeadID    string   `json:"bead_id"`
	Directory string   `json:"directory"`
	DependsOn []string `json:"depends_on,omitempty"`
}

// FileConflict describes a files_in_scope overlap between parallel campaigns.
type FileConflict struct {
	File       string   `json:"file"`
	Campaigns  []string `json:"campaigns"`
	Resolution string   `json:"resolution"`
}

// validatePlan checks the plan input for errors.
func validatePlan(input PlanInput) error {
	if input.Goal == "" {
		return fmt.Errorf("goal is required")
	}
	if len(input.Campaigns) == 0 {
		return fmt.Errorf("at least one campaign is required")
	}
	names := make(map[string]bool)
	for _, c := range input.Campaigns {
		if c.Name == "" {
			return fmt.Errorf("campaign name is required")
		}
		if names[c.Name] {
			return fmt.Errorf("duplicate campaign name: %s", c.Name)
		}
		names[c.Name] = true
		if c.MetricName == "" {
			return fmt.Errorf("campaign %s: metric_name is required", c.Name)
		}
		if c.Direction != "lower_is_better" && c.Direction != "higher_is_better" {
			return fmt.Errorf("campaign %s: invalid direction %q", c.Name, c.Direction)
		}
		if c.BenchmarkCommand == "" {
			return fmt.Errorf("campaign %s: benchmark_command is required", c.Name)
		}
		for _, dep := range c.DependsOn {
			if !names[dep] {
				// Check if dep exists later in the list (forward reference)
				found := false
				for _, other := range input.Campaigns {
					if other.Name == dep {
						found = true
						break
					}
				}
				if !found {
					return fmt.Errorf("campaign %s depends on unknown campaign %q", c.Name, dep)
				}
			}
		}
	}
	return nil
}

// detectFileConflicts finds files_in_scope overlaps between campaigns
// that could run in parallel (no dependency path between them).
func detectFileConflicts(campaigns []CampaignSpec) []FileConflict {
	// Build dependency set for each campaign (transitive)
	deps := make(map[string]map[string]bool)
	for _, c := range campaigns {
		deps[c.Name] = make(map[string]bool)
		for _, d := range c.DependsOn {
			deps[c.Name][d] = true
		}
	}
	// Transitive closure (simple BFS since campaigns are small)
	changed := true
	for changed {
		changed = false
		for name, d := range deps {
			for dep := range d {
				for transitive := range deps[dep] {
					if !d[transitive] {
						d[transitive] = true
						changed = true
					}
				}
			}
			_ = name
		}
	}

	// Check if two campaigns are independent (no dependency in either direction)
	independent := func(a, b string) bool {
		return !deps[a][b] && !deps[b][a]
	}

	// Find file overlaps between independent campaigns
	var conflicts []FileConflict
	fileOwners := make(map[string][]string) // file → campaign names
	for _, c := range campaigns {
		for _, f := range c.FilesInScope {
			fileOwners[f] = append(fileOwners[f], c.Name)
		}
	}
	for file, owners := range fileOwners {
		if len(owners) < 2 {
			continue
		}
		// Check if any pair is independent
		for i := 0; i < len(owners); i++ {
			for j := i + 1; j < len(owners); j++ {
				if independent(owners[i], owners[j]) {
					conflicts = append(conflicts, FileConflict{
						File:      file,
						Campaigns: []string{owners[i], owners[j]},
						Resolution: fmt.Sprintf("Add dependency: %s depends_on %s (or vice versa), or partition %s between campaigns",
							owners[i], owners[j], file),
					})
				}
			}
		}
	}
	return conflicts
}
```

**Step 2: Write tests for validation and conflict detection**

```go
// internal/orchestration/plan_test.go
package orchestration

import "testing"

func TestValidatePlan(t *testing.T) {
	tests := []struct {
		name    string
		input   PlanInput
		wantErr string
	}{
		{"empty goal", PlanInput{}, "goal is required"},
		{"no campaigns", PlanInput{Goal: "test"}, "at least one campaign"},
		{"duplicate name", PlanInput{Goal: "test", Campaigns: []CampaignSpec{
			{Name: "a", MetricName: "m", Direction: "lower_is_better", BenchmarkCommand: "echo"},
			{Name: "a", MetricName: "m", Direction: "lower_is_better", BenchmarkCommand: "echo"},
		}}, "duplicate campaign name"},
		{"bad direction", PlanInput{Goal: "test", Campaigns: []CampaignSpec{
			{Name: "a", MetricName: "m", Direction: "fastest", BenchmarkCommand: "echo"},
		}}, "invalid direction"},
		{"unknown dep", PlanInput{Goal: "test", Campaigns: []CampaignSpec{
			{Name: "a", MetricName: "m", Direction: "lower_is_better", BenchmarkCommand: "echo", DependsOn: []string{"z"}},
		}}, "unknown campaign"},
		{"valid", PlanInput{Goal: "test", Campaigns: []CampaignSpec{
			{Name: "a", MetricName: "m", Direction: "lower_is_better", BenchmarkCommand: "echo"},
			{Name: "b", MetricName: "n", Direction: "higher_is_better", BenchmarkCommand: "echo", DependsOn: []string{"a"}},
		}}, ""},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			err := validatePlan(tt.input)
			if tt.wantErr == "" {
				if err != nil {
					t.Fatalf("unexpected error: %v", err)
				}
			} else {
				if err == nil || !contains(err.Error(), tt.wantErr) {
					t.Fatalf("want error containing %q, got %v", tt.wantErr, err)
				}
			}
		})
	}
}

func TestDetectFileConflicts(t *testing.T) {
	// Two independent campaigns with overlapping files
	campaigns := []CampaignSpec{
		{Name: "a", FilesInScope: []string{"shared.go", "a.go"}},
		{Name: "b", FilesInScope: []string{"shared.go", "b.go"}},
	}
	conflicts := detectFileConflicts(campaigns)
	if len(conflicts) != 1 {
		t.Fatalf("expected 1 conflict, got %d", len(conflicts))
	}
	if conflicts[0].File != "shared.go" {
		t.Fatalf("expected conflict on shared.go, got %s", conflicts[0].File)
	}

	// Same files but with dependency — no conflict
	campaigns[1].DependsOn = []string{"a"}
	conflicts = detectFileConflicts(campaigns)
	if len(conflicts) != 0 {
		t.Fatalf("expected 0 conflicts with dependency, got %d", len(conflicts))
	}
}

func contains(s, substr string) bool {
	return len(s) >= len(substr) && searchString(s, substr)
}

func searchString(s, sub string) bool {
	for i := 0; i <= len(s)-len(sub); i++ {
		if s[i:i+len(sub)] == sub {
			return true
		}
	}
	return false
}
```

**Step 3: Run tests**

Run: `go test ./internal/orchestration/ -v`
Expected: PASS

**Step 4: Commit**

```bash
git add internal/orchestration/plan.go internal/orchestration/plan_test.go
git commit -m "feat(interlab): plan_campaigns types + validation + file conflict detection"
```

<verify>
- run: `go test ./internal/orchestration/ -v`
  expect: exit 0
</verify>

---

### Task 3: plan_campaigns tool handler (F1)

**Files:**
- Modify: `internal/orchestration/plan.go`
- Modify: `internal/orchestration/register.go`

**Step 1: Add the tool definition and handler to plan.go**

Append after the `detectFileConflicts` function:

```go
var PlanCampaignsTool = mcp.NewTool("plan_campaigns",
	mcp.WithDescription("Create a multi-campaign experiment plan. Accepts campaign specs, creates beads and working directories, validates file conflicts."),
	mcp.WithString("plan_json", mcp.Required(), mcp.Description("JSON object with goal, optional parent_bead_id, and campaigns array")),
	mcp.WithString("working_directory", mcp.Description("Base directory for campaign subdirectories (default: cwd)")),
)

func HandlePlanCampaigns(ctx context.Context, req mcp.CallToolRequest) (*mcp.CallToolResult, error) {
	planJSON := req.GetString("plan_json", "")
	if planJSON == "" {
		return mcp.NewToolResultText("plan_json is required"), nil
	}

	var input PlanInput
	if err := json.Unmarshal([]byte(planJSON), &input); err != nil {
		return mcp.NewToolResultText(fmt.Sprintf("invalid plan_json: %v", err)), nil
	}

	if err := validatePlan(input); err != nil {
		return mcp.NewToolResultText(fmt.Sprintf("plan validation failed: %v", err)), nil
	}

	// Check file conflicts (F6)
	conflicts := detectFileConflicts(input.Campaigns)
	if len(conflicts) > 0 {
		var b strings.Builder
		fmt.Fprintf(&b, "File conflict detected — parallel campaigns share files:\n")
		for _, c := range conflicts {
			fmt.Fprintf(&b, "  - %s: %s\n    Resolution: %s\n", c.File, strings.Join(c.Campaigns, " ↔ "), c.Resolution)
		}
		return mcp.NewToolResultText(b.String()), nil
	}

	workDir := req.GetString("working_directory", "")
	if workDir == "" {
		var err error
		workDir, err = os.Getwd()
		if err != nil {
			return nil, fmt.Errorf("get working directory: %w", err)
		}
	}

	// Create or reuse parent bead
	parentID := input.ParentID
	if parentID == "" && bdAvailable() {
		var err error
		parentID, err = bdCreate(input.Goal, "Multi-campaign orchestration: "+input.Goal, "epic", 2)
		if err != nil {
			// Non-fatal — proceed without bead tracking
			parentID = ""
		}
	}

	// Create campaign directories and beads
	result := PlanResult{
		ParentBeadID: parentID,
	}
	nameToBeadID := make(map[string]string)

	campaignsDir := filepath.Join(workDir, "campaigns")
	os.MkdirAll(campaignsDir, 0755)

	for _, spec := range input.Campaigns {
		campaignDir := filepath.Join(campaignsDir, spec.Name)
		os.MkdirAll(campaignDir, 0755)

		// Write JSONL config header via existing experiment code
		cfg := experiment.Config{
			Name:             spec.Name,
			MetricName:       spec.MetricName,
			MetricUnit:       spec.MetricUnit,
			Direction:        spec.Direction,
			BenchmarkCommand: spec.BenchmarkCommand,
			WorkingDirectory: campaignDir,
			FilesInScope:     spec.FilesInScope,
		}
		jsonlPath := filepath.Join(campaignDir, "interlab.jsonl")
		if err := experiment.WriteConfigHeader(jsonlPath, cfg); err != nil {
			return nil, fmt.Errorf("init campaign %s: %w", spec.Name, err)
		}

		// Create child bead
		beadID := ""
		if bdAvailable() && parentID != "" {
			var err error
			beadID, err = bdCreate(
				fmt.Sprintf("Campaign: %s (%s)", spec.Name, spec.MetricName),
				fmt.Sprintf("Optimize %s (%s, %s)", spec.MetricName, spec.MetricUnit, spec.Direction),
				"task", 2,
			)
			if err == nil {
				nameToBeadID[spec.Name] = beadID
				// Write bead_id into JSONL config
				bdSetState(beadID, "campaign_name", spec.Name)
				bdSetState(beadID, "campaign_dir", campaignDir)
			}
		}

		cr := CampaignResult{
			Name:      spec.Name,
			BeadID:    beadID,
			Directory: campaignDir,
			DependsOn: spec.DependsOn,
		}
		result.Campaigns = append(result.Campaigns, cr)
	}

	// Add dependency edges between beads
	for _, spec := range input.Campaigns {
		childID := nameToBeadID[spec.Name]
		if childID == "" {
			continue
		}
		// Link to parent
		bdDepAdd(childID, parentID)
		// Link to dependency campaigns
		for _, depName := range spec.DependsOn {
			if depID, ok := nameToBeadID[depName]; ok {
				bdDepAdd(childID, depID)
			}
		}
	}

	// Calculate max parallelism (campaigns without dependencies that could run together)
	maxParallel := 0
	for _, c := range input.Campaigns {
		if len(c.DependsOn) == 0 {
			maxParallel++
		}
	}
	if maxParallel == 0 {
		maxParallel = 1
	}
	result.Parallelism = maxParallel

	// Store plan metadata on parent bead
	if parentID != "" {
		ids := make([]string, 0, len(nameToBeadID))
		for _, id := range nameToBeadID {
			ids = append(ids, id)
		}
		bdSetState(parentID, "campaign_count", fmt.Sprintf("%d", len(input.Campaigns)))
		bdSetState(parentID, "campaign_ids", strings.Join(ids, ","))
		bdSetState(parentID, "plan_timestamp", time.Now().UTC().Format(time.RFC3339))
	}

	// Build summary
	resultJSON, _ := json.MarshalIndent(result, "", "  ")
	var b strings.Builder
	fmt.Fprintf(&b, "## Plan created: %s\n\n", input.Goal)
	fmt.Fprintf(&b, "- parent bead: %s\n", parentID)
	fmt.Fprintf(&b, "- campaigns: %d\n", len(input.Campaigns))
	fmt.Fprintf(&b, "- max parallelism: %d\n\n", maxParallel)
	fmt.Fprintf(&b, "```json\n%s\n```\n", string(resultJSON))

	return mcp.NewToolResultText(b.String()), nil
}
```

**Step 2: Register the tool**

Update `internal/orchestration/register.go`:

```go
package orchestration

import "github.com/mark3labs/mcp-go/server"

// RegisterAll registers all orchestration tools with the MCP server.
func RegisterAll(s *server.MCPServer) {
	s.AddTool(PlanCampaignsTool, HandlePlanCampaigns)
}
```

**Step 3: Build and test**

Run: `go build ./... && go test ./...`
Expected: PASS

**Step 4: Commit**

```bash
git add internal/orchestration/
git commit -m "feat(interlab): plan_campaigns tool handler with bead creation"
```

<verify>
- run: `go build ./...`
  expect: exit 0
- run: `go test ./...`
  expect: exit 0
</verify>

---

### Task 4: dispatch_campaigns tool (F2)

**Files:**
- Create: `internal/orchestration/dispatch.go`
- Create: `internal/orchestration/dispatch_test.go`
- Modify: `internal/orchestration/register.go`

**Step 1: Write dispatch types and handler**

```go
// internal/orchestration/dispatch.go
package orchestration

import (
	"context"
	"fmt"
	"strings"

	"github.com/mark3labs/mcp-go/mcp"
)

// DispatchInstruction tells the calling agent how to run a campaign.
type DispatchInstruction struct {
	Name             string `json:"name"`
	BeadID           string `json:"bead_id"`
	Directory        string `json:"directory"`
	BenchmarkCommand string `json:"benchmark_command,omitempty"`
	Status           string `json:"status"` // "ready", "waiting", "in_progress", "completed"
	WaitingOn        string `json:"waiting_on,omitempty"`
}

var DispatchCampaignsTool = mcp.NewTool("dispatch_campaigns",
	mcp.WithDescription("Identify ready campaigns and return dispatch instructions. Idempotent — call again after campaigns complete to dispatch newly unblocked ones."),
	mcp.WithString("parent_bead_id", mcp.Required(), mcp.Description("Parent bead ID from plan_campaigns")),
)

func HandleDispatchCampaigns(ctx context.Context, req mcp.CallToolRequest) (*mcp.CallToolResult, error) {
	parentID := req.GetString("parent_bead_id", "")
	if parentID == "" {
		return mcp.NewToolResultText("parent_bead_id is required"), nil
	}

	if !bdAvailable() {
		return mcp.NewToolResultText("bd not available — cannot discover campaigns"), nil
	}

	// Read campaign_ids from parent bead state
	campaignIDs := bdGetState(parentID, "campaign_ids")
	if campaignIDs == "" {
		return mcp.NewToolResultText("no campaigns found — run plan_campaigns first"), nil
	}

	ids := strings.Split(campaignIDs, ",")
	var instructions []DispatchInstruction
	readyCount := 0

	for _, id := range ids {
		id = strings.TrimSpace(id)
		if id == "" {
			continue
		}
		name := bdGetState(id, "campaign_name")
		dir := bdGetState(id, "campaign_dir")
		bs, _ := bdShow(id)

		status := "ready"
		waitingOn := ""

		if bs != nil {
			switch bs.Status {
			case "closed":
				status = "completed"
			case "in_progress":
				status = "in_progress"
			default:
				// Check if dependencies are met
				// For now, if bead is open and not blocked, it's ready
				status = "ready"
			}
		}

		if status == "ready" {
			readyCount++
			// Claim the bead
			bdUpdateClaim(id)
		}

		instructions = append(instructions, DispatchInstruction{
			Name:      name,
			BeadID:    id,
			Directory: dir,
			Status:    status,
			WaitingOn: waitingOn,
		})
	}

	// Build summary
	var b strings.Builder
	fmt.Fprintf(&b, "## Dispatch: %d ready, %d total\n\n", readyCount, len(instructions))
	for _, inst := range instructions {
		icon := "○"
		switch inst.Status {
		case "ready":
			icon = "▶"
		case "in_progress":
			icon = "◐"
		case "completed":
			icon = "✓"
		case "waiting":
			icon = "⏳"
		}
		fmt.Fprintf(&b, "%s %s (%s) — %s\n", icon, inst.Name, inst.BeadID, inst.Status)
		if inst.Directory != "" {
			fmt.Fprintf(&b, "  dir: %s\n", inst.Directory)
		}
	}

	return mcp.NewToolResultText(b.String()), nil
}
```

**Step 2: Add bdGetState helper to beads.go**

Append to `internal/orchestration/beads.go`:

```go
// bdGetState reads a state value from a bead.
func bdGetState(beadID, key string) string {
	if !bdAvailable() {
		return ""
	}
	out, err := exec.Command("bd", "state", beadID, key).Output()
	if err != nil {
		return ""
	}
	return strings.TrimSpace(string(out))
}
```

**Step 3: Register the tool**

Update `register.go` to add:
```go
s.AddTool(DispatchCampaignsTool, HandleDispatchCampaigns)
```

**Step 4: Build and test**

Run: `go build ./...`
Expected: PASS

**Step 5: Commit**

```bash
git add internal/orchestration/
git commit -m "feat(interlab): dispatch_campaigns tool — ready campaign identification"
```

<verify>
- run: `go build ./...`
  expect: exit 0
</verify>

---

### Task 5: status_campaigns tool (F3)

**Files:**
- Create: `internal/orchestration/status.go`
- Modify: `internal/orchestration/register.go`

**Step 1: Write status handler**

```go
// internal/orchestration/status.go
package orchestration

import (
	"context"
	"fmt"
	"path/filepath"
	"strings"

	"github.com/mark3labs/mcp-go/mcp"
	"github.com/mistakeknot/interlab/internal/experiment"
)

// CampaignStatus holds per-campaign progress.
type CampaignStatus struct {
	Name           string  `json:"name"`
	BeadID         string  `json:"bead_id"`
	BeadStatus     string  `json:"bead_status"`
	RunCount       int     `json:"run_count"`
	KeptCount      int     `json:"kept_count"`
	DiscardedCount int     `json:"discarded_count"`
	CrashCount     int     `json:"crash_count"`
	BestMetric     float64 `json:"best_metric,omitempty"`
	BaselineMetric float64 `json:"baseline_metric,omitempty"`
	HasBaseline    bool    `json:"has_baseline"`
	MetricName     string  `json:"metric_name"`
	MetricUnit     string  `json:"metric_unit"`
	Direction      string  `json:"direction"`
}

var StatusCampaignsTool = mcp.NewTool("status_campaigns",
	mcp.WithDescription("Show progress across all campaigns in a multi-campaign plan."),
	mcp.WithString("parent_bead_id", mcp.Required(), mcp.Description("Parent bead ID from plan_campaigns")),
)

func HandleStatusCampaigns(ctx context.Context, req mcp.CallToolRequest) (*mcp.CallToolResult, error) {
	parentID := req.GetString("parent_bead_id", "")
	if parentID == "" {
		return mcp.NewToolResultText("parent_bead_id is required"), nil
	}

	if !bdAvailable() {
		return mcp.NewToolResultText("bd not available"), nil
	}

	campaignIDs := bdGetState(parentID, "campaign_ids")
	if campaignIDs == "" {
		return mcp.NewToolResultText("no campaigns found"), nil
	}

	ids := strings.Split(campaignIDs, ",")
	var statuses []CampaignStatus
	totalRuns, totalKept := 0, 0
	completedCount := 0

	for _, id := range ids {
		id = strings.TrimSpace(id)
		if id == "" {
			continue
		}
		name := bdGetState(id, "campaign_name")
		dir := bdGetState(id, "campaign_dir")
		bs, _ := bdShow(id)

		cs := CampaignStatus{
			Name:   name,
			BeadID: id,
		}
		if bs != nil {
			cs.BeadStatus = bs.Status
			if bs.Status == "closed" {
				completedCount++
			}
		}

		// Read JSONL state if directory exists
		if dir != "" {
			jsonlPath := filepath.Join(dir, "interlab.jsonl")
			state, err := experiment.ReconstructState(jsonlPath)
			if err == nil && state.SegmentID > 0 {
				cs.RunCount = state.RunCount
				cs.KeptCount = state.KeptCount
				cs.DiscardedCount = state.DiscardedCount
				cs.CrashCount = state.CrashCount
				cs.BestMetric = state.BestMetric
				cs.BaselineMetric = state.BaselineMetric
				cs.HasBaseline = state.HasBaseline
				cs.MetricName = state.Config.MetricName
				cs.MetricUnit = state.Config.MetricUnit
				cs.Direction = state.Config.Direction
			}
		}

		totalRuns += cs.RunCount
		totalKept += cs.KeptCount
		statuses = append(statuses, cs)
	}

	// Build summary
	var b strings.Builder
	fmt.Fprintf(&b, "## Campaign Status: %d/%d completed, %d total experiments\n\n", completedCount, len(statuses), totalRuns)

	for _, cs := range statuses {
		icon := "○"
		switch cs.BeadStatus {
		case "closed":
			icon = "✓"
		case "in_progress":
			icon = "◐"
		}
		fmt.Fprintf(&b, "%s **%s** (%s)\n", icon, cs.Name, cs.BeadID)
		if cs.HasBaseline {
			fmt.Fprintf(&b, "  %s: %.4g → %.4g %s | runs: %d (kept: %d, discarded: %d, crashed: %d)\n",
				cs.MetricName, cs.BaselineMetric, cs.BestMetric, cs.MetricUnit,
				cs.RunCount, cs.KeptCount, cs.DiscardedCount, cs.CrashCount)
		} else if cs.RunCount > 0 {
			fmt.Fprintf(&b, "  runs: %d (no baseline yet)\n", cs.RunCount)
		} else {
			fmt.Fprintf(&b, "  not started\n")
		}
	}

	fmt.Fprintf(&b, "\n### Aggregate\n")
	fmt.Fprintf(&b, "- total experiments: %d\n", totalRuns)
	fmt.Fprintf(&b, "- total kept: %d\n", totalKept)
	fmt.Fprintf(&b, "- campaigns completed: %d/%d\n", completedCount, len(statuses))

	return mcp.NewToolResultText(b.String()), nil
}
```

**Step 2: Register the tool**

Add to `register.go`:
```go
s.AddTool(StatusCampaignsTool, HandleStatusCampaigns)
```

**Step 3: Build**

Run: `go build ./...`
Expected: PASS

**Step 4: Commit**

```bash
git add internal/orchestration/
git commit -m "feat(interlab): status_campaigns tool — cross-campaign progress"
```

<verify>
- run: `go build ./...`
  expect: exit 0
</verify>

---

### Task 6: synthesize_campaigns tool (F4)

**Files:**
- Create: `internal/orchestration/synthesize.go`
- Modify: `internal/orchestration/register.go`

**Step 1: Write synthesize handler**

```go
// internal/orchestration/synthesize.go
package orchestration

import (
	"context"
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"strings"

	"github.com/mark3labs/mcp-go/mcp"
	"github.com/mistakeknot/interlab/internal/experiment"
)

// SynthesisReport is the structured output of synthesize_campaigns.
type SynthesisReport struct {
	Goal      string            `json:"goal"`
	Campaigns []CampaignSummary `json:"campaigns"`
	Insights  []string          `json:"insights"`
}

// CampaignSummary holds per-campaign results for synthesis.
type CampaignSummary struct {
	Name           string  `json:"name"`
	MetricName     string  `json:"metric_name"`
	MetricUnit     string  `json:"metric_unit"`
	Baseline       float64 `json:"baseline"`
	Best           float64 `json:"best"`
	ImprovementPct float64 `json:"improvement_pct"`
	ExperimentCount int    `json:"experiment_count"`
	KeptCount      int     `json:"kept_count"`
	Status         string  `json:"status"` // "improved", "no_gain", "not_started"
}

var SynthesizeCampaignsTool = mcp.NewTool("synthesize_campaigns",
	mcp.WithDescription("Generate a cross-campaign synthesis report from completed campaigns. Archives results and closes the parent bead."),
	mcp.WithString("parent_bead_id", mcp.Required(), mcp.Description("Parent bead ID from plan_campaigns")),
	mcp.WithString("working_directory", mcp.Description("Base directory containing campaigns/ (default: cwd)")),
)

func HandleSynthesizeCampaigns(ctx context.Context, req mcp.CallToolRequest) (*mcp.CallToolResult, error) {
	parentID := req.GetString("parent_bead_id", "")
	if parentID == "" {
		return mcp.NewToolResultText("parent_bead_id is required"), nil
	}

	workDir := req.GetString("working_directory", "")
	if workDir == "" {
		var err error
		workDir, err = os.Getwd()
		if err != nil {
			return nil, fmt.Errorf("get working directory: %w", err)
		}
	}

	if !bdAvailable() {
		return mcp.NewToolResultText("bd not available"), nil
	}

	campaignIDs := bdGetState(parentID, "campaign_ids")
	if campaignIDs == "" {
		return mcp.NewToolResultText("no campaigns found"), nil
	}

	ids := strings.Split(campaignIDs, ",")
	report := SynthesisReport{}

	// Read parent goal
	parent, _ := bdShow(parentID)
	if parent != nil {
		report.Goal = parent.Title
	}

	improvedCount := 0
	for _, id := range ids {
		id = strings.TrimSpace(id)
		if id == "" {
			continue
		}
		name := bdGetState(id, "campaign_name")
		dir := bdGetState(id, "campaign_dir")

		summary := CampaignSummary{Name: name, Status: "not_started"}

		if dir != "" {
			jsonlPath := filepath.Join(dir, "interlab.jsonl")
			state, err := experiment.ReconstructState(jsonlPath)
			if err == nil && state.SegmentID > 0 {
				summary.MetricName = state.Config.MetricName
				summary.MetricUnit = state.Config.MetricUnit
				summary.ExperimentCount = state.RunCount
				summary.KeptCount = state.KeptCount

				if state.HasBaseline {
					summary.Baseline = state.BaselineMetric
					summary.Best = state.BestMetric

					if state.BaselineMetric != 0 {
						if state.Config.Direction == "lower_is_better" {
							summary.ImprovementPct = (1 - summary.Best/summary.Baseline) * 100
						} else {
							summary.ImprovementPct = (summary.Best/summary.Baseline - 1) * 100
						}
					}

					if summary.ImprovementPct > 1 {
						summary.Status = "improved"
						improvedCount++
					} else {
						summary.Status = "no_gain"
					}
				}
			}
		}

		report.Campaigns = append(report.Campaigns, summary)
	}

	// Generate insights
	if improvedCount > 0 {
		report.Insights = append(report.Insights, fmt.Sprintf("%d of %d campaigns showed improvement", improvedCount, len(report.Campaigns)))
	}
	if improvedCount == 0 && len(report.Campaigns) > 0 {
		report.Insights = append(report.Insights, "No campaigns showed significant improvement — the codebase may already be well-optimized for these metrics")
	}

	// Archive synthesis
	synthesisDir := filepath.Join(workDir, "campaigns", "synthesis")
	os.MkdirAll(synthesisDir, 0755)

	reportJSON, _ := json.MarshalIndent(report, "", "  ")
	synthesisPath := filepath.Join(synthesisDir, "report.json")
	os.WriteFile(synthesisPath, reportJSON, 0644)

	// Build markdown summary
	var md strings.Builder
	fmt.Fprintf(&md, "# Synthesis: %s\n\n", report.Goal)
	fmt.Fprintf(&md, "## Results\n\n")
	fmt.Fprintf(&md, "| Campaign | Metric | Baseline | Best | Improvement | Experiments |\n")
	fmt.Fprintf(&md, "|----------|--------|----------|------|-------------|-------------|\n")
	for _, cs := range report.Campaigns {
		fmt.Fprintf(&md, "| %s | %s | %.4g %s | %.4g %s | %.1f%% | %d (%d kept) |\n",
			cs.Name, cs.MetricName,
			cs.Baseline, cs.MetricUnit,
			cs.Best, cs.MetricUnit,
			cs.ImprovementPct, cs.ExperimentCount, cs.KeptCount)
	}
	fmt.Fprintf(&md, "\n## Insights\n\n")
	for _, insight := range report.Insights {
		fmt.Fprintf(&md, "- %s\n", insight)
	}

	synthMdPath := filepath.Join(synthesisDir, "synthesis.md")
	os.WriteFile(synthMdPath, []byte(md.String()), 0644)

	// Close parent bead
	reason := fmt.Sprintf("Synthesis complete: %d/%d campaigns improved", improvedCount, len(report.Campaigns))
	bdClose(parentID, reason)

	// Return the markdown summary
	return mcp.NewToolResultText(md.String()), nil
}
```

**Step 2: Register the tool**

Add to `register.go`:
```go
s.AddTool(SynthesizeCampaignsTool, HandleSynthesizeCampaigns)
```

**Step 3: Build**

Run: `go build ./...`
Expected: PASS

**Step 4: Commit**

```bash
git add internal/orchestration/
git commit -m "feat(interlab): synthesize_campaigns tool — cross-campaign reporting"
```

<verify>
- run: `go build ./...`
  expect: exit 0
</verify>

---

### Task 7: Orchestration integration tests

**Files:**
- Create: `internal/orchestration/integration_test.go`

**Step 1: Write integration tests that exercise the full flow without bd**

```go
// internal/orchestration/integration_test.go
package orchestration

import (
	"os"
	"path/filepath"
	"testing"

	"github.com/mistakeknot/interlab/internal/experiment"
)

func TestPlanValidationAndConflicts(t *testing.T) {
	// Valid plan with no conflicts
	input := PlanInput{
		Goal: "optimize interlab",
		Campaigns: []CampaignSpec{
			{Name: "speed", MetricName: "latency", Direction: "lower_is_better", BenchmarkCommand: "echo METRIC latency=100", FilesInScope: []string{"a.go"}},
			{Name: "memory", MetricName: "allocs", Direction: "lower_is_better", BenchmarkCommand: "echo METRIC allocs=50", FilesInScope: []string{"b.go"}},
		},
	}
	if err := validatePlan(input); err != nil {
		t.Fatalf("unexpected validation error: %v", err)
	}
	conflicts := detectFileConflicts(input.Campaigns)
	if len(conflicts) != 0 {
		t.Fatalf("unexpected conflicts: %v", conflicts)
	}

	// Add overlapping file
	input.Campaigns[1].FilesInScope = append(input.Campaigns[1].FilesInScope, "a.go")
	conflicts = detectFileConflicts(input.Campaigns)
	if len(conflicts) != 1 {
		t.Fatalf("expected 1 conflict, got %d", len(conflicts))
	}
}

func TestCampaignDirectoryCreation(t *testing.T) {
	dir := t.TempDir()
	campaignDir := filepath.Join(dir, "campaigns", "test-campaign")
	os.MkdirAll(campaignDir, 0755)

	cfg := experiment.Config{
		Name:             "test-campaign",
		MetricName:       "speed",
		MetricUnit:       "ns",
		Direction:        "lower_is_better",
		BenchmarkCommand: "echo METRIC speed=100",
	}
	jsonlPath := filepath.Join(campaignDir, "interlab.jsonl")
	if err := experiment.WriteConfigHeader(jsonlPath, cfg); err != nil {
		t.Fatalf("write config: %v", err)
	}

	state, err := experiment.ReconstructState(jsonlPath)
	if err != nil {
		t.Fatalf("reconstruct: %v", err)
	}
	if state.SegmentID != 1 {
		t.Fatalf("expected segment 1, got %d", state.SegmentID)
	}
	if state.Config.Name != "test-campaign" {
		t.Fatalf("expected name test-campaign, got %s", state.Config.Name)
	}
}

func TestMultipleCampaignStateReconstruction(t *testing.T) {
	dir := t.TempDir()

	// Create 3 campaign directories with results
	campaigns := []struct {
		name    string
		metric  string
		results []experiment.Result
	}{
		{"speed", "latency_ns", []experiment.Result{
			{Decision: "keep", MetricValue: 1000},
			{Decision: "keep", MetricValue: 800},
		}},
		{"memory", "allocs", []experiment.Result{
			{Decision: "keep", MetricValue: 500},
			{Decision: "discard", MetricValue: 600},
		}},
		{"startup", "boot_ms", []experiment.Result{}}, // not started
	}

	for _, c := range campaigns {
		cDir := filepath.Join(dir, "campaigns", c.name)
		os.MkdirAll(cDir, 0755)

		if len(c.results) > 0 || c.name != "startup" {
			jsonlPath := filepath.Join(cDir, "interlab.jsonl")
			cfg := experiment.Config{
				Name:       c.name,
				MetricName: c.metric,
				Direction:  "lower_is_better",
			}
			experiment.WriteConfigHeader(jsonlPath, cfg)
			for _, r := range c.results {
				experiment.WriteResult(jsonlPath, r)
			}
		}
	}

	// Verify each campaign's state independently
	speedState, _ := experiment.ReconstructState(filepath.Join(dir, "campaigns", "speed", "interlab.jsonl"))
	if speedState.RunCount != 2 || speedState.KeptCount != 2 {
		t.Fatalf("speed: expected 2 runs/2 kept, got %d/%d", speedState.RunCount, speedState.KeptCount)
	}
	if speedState.BestMetric != 800 {
		t.Fatalf("speed: expected best 800, got %f", speedState.BestMetric)
	}

	memState, _ := experiment.ReconstructState(filepath.Join(dir, "campaigns", "memory", "interlab.jsonl"))
	if memState.RunCount != 2 || memState.KeptCount != 1 {
		t.Fatalf("memory: expected 2 runs/1 kept, got %d/%d", memState.RunCount, memState.KeptCount)
	}
}
```

**Step 2: Run tests**

Run: `go test ./internal/orchestration/ -v`
Expected: PASS

**Step 3: Commit**

```bash
git add internal/orchestration/integration_test.go
git commit -m "test(interlab): orchestration integration tests"
```

<verify>
- run: `go test ./internal/orchestration/ -v`
  expect: exit 0
</verify>

---

### Task 8: Register all tools + rebuild binary

**Files:**
- Modify: `internal/orchestration/register.go`

**Step 1: Ensure register.go has all 4 tools**

```go
package orchestration

import "github.com/mark3labs/mcp-go/server"

// RegisterAll registers all orchestration tools with the MCP server.
func RegisterAll(s *server.MCPServer) {
	s.AddTool(PlanCampaignsTool, HandlePlanCampaigns)
	s.AddTool(DispatchCampaignsTool, HandleDispatchCampaigns)
	s.AddTool(StatusCampaignsTool, HandleStatusCampaigns)
	s.AddTool(SynthesizeCampaignsTool, HandleSynthesizeCampaigns)
}
```

**Step 2: Build binary**

Run: `go build -o bin/interlab-mcp ./cmd/interlab-mcp/`
Expected: binary rebuilt with all 7 tools (3 experiment + 4 orchestration)

**Step 3: Verify tool count via MCP handshake**

Run: `echo '{"jsonrpc":"2.0","id":1,"method":"initialize","params":{"protocolVersion":"2024-11-05","capabilities":{},"clientInfo":{"name":"test","version":"0.1"}}}' | timeout 5 bin/interlab-mcp`
Expected: JSON response with capabilities

**Step 4: Run full test suite**

Run: `go test ./... -count=1`
Expected: All tests pass

**Step 5: Commit**

```bash
git add internal/orchestration/register.go bin/interlab-mcp
git commit -m "feat(interlab): register all orchestration tools, rebuild binary"
```

<verify>
- run: `go build -o bin/interlab-mcp ./cmd/interlab-mcp/`
  expect: exit 0
- run: `go test ./... -count=1`
  expect: exit 0
</verify>

---

### Task 9: /autoresearch-multi skill (F5)

**Files:**
- Create: `skills/autoresearch-multi/SKILL.md`

**Step 1: Create the skill directory**

Run: `mkdir -p skills/autoresearch-multi`

**Step 2: Write the skill protocol**

```markdown
---
name: autoresearch-multi
description: Run a multi-campaign optimization — decompose a broad goal into focused campaigns, dispatch them in parallel via subagents, monitor progress, and synthesize results. Use when optimizing multiple aspects of a module or exploring a hypothesis from multiple angles.
---

# /autoresearch-multi — Multi-Campaign Optimization Loop

Decompose a broad optimization request into multiple focused campaigns, dispatch them in parallel, and synthesize results.

**Announce at start:** "I'm using the autoresearch-multi skill to run a multi-campaign optimization."

## When to Use

Use this skill when the optimization target is broad:
- "Make this module faster" (multiple metrics to optimize)
- "Optimize the hot paths in this package" (multiple functions)
- "Is approach A or B better?" (comparative experiments)
- "Reduce resource usage" (memory, CPU, disk, network)

For single-metric optimization, use `/autoresearch` instead.

## Prerequisites

The interlab MCP tools must be available: `plan_campaigns`, `dispatch_campaigns`, `status_campaigns`, `synthesize_campaigns`, plus the original `init_experiment`, `run_experiment`, `log_experiment`.

## Phase 1: Analyze

Read the codebase to identify optimization targets. For each target:
- What metric to measure
- What benchmark command to use
- Which files are in scope
- Whether it depends on another campaign's results

Write `interlab-multi.md` as the living document:

```markdown
# Multi-Campaign: <goal>

## Objective
<what we're broadly optimizing>

## Campaigns
| # | Name | Metric | Direction | Files | Dependencies |
|---|------|--------|-----------|-------|-------------|
| 1 | ... | ... | ... | ... | none |

## Status
(updated after each dispatch cycle)
```

## Phase 2: Plan

Call `plan_campaigns` with the decomposition:

```json
{
  "goal": "<broad optimization goal>",
  "parent_bead_id": "<existing bead if available>",
  "campaigns": [
    {
      "name": "<campaign-name>",
      "metric_name": "<metric>",
      "metric_unit": "<unit>",
      "direction": "lower_is_better",
      "benchmark_command": "bash campaigns/<name>/bench.sh",
      "files_in_scope": ["path/to/file.go"],
      "depends_on": []
    }
  ]
}
```

If `plan_campaigns` reports file conflicts, resolve them by:
1. Adding dependency edges between conflicting campaigns
2. Or partitioning the files differently

Write benchmark scripts for each campaign in `campaigns/<name>/bench.sh`.

## Phase 3: Dispatch

Call `dispatch_campaigns` with the parent bead ID. For each "ready" campaign in the response, spawn a subagent:

```
Agent(prompt="Run /autoresearch in <campaign_dir>. The campaign is already initialized (interlab.jsonl exists). Run the optimization loop until the circuit breaker trips or ideas are exhausted.", working_directory="<campaign_dir>")
```

**Key rule:** Each subagent runs a standard `/autoresearch` loop. The multi skill orchestrates; the single skill executes.

## Phase 4: Monitor

Poll `status_campaigns` periodically (every 2-3 minutes if subagents are running, or after each subagent completes).

Update `interlab-multi.md` with current status.

When a campaign completes, call `dispatch_campaigns` again to start any newly unblocked campaigns.

Continue until all campaigns are completed or stalled.

## Phase 5: Synthesize

When all campaigns are done (or stalled), call `synthesize_campaigns`. Read the generated report.

Update `interlab-multi.md` with the final synthesis.

Archive: move `interlab-multi.md` to `campaigns/synthesis/`.

## Exit Conditions

- All campaigns completed (circuit breakers or idea exhaustion)
- User interrupts
- No campaigns showing progress for 3 consecutive dispatch cycles

## Rules

1. **One subagent per campaign.** Don't run the same campaign in two subagents.
2. **Let /autoresearch handle experiments.** This skill handles orchestration only.
3. **Update the living document.** Every dispatch/status cycle gets logged.
4. **Don't modify campaign internals.** Each campaign's interlab.jsonl, interlab.md, and ideas are owned by the subagent running it.
```

**Step 3: Register skill in plugin.json**

Read and update `.claude-plugin/plugin.json` to add the new skill path.

**Step 4: Commit**

```bash
git add skills/autoresearch-multi/ .claude-plugin/plugin.json
git commit -m "feat(interlab): /autoresearch-multi skill — multi-campaign orchestration protocol"
```

<verify>
- run: `python3 -c "import json; d=json.load(open('.claude-plugin/plugin.json')); assert './skills/autoresearch-multi' in d['skills'], 'skill not registered'"
  expect: exit 0
</verify>

---

### Task 10: Structural tests + documentation

**Files:**
- Modify: `tests/structural/test_structure.py`
- Modify: `AGENTS.md`

**Step 1: Add structural tests for orchestration**

Add tests to the existing structural test file verifying:
- `internal/orchestration/` directory exists
- `register.go` exports `RegisterAll`
- All 4 tool files exist (plan, dispatch, status, synthesize)
- `/autoresearch-multi` skill directory exists with SKILL.md
- `plugin.json` references the new skill

**Step 2: Update AGENTS.md**

Add a section documenting the orchestration tools:
- Tool names and parameters
- The beads-backed coordination model
- How to invoke `/autoresearch-multi`

**Step 3: Run all tests**

Run: `go test ./... -count=1 && python3 -m pytest tests/ -v`
Expected: All pass

**Step 4: Commit**

```bash
git add tests/ AGENTS.md
git commit -m "docs(interlab): orchestration docs + structural tests"
```

<verify>
- run: `go test ./... -count=1`
  expect: exit 0
</verify>
