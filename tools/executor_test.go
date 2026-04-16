package tools

import (
	"context"
	"testing"

	"github.com/jujusharp/open-agent-sdk-go/types"
)

type executorTestTool struct {
	name            string
	concurrencySafe bool
	calls           *int
}

func (t executorTestTool) Name() string { return t.name }
func (t executorTestTool) Description() string {
	return t.name
}
func (t executorTestTool) InputSchema() types.ToolInputSchema {
	return types.ToolInputSchema{Type: "object"}
}
func (t executorTestTool) Call(_ context.Context, _ map[string]interface{}, _ *types.ToolUseContext) (*types.ToolResult, error) {
	if t.calls != nil {
		*t.calls++
	}
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

func TestExecutorDeniesAskWithoutPermissionPrompt(t *testing.T) {
	calls := 0
	registry := NewRegistry()
	registry.Register(executorTestTool{name: "Bash", calls: &calls})

	executor := NewExecutor(registry, func(types.Tool, map[string]interface{}) (*types.PermissionDecision, error) {
		return &types.PermissionDecision{
			Behavior: types.PermissionAsk,
			Reason:   "confirm command",
		}, nil
	}, testToolCtx(t))

	results := executor.RunTools(context.Background(), []ToolCallRequest{
		{ToolUseID: "1", ToolName: "Bash", Input: map[string]interface{}{"command": "rm -rf /tmp/demo"}},
	})

	if calls != 0 {
		t.Fatalf("tool was executed %d times; ask without prompt handler must not execute", calls)
	}
	if len(results) != 1 {
		t.Fatalf("got %d results, want 1", len(results))
	}
	if !results[0].Result.IsError {
		t.Fatalf("expected permission error result, got %+v", results[0].Result)
	}
}

func TestExecutorRunsAskWhenPermissionPromptAllows(t *testing.T) {
	calls := 0
	registry := NewRegistry()
	registry.Register(executorTestTool{name: "Bash", calls: &calls})

	executor := NewExecutorWithPermissionPrompt(
		registry,
		func(types.Tool, map[string]interface{}) (*types.PermissionDecision, error) {
			return &types.PermissionDecision{Behavior: types.PermissionAsk, Reason: "confirm command"}, nil
		},
		func(_ context.Context, req types.PermissionPromptRequest) (*types.PermissionDecision, error) {
			if req.ToolName != "Bash" {
				t.Fatalf("unexpected prompt tool name %q", req.ToolName)
			}
			if req.Reason != "confirm command" {
				t.Fatalf("unexpected prompt reason %q", req.Reason)
			}
			return &types.PermissionDecision{Behavior: types.PermissionAllow}, nil
		},
		testToolCtx(t),
	)

	results := executor.RunTools(context.Background(), []ToolCallRequest{
		{ToolUseID: "1", ToolName: "Bash", Input: map[string]interface{}{"command": "echo ok"}},
	})

	if calls != 1 {
		t.Fatalf("tool was executed %d times, want 1", calls)
	}
	if len(results) != 1 || results[0].Result.IsError {
		t.Fatalf("expected successful result, got %+v", results)
	}
}

func TestExecutorDeniesAskWhenPermissionPromptDenies(t *testing.T) {
	calls := 0
	registry := NewRegistry()
	registry.Register(executorTestTool{name: "Bash", calls: &calls})

	executor := NewExecutorWithPermissionPrompt(
		registry,
		func(types.Tool, map[string]interface{}) (*types.PermissionDecision, error) {
			return &types.PermissionDecision{Behavior: types.PermissionAsk}, nil
		},
		func(context.Context, types.PermissionPromptRequest) (*types.PermissionDecision, error) {
			return &types.PermissionDecision{Behavior: types.PermissionDeny, Reason: "no"}, nil
		},
		testToolCtx(t),
	)

	results := executor.RunTools(context.Background(), []ToolCallRequest{
		{ToolUseID: "1", ToolName: "Bash", Input: map[string]interface{}{"command": "echo ok"}},
	})

	if calls != 0 {
		t.Fatalf("tool was executed %d times; denied prompt must not execute", calls)
	}
	if len(results) != 1 || !results[0].Result.IsError {
		t.Fatalf("expected permission error result, got %+v", results)
	}
}
