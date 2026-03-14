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

// PlanCampaignsTool is the MCP tool definition for plan_campaigns.
var PlanCampaignsTool = mcp.NewTool("plan_campaigns",
	mcp.WithDescription("Create a multi-campaign experiment plan. Accepts campaign specs, creates beads and working directories, validates file conflicts."),
	mcp.WithString("plan_json", mcp.Required(), mcp.Description("JSON object with goal, optional parent_bead_id, and campaigns array")),
	mcp.WithString("working_directory", mcp.Description("Base directory for campaign subdirectories (default: cwd)")),
)

// HandlePlanCampaigns implements the plan_campaigns tool.
func HandlePlanCampaigns(ctx context.Context, req mcp.CallToolRequest) (*mcp.CallToolResult, error) {
	planJSON := req.GetString("plan_json", "")
	if planJSON == "" {
		return mcp.NewToolResultText("plan_json is required"), nil
	}

	// Parse plan input.
	var input PlanInput
	if err := json.Unmarshal([]byte(planJSON), &input); err != nil {
		return mcp.NewToolResultText(fmt.Sprintf("invalid plan_json: %v", err)), nil
	}

	// Validate plan structure.
	if err := validatePlan(input); err != nil {
		return mcp.NewToolResultText(fmt.Sprintf("plan validation failed: %v", err)), nil
	}

	// Check for file conflicts between independent campaigns.
	conflicts := detectFileConflicts(input.Campaigns)
	if len(conflicts) > 0 {
		var b strings.Builder
		fmt.Fprintf(&b, "## File Conflicts Detected\n\n")
		fmt.Fprintf(&b, "The following files are modified by independent campaigns that could run in parallel:\n\n")
		for _, c := range conflicts {
			fmt.Fprintf(&b, "- **%s** touched by: %s\n", c.File, strings.Join(c.Campaigns, ", "))
		}
		fmt.Fprintf(&b, "\nResolve conflicts by adding `depends_on` edges or splitting files before proceeding.")
		return mcp.NewToolResultText(b.String()), nil
	}

	// Resolve working directory.
	workDir := req.GetString("working_directory", "")
	if workDir == "" {
		var err error
		workDir, err = os.Getwd()
		if err != nil {
			return nil, fmt.Errorf("get working directory: %w", err)
		}
	}

	// Create or reuse parent bead (best-effort).
	parentBeadID := input.ParentID
	if parentBeadID == "" {
		id, err := bdCreate(
			fmt.Sprintf("interlab: %s", input.Goal),
			input.Goal,
			"epic",
			3,
		)
		if err == nil && id != "" {
			parentBeadID = id
		}
		// Proceed without parent bead if bd is unavailable.
	}

	timestamp := time.Now().UTC().Format(time.RFC3339)
	var campaignResults []CampaignResult
	var childIDs []string

	for _, cs := range input.Campaigns {
		// Create campaign directory.
		campaignDir := filepath.Join(workDir, "campaigns", cs.Name)
		if err := os.MkdirAll(campaignDir, 0755); err != nil {
			return nil, fmt.Errorf("create campaign directory %s: %w", campaignDir, err)
		}

		// Write JSONL config header for the campaign.
		jsonlPath := filepath.Join(campaignDir, "interlab.jsonl")
		cfg := experiment.Config{
			Name:             cs.Name,
			MetricName:       cs.MetricName,
			MetricUnit:       cs.MetricUnit,
			Direction:        cs.Direction,
			BenchmarkCommand: cs.BenchmarkCommand,
			WorkingDirectory: workDir,
			FilesInScope:     cs.FilesInScope,
		}
		if err := experiment.WriteConfigHeader(jsonlPath, cfg); err != nil {
			return nil, fmt.Errorf("write config header for campaign %s: %w", cs.Name, err)
		}

		// Create child bead (best-effort).
		childBeadID := ""
		id, err := bdCreate(
			fmt.Sprintf("interlab/%s: %s", cs.Name, cs.MetricName),
			fmt.Sprintf("Campaign %s — %s (%s, %s)", cs.Name, cs.MetricName, cs.MetricUnit, cs.Direction),
			"task",
			3,
		)
		if err == nil && id != "" {
			childBeadID = id
			childIDs = append(childIDs, id)

			// Set campaign state on the child bead.
			bdSetState(childBeadID, "campaign_name", cs.Name)
			bdSetState(childBeadID, "campaign_dir", campaignDir)

			// Add dependency: child depends on parent.
			if parentBeadID != "" {
				bdDepAdd(childBeadID, parentBeadID)
			}
		}

		// Add depends_on edges between campaigns.
		if childBeadID != "" {
			for _, dep := range cs.DependsOn {
				// Find the bead ID of the dependency campaign.
				for _, prev := range campaignResults {
					if prev.Name == dep && prev.BeadID != "" {
						bdDepAdd(childBeadID, prev.BeadID)
					}
				}
			}
		}

		campaignResults = append(campaignResults, CampaignResult{
			Name:      cs.Name,
			BeadID:    childBeadID,
			Directory: campaignDir,
			DependsOn: cs.DependsOn,
		})
	}

	// Calculate max parallelism: count campaigns with no depends_on.
	parallelism := 0
	for _, cs := range input.Campaigns {
		if len(cs.DependsOn) == 0 {
			parallelism++
		}
	}

	// Store plan metadata on parent bead.
	if parentBeadID != "" {
		bdSetState(parentBeadID, "campaign_count", fmt.Sprintf("%d", len(input.Campaigns)))
		bdSetState(parentBeadID, "campaign_ids", strings.Join(childIDs, ","))
		bdSetState(parentBeadID, "plan_timestamp", timestamp)
	}

	// Build result.
	result := PlanResult{
		ParentBeadID: parentBeadID,
		Campaigns:    campaignResults,
		Parallelism:  parallelism,
		Conflicts:    conflicts,
	}

	// Marshal result JSON for embedding in response.
	resultJSON, err := json.MarshalIndent(result, "", "  ")
	if err != nil {
		return nil, fmt.Errorf("marshal result: %w", err)
	}

	// Build markdown summary.
	var b strings.Builder
	fmt.Fprintf(&b, "## Plan Created\n\n")
	fmt.Fprintf(&b, "**Goal:** %s\n", input.Goal)
	if parentBeadID != "" {
		fmt.Fprintf(&b, "**Parent bead:** %s\n", parentBeadID)
	}
	fmt.Fprintf(&b, "**Campaigns:** %d | **Max parallelism:** %d\n\n", len(campaignResults), parallelism)

	for _, cr := range campaignResults {
		fmt.Fprintf(&b, "- **%s** → `%s`", cr.Name, cr.Directory)
		if cr.BeadID != "" {
			fmt.Fprintf(&b, " (bead: %s)", cr.BeadID)
		}
		if len(cr.DependsOn) > 0 {
			fmt.Fprintf(&b, " [depends on: %s]", strings.Join(cr.DependsOn, ", "))
		}
		fmt.Fprintf(&b, "\n")
	}

	fmt.Fprintf(&b, "\n```json\n%s\n```\n", string(resultJSON))

	return mcp.NewToolResultText(b.String()), nil
}

