package mutation

import (
	"context"
	"encoding/json"
	"fmt"
	"os"

	"github.com/mark3labs/mcp-go/mcp"
	"github.com/mark3labs/mcp-go/server"
	"github.com/spf13/cast"
)

// Package-level store, injected via RegisterAll from main.go.
var globalStore *Store

var mutationRecordTool = mcp.NewTool("mutation_record",
	mcp.WithDescription("Record a mutation (approach attempt) with provenance metadata. Returns mutation ID, is_new_best status, and current best quality for this task type."),
	mcp.WithString("task_type", mcp.Required(), mcp.Description("Task category for cross-campaign queries (e.g., 'plugin-quality', 'agent-quality')")),
	mcp.WithString("hypothesis", mcp.Required(), mcp.Description("What approach was tried")),
	mcp.WithNumber("quality_signal", mcp.Required(), mcp.Description("Quality metric value (higher is better)")),
	mcp.WithString("session_id", mcp.Description("Session that produced this mutation (default: $CLAUDE_SESSION_ID)")),
	mcp.WithString("campaign_id", mcp.Description("Campaign this mutation belongs to")),
	mcp.WithString("inspired_by", mcp.Description("Session ID that inspired this approach (for provenance tracking)")),
	mcp.WithString("metadata", mcp.Description("JSON string of arbitrary key-value metadata")),
)

var mutationQueryTool = mcp.NewTool("mutation_query",
	mcp.WithDescription("Query mutation history with filters. Returns mutations sorted by quality (best first). Use at campaign start to seed hypotheses from prior approaches."),
	mcp.WithString("task_type", mcp.Description("Filter by task type")),
	mcp.WithString("campaign_id", mcp.Description("Filter by campaign")),
	mcp.WithBoolean("is_new_best", mcp.Description("If true, only return mutations that were new-best at time of recording")),
	mcp.WithNumber("min_quality", mcp.Description("Minimum quality_signal threshold")),
	mcp.WithString("inspired_by_session", mcp.Description("Filter mutations inspired by a specific session")),
	mcp.WithNumber("limit", mcp.Description("Max results to return (default: 20)")),
)

var mutationGenealogyTool = mcp.NewTool("mutation_genealogy",
	mcp.WithDescription("Trace inspiredBy provenance chains to visualize idea evolution. Returns a tree of mutations showing ancestry and descendants with quality signals."),
	mcp.WithNumber("mutation_id", mcp.Description("ID of the mutation to trace from")),
	mcp.WithString("session_id", mcp.Description("Session ID to find the most recent mutation for")),
	mcp.WithNumber("max_depth", mcp.Description("Maximum traversal depth (default: 10)")),
)

// RegisterAll registers mutation tools. Store must be opened once in main.go.
func RegisterAll(s *server.MCPServer, store *Store) {
	globalStore = store
	s.AddTool(mutationRecordTool, handleMutationRecord)
	s.AddTool(mutationQueryTool, handleMutationQuery)
	s.AddTool(mutationGenealogyTool, handleMutationGenealogy)
}

// argFloat64 extracts a float64 from MCP arguments (JSON numbers arrive as float64).
func argFloat64(args map[string]any, key string, def float64) float64 {
	v, ok := args[key]
	if !ok {
		return def
	}
	return cast.ToFloat64(v)
}

// argBool extracts a bool from MCP arguments.
func argBool(args map[string]any, key string, def bool) bool {
	v, ok := args[key]
	if !ok {
		return def
	}
	return cast.ToBool(v)
}

func handleMutationRecord(ctx context.Context, req mcp.CallToolRequest) (*mcp.CallToolResult, error) {
	args := req.GetArguments()
	taskType := req.GetString("task_type", "")
	hypothesis := req.GetString("hypothesis", "")
	qualitySignal := argFloat64(args, "quality_signal", 0)

	if taskType == "" || hypothesis == "" {
		return mcp.NewToolResultText("Error: task_type and hypothesis are required"), nil
	}

	sessionID := req.GetString("session_id", "")
	if sessionID == "" {
		sessionID = os.Getenv("CLAUDE_SESSION_ID")
	}

	var meta map[string]string
	if metaStr := req.GetString("metadata", ""); metaStr != "" {
		json.Unmarshal([]byte(metaStr), &meta)
	}

	id, isNewBest, bestQuality, err := globalStore.Record(RecordParams{
		SessionID:     sessionID,
		CampaignID:    req.GetString("campaign_id", ""),
		TaskType:      taskType,
		Hypothesis:    hypothesis,
		QualitySignal: qualitySignal,
		InspiredBy:    req.GetString("inspired_by", ""),
		Metadata:      meta,
	})
	if err != nil {
		return nil, fmt.Errorf("recording mutation: %w", err)
	}

	resp, _ := json.Marshal(map[string]any{
		"mutation_id":  id,
		"is_new_best":  isNewBest,
		"best_quality": bestQuality,
		"task_type":    taskType,
	})
	return mcp.NewToolResultText(string(resp)), nil
}

func handleMutationQuery(ctx context.Context, req mcp.CallToolRequest) (*mcp.CallToolResult, error) {
	args := req.GetArguments()
	params := QueryParams{
		TaskType:          req.GetString("task_type", ""),
		CampaignID:        req.GetString("campaign_id", ""),
		InspiredBySession: req.GetString("inspired_by_session", ""),
		MinQuality:        argFloat64(args, "min_quality", 0),
		Limit:             int(argFloat64(args, "limit", 20)),
	}

	if isNewBest := argBool(args, "is_new_best", false); isNewBest {
		params.IsNewBestOnly = &isNewBest
	}

	mutations, err := globalStore.Query(params)
	if err != nil {
		return nil, fmt.Errorf("querying mutations: %w", err)
	}

	resp, _ := json.Marshal(map[string]any{
		"mutations": mutations,
		"count":     len(mutations),
	})
	return mcp.NewToolResultText(string(resp)), nil
}

func handleMutationGenealogy(ctx context.Context, req mcp.CallToolRequest) (*mcp.CallToolResult, error) {
	args := req.GetArguments()
	mutationID := int64(argFloat64(args, "mutation_id", 0))
	sessionID := req.GetString("session_id", "")

	if mutationID == 0 && sessionID == "" {
		return mcp.NewToolResultText("Error: must provide mutation_id or session_id"), nil
	}

	tree, err := globalStore.Genealogy(GenealogyParams{
		MutationID: mutationID,
		SessionID:  sessionID,
		MaxDepth:   int(argFloat64(args, "max_depth", 10)),
	})
	if err != nil {
		return nil, fmt.Errorf("tracing genealogy: %w", err)
	}

	resp, _ := json.Marshal(tree)
	return mcp.NewToolResultText(string(resp)), nil
}
