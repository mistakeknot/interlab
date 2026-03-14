package orchestration

import (
	"context"
	"fmt"
	"strings"

	"github.com/mark3labs/mcp-go/mcp"
)

// DispatchInstruction describes a campaign ready for execution.
type DispatchInstruction struct {
	Name             string `json:"name"`
	BeadID           string `json:"bead_id"`
	Directory        string `json:"directory"`
	BenchmarkCommand string `json:"benchmark_command,omitempty"`
	Status           string `json:"status"`
	WaitingOn        string `json:"waiting_on,omitempty"`
}

// DispatchCampaignsTool is the MCP tool definition for dispatch_campaigns.
var DispatchCampaignsTool = mcp.NewTool("dispatch_campaigns",
	mcp.WithDescription("Identify ready campaigns and return dispatch instructions. Idempotent — call again after campaigns complete to dispatch newly unblocked ones."),
	mcp.WithString("parent_bead_id", mcp.Required(), mcp.Description("Parent bead ID from plan_campaigns")),
)

// HandleDispatchCampaigns implements the dispatch_campaigns tool.
func HandleDispatchCampaigns(ctx context.Context, req mcp.CallToolRequest) (*mcp.CallToolResult, error) {
	parentBeadID := req.GetString("parent_bead_id", "")
	if parentBeadID == "" {
		return mcp.NewToolResultText("parent_bead_id is required"), nil
	}

	if !bdAvailable() {
		return mcp.NewToolResultText("bd CLI is not available — cannot dispatch campaigns without bead tracking"), nil
	}

	// Read campaign IDs from parent bead state.
	campaignIDsRaw := bdGetState(parentBeadID, "campaign_ids")
	if campaignIDsRaw == "" {
		return mcp.NewToolResultText(fmt.Sprintf("no campaign_ids state found on parent bead %s — run plan_campaigns first", parentBeadID)), nil
	}

	campaignIDs := strings.Split(campaignIDsRaw, ",")

	var instructions []DispatchInstruction
	readyCount := 0
	inProgressCount := 0
	completedCount := 0
	waitingCount := 0

	for _, cid := range campaignIDs {
		cid = strings.TrimSpace(cid)
		if cid == "" {
			continue
		}

		// Read campaign metadata from bead state.
		name := bdGetState(cid, "campaign_name")
		dir := bdGetState(cid, "campaign_dir")

		// Get bead status.
		bs, err := bdShow(cid)
		if err != nil || bs == nil {
			instructions = append(instructions, DispatchInstruction{
				Name:   name,
				BeadID: cid,
				Status: "unknown",
			})
			continue
		}

		statusLower := strings.ToLower(bs.Status)

		var status string
		switch {
		case statusLower == "closed":
			status = "completed"
			completedCount++
		case statusLower == "in_progress" || statusLower == "in progress":
			status = "in_progress"
			inProgressCount++
		default:
			status = "ready"
			readyCount++
		}

		inst := DispatchInstruction{
			Name:      name,
			BeadID:    cid,
			Directory: dir,
			Status:    status,
		}

		// For ready campaigns, claim the bead.
		if status == "ready" {
			if err := bdUpdateClaim(cid); err != nil {
				// Non-fatal — still report as ready.
				inst.Status = "ready (claim failed)"
			}
		}

		instructions = append(instructions, inst)
	}

	// Check for waiting campaigns: "ready" campaigns whose dependencies are not yet completed.
	// Re-classify: a "ready" campaign with uncompleted deps is actually "waiting".
	// Build a set of completed bead IDs for fast lookup.
	completedSet := make(map[string]bool)
	for _, inst := range instructions {
		if inst.Status == "completed" {
			completedSet[inst.BeadID] = true
		}
	}

	// We don't have explicit dep info here, so waiting detection is based on
	// bead status only. All non-closed, non-in_progress beads are "ready" if
	// bd reports them as open. The bd update --claim call above ensures they
	// transition properly.
	// No reclassification needed — bead status is authoritative.

	waitingCount = len(instructions) - readyCount - inProgressCount - completedCount

	// Build markdown summary.
	var b strings.Builder
	fmt.Fprintf(&b, "## Dispatch Status\n\n")
	fmt.Fprintf(&b, "**Parent bead:** %s\n", parentBeadID)
	fmt.Fprintf(&b, "**Campaigns:** %d total | ", len(instructions))
	fmt.Fprintf(&b, "▶ %d ready | ◐ %d in_progress | ✓ %d completed", readyCount, inProgressCount, completedCount)
	if waitingCount > 0 {
		fmt.Fprintf(&b, " | ⏳ %d waiting", waitingCount)
	}
	fmt.Fprintf(&b, "\n\n")

	for _, inst := range instructions {
		var icon string
		switch inst.Status {
		case "ready":
			icon = "▶"
		case "in_progress":
			icon = "◐"
		case "completed":
			icon = "✓"
		default:
			icon = "⏳"
		}

		fmt.Fprintf(&b, "- %s **%s** (`%s`)", icon, inst.Name, inst.BeadID)
		if inst.Directory != "" {
			fmt.Fprintf(&b, " → `%s`", inst.Directory)
		}
		if inst.WaitingOn != "" {
			fmt.Fprintf(&b, " [waiting on: %s]", inst.WaitingOn)
		}
		fmt.Fprintf(&b, "\n")
	}

	if readyCount > 0 {
		fmt.Fprintf(&b, "\n**Next step:** Run experiments for the %d ready campaign(s) above.\n", readyCount)
	} else if inProgressCount > 0 {
		fmt.Fprintf(&b, "\n**Status:** All dispatchable campaigns are in progress. Call again after they complete.\n")
	} else if completedCount == len(instructions) {
		fmt.Fprintf(&b, "\n**All campaigns completed.** The experiment plan is done.\n")
	}

	return mcp.NewToolResultText(b.String()), nil
}
