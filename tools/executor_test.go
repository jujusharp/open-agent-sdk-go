package tools

import (
	"context"
	"testing"

	"github.com/jujusharp/open-agent-sdk-go/types"
)

type executorTestTool struct {
	name            string
	concurrencySafe bool
}

func (t executorTestTool) Name() string { return t.name }
func (t executorTestTool) Description() string {
	return t.name
}
func (t executorTestTool) InputSchema() types.ToolInputSchema {
	return types.ToolInputSchema{Type: "object"}
}
func (t executorTestTool) Call(_ context.Context, _ map[string]interface{}, _ *types.ToolUseContext) (*types.ToolResult, error) {
	return &types.ToolResult{
		Content: []types.ContentBlock{{Type: types.ContentBlockText, Text: t.name}},
	}, nil
}
func (t executorTestTool) IsConcurrencySafe(map[string]interface{}) bool { return t.concurrencySafe }
func (t executorTestTool) IsReadOnly(map[string]interface{}) bool        { return true }

func TestExecutorPreservesInputOrder(t *testing.T) {
	registry := NewRegistry()
	registry.Register(executorTestTool{name: "Skill", concurrencySafe: false})
	registry.Register(executorTestTool{name: "Read", concurrencySafe: true})

	executor := NewExecutor(registry, nil, testToolCtx(t))
	results := executor.RunTools(context.Background(), []ToolCallRequest{
		{ToolUseID: "1", ToolName: "Skill"},
		{ToolUseID: "2", ToolName: "Read"},
	})

	if len(results) != 2 {
		t.Fatalf("got %d results, want 2", len(results))
	}
	if results[0].ToolUseID != "1" || results[0].Result.Content[0].Text != "Skill" {
		t.Fatalf("first result mismatch: %+v", results[0])
	}
	if results[1].ToolUseID != "2" || results[1].Result.Content[0].Text != "Read" {
		t.Fatalf("second result mismatch: %+v", results[1])
	}
}
