package mutation

import (
	"context"
	"encoding/json"
	"path/filepath"
	"testing"

	"github.com/mark3labs/mcp-go/mcp"
	"github.com/mark3labs/mcp-go/server"
)

func setupToolStore(t *testing.T) {
	t.Helper()
	dir := t.TempDir()
	store, err := NewStore(filepath.Join(dir, "test.db"))
	if err != nil {
		t.Fatalf("NewStore: %v", err)
	}
	globalStore = store
	t.Cleanup(func() {
		store.Close()
		globalStore = nil
	})
}

func TestToolRegistration(t *testing.T) {
	setupToolStore(t)
	s := server.NewMCPServer("test", "0.1.0", server.WithToolCapabilities(true))
	RegisterAll(s, globalStore)
}

func TestHandleMutationRecord(t *testing.T) {
	setupToolStore(t)

	req := mcp.CallToolRequest{}
	req.Params.Name = "mutation_record"
	req.Params.Arguments = map[string]any{
		"task_type":      "plugin-quality",
		"hypothesis":     "add docstrings to all exported functions",
		"quality_signal": 0.82,
		"session_id":     "test-session-1",
		"campaign_id":    "test-campaign",
	}

	result, err := handleMutationRecord(context.Background(), req)
	if err != nil {
		t.Fatalf("handleMutationRecord: %v", err)
	}

	text := result.Content[0].(mcp.TextContent).Text
	if text == "" {
		t.Error("expected non-empty response")
	}

	var resp map[string]any
	if err := json.Unmarshal([]byte(text), &resp); err != nil {
		t.Fatalf("response should be JSON: %v\nGot: %s", err, text)
	}
	if resp["is_new_best"] != true {
		t.Error("first mutation should be new best")
	}
}

func TestHandleMutationQuery(t *testing.T) {
	setupToolStore(t)

	// Record a mutation first
	recordReq := mcp.CallToolRequest{}
	recordReq.Params.Name = "mutation_record"
	recordReq.Params.Arguments = map[string]any{
		"task_type":      "plugin-quality",
		"hypothesis":     "test hypothesis",
		"quality_signal": 0.75,
	}
	handleMutationRecord(context.Background(), recordReq)

	// Query
	queryReq := mcp.CallToolRequest{}
	queryReq.Params.Name = "mutation_query"
	queryReq.Params.Arguments = map[string]any{
		"task_type": "plugin-quality",
	}

	result, err := handleMutationQuery(context.Background(), queryReq)
	if err != nil {
		t.Fatalf("handleMutationQuery: %v", err)
	}

	text := result.Content[0].(mcp.TextContent).Text
	var resp struct {
		Mutations []Mutation `json:"mutations"`
		Count     int        `json:"count"`
	}
	if err := json.Unmarshal([]byte(text), &resp); err != nil {
		t.Fatalf("response should be JSON: %v", err)
	}
	if resp.Count != 1 {
		t.Errorf("expected 1 mutation, got %d", resp.Count)
	}
}

func TestHandleMutationQueryEmpty(t *testing.T) {
	setupToolStore(t)

	queryReq := mcp.CallToolRequest{}
	queryReq.Params.Arguments = map[string]any{
		"task_type": "nonexistent",
	}

	result, err := handleMutationQuery(context.Background(), queryReq)
	if err != nil {
		t.Fatalf("handleMutationQuery: %v", err)
	}

	text := result.Content[0].(mcp.TextContent).Text
	// Verify JSON array is [] not null
	if !json.Valid([]byte(text)) {
		t.Fatalf("invalid JSON: %s", text)
	}
	var resp struct {
		Mutations []Mutation `json:"mutations"`
	}
	json.Unmarshal([]byte(text), &resp)
	if resp.Mutations == nil {
		t.Error("mutations should be empty array, not null")
	}
}

func TestHandleMutationRecordValidation(t *testing.T) {
	setupToolStore(t)

	// Missing required fields
	req := mcp.CallToolRequest{}
	req.Params.Arguments = map[string]any{
		"quality_signal": 0.5,
	}

	result, err := handleMutationRecord(context.Background(), req)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	text := result.Content[0].(mcp.TextContent).Text
	if text != "Error: task_type and hypothesis are required" {
		t.Errorf("expected validation error, got: %s", text)
	}
}