// CampaignSpec describes a single experiment campaign within a plan.
type CampaignSpec struct {
	Name             string   `json:"name"`
	MetricName       string   `json:"metric_name"`
	MetricUnit       string   `json:"metric_unit"`
	Direction        string   `json:"direction"`
	BenchmarkCommand string   `json:"benchmark_command"`
	FilesInScope     []string `json:"files_in_scope"`
	DependsOn        []string `json:"depends_on"`
}

// PlanInput is the input to the plan_campaigns tool.
type PlanInput struct {
	Goal      string         `json:"goal"`
	ParentID  string         `json:"parent_bead_id,omitempty"`
	Campaigns []CampaignSpec `json:"campaigns"`
}

// PlanResult is the output of plan_campaigns.
type PlanResult struct {
	ParentBeadID string         `json:"parent_bead_id"`
	Campaigns    []CampaignResult `json:"campaigns"`
	Parallelism  int            `json:"parallelism"`
	Conflicts    []FileConflict `json:"conflicts"`
}

// CampaignResult describes a created campaign within a plan result.
type CampaignResult struct {
	Name      string   `json:"name"`
	BeadID    string   `json:"bead_id"`
	Directory string   `json:"directory"`
	DependsOn []string `json:"depends_on"`
}

// FileConflict describes a file touched by multiple independent campaigns.
type FileConflict struct {
	File       string   `json:"file"`
	Campaigns  []string `json:"campaigns"`
	Resolution string   `json:"resolution"`
}

