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

// SynthesisReport is the final cross-campaign report.
type SynthesisReport struct {
	Goal      string            `json:"goal"`
	Campaigns []CampaignSummary `json:"campaigns"`
	Insights  []string          `json:"insights"`
}

// CampaignSummary holds the outcome of a single campaign within a synthesis.
type CampaignSummary struct {
	Name            string  `json:"name"`
	MetricName      string  `json:"metric_name"`
	MetricUnit      string  `json:"metric_unit"`
	Baseline        float64 `json:"baseline"`
	Best            float64 `json:"best"`
	ImprovementPct  float64 `json:"improvement_pct"`
	ExperimentCount int     `json:"experiment_count"`
	KeptCount       int     `json:"kept_count"`
	Status          string  `json:"status"` // "improved", "no_gain", "not_started"
}

// SynthesizeCampaignsTool is the MCP tool definition for synthesize_campaigns.
var SynthesizeCampaignsTool = mcp.NewTool("synthesize_campaigns",
	mcp.WithDescription("Generate a cross-campaign synthesis report from completed campaigns. Archives results and closes the parent bead."),
	mcp.WithString("parent_bead_id", mcp.Required(), mcp.Description("Parent bead ID from plan_campaigns")),
	mcp.WithString("working_directory", mcp.Description("Base directory containing campaigns/ (default: cwd)")),
)

