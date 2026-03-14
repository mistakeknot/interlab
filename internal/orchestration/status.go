package orchestration

import (
	"context"
	"fmt"
	"path/filepath"
	"strings"

	"github.com/mark3labs/mcp-go/mcp"
	"github.com/mistakeknot/interlab/internal/experiment"
)

// CampaignStatus holds the status of a single campaign within a plan.
type CampaignStatus struct {
	Name           string  `json:"name"`
	BeadID         string  `json:"bead_id"`
	BeadStatus     string  `json:"bead_status"`
	RunCount       int     `json:"run_count"`
	KeptCount      int     `json:"kept_count"`
	DiscardedCount int     `json:"discarded_count"`
	CrashCount     int     `json:"crash_count"`
	BestMetric     float64 `json:"best_metric"`
	BaselineMetric float64 `json:"baseline_metric"`
	HasBaseline    bool    `json:"has_baseline"`
	MetricName     string  `json:"metric_name"`
	MetricUnit     string  `json:"metric_unit"`
	Direction      string  `json:"direction"`
}

// StatusCampaignsTool is the MCP tool definition for status_campaigns.
var StatusCampaignsTool = mcp.NewTool("status_campaigns",
	mcp.WithDescription("Show progress across all campaigns in a multi-campaign plan."),
	mcp.WithString("parent_bead_id", mcp.Required(), mcp.Description("Parent bead ID from plan_campaigns")),
)

// HandleStatusCampaigns implements the status_campaigns tool.
func HandleStatusCampaigns(ctx context.Context, req mcp.CallToolRequest) (*mcp.CallToolResult, error) {
	parentBeadID := req.GetString("parent_bead_id", "")
	if parentBeadID == "" {
		return mcp.NewToolResultText("parent_bead_id is required"), nil
	}

	if !bdAvailable() {
		return mcp.NewToolResultText("bd CLI is not available — cannot read campaign state"), nil
	}

	// Read campaign IDs from parent bead state.
	campaignIDsRaw := bdGetState(parentBeadID, "campaign_ids")
	if campaignIDsRaw == "" {
		return mcp.NewToolResultText(fmt.Sprintf("no campaign_ids state found on bead %s — was plan_campaigns used to create this plan?", parentBeadID)), nil
	}

	campaignIDs := strings.Split(campaignIDsRaw, ",")

	statuses := make([]CampaignStatus, 0, len(campaignIDs))
	totalRuns := 0
	totalKept := 0
	completedCount := 0

	for _, beadID := range campaignIDs {
		beadID = strings.TrimSpace(beadID)
		if beadID == "" {
			continue
		}

		cs := CampaignStatus{
			BeadID: beadID,
		}

		// Read bead metadata.
		bs, err := bdShow(beadID)
		if err == nil && bs != nil {
			cs.BeadStatus = bs.Status
			cs.Name = bs.Title
		}

		// Read campaign name and dir from bead state.
		campaignName := bdGetState(beadID, "campaign_name")
		if campaignName != "" {
			cs.Name = campaignName
		}
		campaignDir := bdGetState(beadID, "campaign_dir")

		// Read JSONL state if campaign_dir is available.
		if campaignDir != "" {
			jsonlPath := filepath.Join(campaignDir, "interlab.jsonl")
			state, err := experiment.ReconstructState(jsonlPath)
			if err == nil && state != nil {
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
		if cs.BeadStatus == "closed" || cs.BeadStatus == "done" {
			completedCount++
		}

		statuses = append(statuses, cs)
	}

	// Build markdown summary. Pre-size for ~256 bytes per campaign.
	var b strings.Builder
	b.Grow(256 * (len(statuses) + 1))
	b.WriteString("## Campaign Status\n\n")
	fmt.Fprintf(&b, "**Parent bead:** %s\n", parentBeadID)
	fmt.Fprintf(&b, "**Campaigns:** %d | **Completed:** %d | **Total runs:** %d | **Total kept:** %d\n\n", len(statuses), completedCount, totalRuns, totalKept)

	for i := range statuses {
		cs := &statuses[i]
		icon := statusIcon(cs.BeadStatus)
		b.WriteString("### ")
		b.WriteString(icon)
		b.WriteByte(' ')
		b.WriteString(cs.Name)
		b.WriteByte('\n')
		fmt.Fprintf(&b, "- **Bead:** %s (%s)\n", cs.BeadID, cs.BeadStatus)
		fmt.Fprintf(&b, "- **Runs:** %d — kept: %d, discarded: %d, crashed: %d\n", cs.RunCount, cs.KeptCount, cs.DiscardedCount, cs.CrashCount)

		if cs.HasBaseline {
			// For lower_is_better, improvement = baseline - best (positive means better).
			// For higher_is_better, improvement = best - baseline (positive means better).
			var improvement float64
			if cs.Direction == "lower_is_better" {
				improvement = cs.BaselineMetric - cs.BestMetric
			} else {
				improvement = cs.BestMetric - cs.BaselineMetric
			}
			pct := 0.0
			if cs.BaselineMetric != 0 {
				pct = (improvement / cs.BaselineMetric) * 100
			}
			dirLabel := "improvement"
			if pct < 0 {
				dirLabel = "regression"
				pct = -pct
			}
			fmt.Fprintf(&b, "- **%s:** %.2f → %.2f %s (%.1f%% %s)\n", cs.MetricName, cs.BaselineMetric, cs.BestMetric, cs.MetricUnit, pct, dirLabel)
		} else if cs.RunCount == 0 {
			b.WriteString("- *No runs yet*\n")
		}

		b.WriteByte('\n')
	}

	return mcp.NewToolResultText(b.String()), nil
}

// statusIcon returns an icon for the bead status.
func statusIcon(status string) string {
	switch status {
	case "closed", "done":
		return "[done]"
	case "in_progress":
		return "[running]"
	case "open":
		return "[pending]"
	default:
		return "[?]"
	}
}