// validatePlan checks that a PlanInput is well-formed.
func validatePlan(input PlanInput) error {
	if input.Goal == "" {
		return fmt.Errorf("goal is required")
	}
	if len(input.Campaigns) == 0 {
		return fmt.Errorf("at least 1 campaign is required")
	}

	names := make(map[string]bool, len(input.Campaigns))
	for _, c := range input.Campaigns {
		if names[c.Name] {
			return fmt.Errorf("duplicate campaign name: %q", c.Name)
		}
		names[c.Name] = true
	}

	for _, c := range input.Campaigns {
		if c.Direction != "lower_is_better" && c.Direction != "higher_is_better" {
			return fmt.Errorf("campaign %q: invalid direction %q (must be lower_is_better or higher_is_better)", c.Name, c.Direction)
		}
		if c.BenchmarkCommand == "" {
			return fmt.Errorf("campaign %q: benchmark_command is required", c.Name)
		}
		for _, dep := range c.DependsOn {
			if !names[dep] {
				return fmt.Errorf("campaign %q: depends_on references unknown campaign %q", c.Name, dep)
			}
		}
	}

	return nil
}

// detectFileConflicts finds files_in_scope overlaps between campaigns that
// could run in parallel (no transitive dependency path between them).
func detectFileConflicts(campaigns []CampaignSpec) []FileConflict {
	// Build name → index mapping.
	nameIdx := make(map[string]int, len(campaigns))
	for i, c := range campaigns {
		nameIdx[c.Name] = i
	}

	n := len(campaigns)

	// Build transitive closure of the dependency graph using Floyd-Warshall
	// on a reachability matrix. reachable[i][j] means campaign i transitively
	// depends on campaign j (or vice versa — we check both directions).
	reachable := make([][]bool, n)
	for i := range reachable {
		reachable[i] = make([]bool, n)
	}
	// Seed direct edges: if campaign i depends on campaign j, mark reachable[i][j].
	for i, c := range campaigns {
		for _, dep := range c.DependsOn {
			if j, ok := nameIdx[dep]; ok {
				reachable[i][j] = true
			}
		}
	}
	// Transitive closure.
	for k := 0; k < n; k++ {
		for i := 0; i < n; i++ {
			for j := 0; j < n; j++ {
				if reachable[i][k] && reachable[k][j] {
					reachable[i][j] = true
				}
			}
		}
	}

	// Two campaigns are independent if neither transitively depends on the other.
	independent := func(i, j int) bool {
		return !reachable[i][j] && !reachable[j][i]
	}

	// Build file → list of campaign indices.
	fileOwners := make(map[string][]int)
	for i, c := range campaigns {
		for _, f := range c.FilesInScope {
			fileOwners[f] = append(fileOwners[f], i)
		}
	}

	// For each file with multiple owners, check if any pair is independent.
	var conflicts []FileConflict
	for file, owners := range fileOwners {
		if len(owners) < 2 {
			continue
		}
		var independentCampaigns []string
		for a := 0; a < len(owners); a++ {
			for b := a + 1; b < len(owners); b++ {
				if independent(owners[a], owners[b]) {
					// Collect names of independent campaigns touching this file.
					aName := campaigns[owners[a]].Name
					bName := campaigns[owners[b]].Name
					// Deduplicate: use a set.
					found := make(map[string]bool)
					for _, name := range independentCampaigns {
						found[name] = true
					}
					if !found[aName] {
						independentCampaigns = append(independentCampaigns, aName)
					}
					if !found[bName] {
						independentCampaigns = append(independentCampaigns, bName)
					}
				}
			}
		}
		if len(independentCampaigns) > 0 {
			conflicts = append(conflicts, FileConflict{
				File:      file,
				Campaigns: independentCampaigns,
			})
		}
	}

	return conflicts
}