// HandleSynthesizeCampaigns implements the synthesize_campaigns tool.
func HandleSynthesizeCampaigns(ctx context.Context, req mcp.CallToolRequest) (*mcp.CallToolResult, error) {
	parentBeadID := req.GetString("parent_bead_id", "")
	if parentBeadID == "" {
		return mcp.NewToolResultText("parent_bead_id is required"), nil
	}

	if !bdAvailable() {
		return mcp.NewToolResultText("bd CLI is not available — cannot synthesize campaigns without bead tracking"), nil
	}

	// Read campaign IDs from parent bead state.
	campaignIDsRaw := bdGetState(parentBeadID, "campaign_ids")
	if campaignIDsRaw == "" {
		return mcp.NewToolResultText(fmt.Sprintf("no campaign_ids state found on bead %s — was plan_campaigns used to create this plan?", parentBeadID)), nil
	}

	campaignIDs := strings.Split(campaignIDsRaw, ",")

	// Read goal from parent bead.
	goal := ""
	bs, err := bdShow(parentBeadID)
	if err == nil && bs != nil {
		goal = bs.Title
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

	var summaries []CampaignSummary

	for _, cid := range campaignIDs {
		cid = strings.TrimSpace(cid)
		if cid == "" {
			continue
		}

		// Read campaign metadata from bead state.
		campaignName := bdGetState(cid, "campaign_name")
		campaignDir := bdGetState(cid, "campaign_dir")

		summary := CampaignSummary{
			Name:   campaignName,
			Status: "not_started",
		}

		if campaignDir != "" {
			jsonlPath := filepath.Join(campaignDir, "interlab.jsonl")
			state, err := experiment.ReconstructState(jsonlPath)
			if err == nil && state != nil {
				summary.MetricName = state.Config.MetricName
				summary.MetricUnit = state.Config.MetricUnit
				summary.ExperimentCount = state.RunCount
				summary.KeptCount = state.KeptCount

				if state.HasBaseline {
					summary.Baseline = state.BaselineMetric
					summary.Best = state.BestMetric

					// Calculate improvement percentage.
					if summary.Baseline != 0 {
						if state.Config.Direction == "lower_is_better" {
							summary.ImprovementPct = (1 - summary.Best/summary.Baseline) * 100
						} else {
							summary.ImprovementPct = (summary.Best/summary.Baseline - 1) * 100
						}
					}

					if summary.ImprovementPct > 1 {
						summary.Status = "improved"
					} else {
						summary.Status = "no_gain"
					}
				} else if state.RunCount > 0 {
					summary.Status = "no_gain"
				}
			}
		}

		summaries = append(summaries, summary)
	}

	// Generate insights.
	insights := generateInsights(summaries)

	report := SynthesisReport{
		Goal:      goal,
		Campaigns: summaries,
		Insights:  insights,
	}

	// Archive: write report.json and synthesis.md to campaigns/synthesis/.
	synthesisDir := filepath.Join(workDir, "campaigns", "synthesis")
	if err := os.MkdirAll(synthesisDir, 0755); err != nil {
		return nil, fmt.Errorf("create synthesis directory: %w", err)
	}

	reportJSON, err := json.MarshalIndent(report, "", "  ")
	if err != nil {
		return nil, fmt.Errorf("marshal report: %w", err)
	}

	if err := os.WriteFile(filepath.Join(synthesisDir, "report.json"), reportJSON, 0644); err != nil {
		return nil, fmt.Errorf("write report.json: %w", err)
	}

	// Build markdown output.
	md := buildSynthesisMarkdown(report)

	if err := os.WriteFile(filepath.Join(synthesisDir, "synthesis.md"), []byte(md), 0644); err != nil {
		return nil, fmt.Errorf("write synthesis.md: %w", err)
	}

	// Close parent bead with summary reason.
	improvedCount := 0
	for _, s := range summaries {
		if s.Status == "improved" {
			improvedCount++
		}
	}
	reason := fmt.Sprintf("Synthesis complete: %d/%d campaigns improved", improvedCount, len(summaries))
	bdClose(parentBeadID, reason)

	return mcp.NewToolResultText(md), nil
}

// generateInsights produces human-readable observations from campaign summaries.
func generateInsights(summaries []CampaignSummary) []string {
	var insights []string

	improvedCount := 0
	noGainCount := 0
	notStartedCount := 0
	var bestImprovement float64
	var bestCampaign string

	for _, s := range summaries {
		switch s.Status {
		case "improved":
			improvedCount++
			if s.ImprovementPct > bestImprovement {
				bestImprovement = s.ImprovementPct
				bestCampaign = s.Name
			}
		case "no_gain":
			noGainCount++
		case "not_started":
			notStartedCount++
		}
	}

	total := len(summaries)

	if improvedCount == total {
		insights = append(insights, "All campaigns achieved measurable improvement.")
	} else if improvedCount > 0 {
		insights = append(insights, fmt.Sprintf("%d of %d campaigns achieved measurable improvement.", improvedCount, total))
	}

	if bestCampaign != "" {
		insights = append(insights, fmt.Sprintf("Best improvement: %s at %.1f%%.", bestCampaign, bestImprovement))
	}

	if noGainCount > 0 {
		insights = append(insights, fmt.Sprintf("%d campaign(s) showed no significant gain (<=1%% improvement).", noGainCount))
	}

	if notStartedCount > 0 {
		insights = append(insights, fmt.Sprintf("%d campaign(s) were never started.", notStartedCount))
	}

	if len(insights) == 0 {
		insights = append(insights, "No campaigns to analyze.")
	}

	return insights
}

// buildSynthesisMarkdown renders the synthesis report as a markdown string.
func buildSynthesisMarkdown(report SynthesisReport) string {
	var b strings.Builder

	fmt.Fprintf(&b, "## Synthesis Report\n\n")
	if report.Goal != "" {
		fmt.Fprintf(&b, "**Goal:** %s\n\n", report.Goal)
	}

	// Markdown table.
	fmt.Fprintf(&b, "| Campaign | Metric | Baseline | Best | Improvement | Experiments | Kept | Status |\n")
	fmt.Fprintf(&b, "|----------|--------|----------|------|-------------|-------------|------|--------|\n")

	for _, s := range report.Campaigns {
		statusIcon := "---"
		switch s.Status {
		case "improved":
			statusIcon = "improved"
		case "no_gain":
			statusIcon = "no_gain"
		case "not_started":
			statusIcon = "not_started"
		}

		metricLabel := s.MetricName
		if s.MetricUnit != "" {
			metricLabel = fmt.Sprintf("%s (%s)", s.MetricName, s.MetricUnit)
		}

		if s.Status == "not_started" {
			fmt.Fprintf(&b, "| %s | %s | — | — | — | %d | %d | %s |\n",
				s.Name, metricLabel, s.ExperimentCount, s.KeptCount, statusIcon)
		} else {
			fmt.Fprintf(&b, "| %s | %s | %.2f | %.2f | %.1f%% | %d | %d | %s |\n",
				s.Name, metricLabel, s.Baseline, s.Best, s.ImprovementPct,
				s.ExperimentCount, s.KeptCount, statusIcon)
		}
	}

	// Insights.
	if len(report.Insights) > 0 {
		fmt.Fprintf(&b, "\n### Insights\n\n")
		for _, insight := range report.Insights {
			fmt.Fprintf(&b, "- %s\n", insight)
		}
	}

	return b.String()
}
