package orchestration

import "fmt"

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
