package agent

import (
	"context"
	"strings"
	"testing"

	"github.com/codeany-ai/open-agent-sdk-go/skills"
	"github.com/codeany-ai/open-agent-sdk-go/tools"
	"github.com/codeany-ai/open-agent-sdk-go/types"
)

type fakeTool struct{ name string }

func (t fakeTool) Name() string { return t.name }
func (t fakeTool) Description() string {
	return t.name + " tool"
}
func (t fakeTool) InputSchema() types.ToolInputSchema {
	return types.ToolInputSchema{Type: "object"}
}
func (t fakeTool) Call(context.Context, map[string]interface{}, *types.ToolUseContext) (*types.ToolResult, error) {
	return textToolResult("ok"), nil
}
func (t fakeTool) IsConcurrencySafe(map[string]interface{}) bool { return true }
func (t fakeTool) IsReadOnly(map[string]interface{}) bool        { return true }

type countingTool struct {
	name  string
	calls *[]string
}

func (t countingTool) Name() string { return t.name }
func (t countingTool) Description() string {
	return t.name
}
func (t countingTool) InputSchema() types.ToolInputSchema {
	return types.ToolInputSchema{Type: "object"}
}
func (t countingTool) Call(_ context.Context, _ map[string]interface{}, _ *types.ToolUseContext) (*types.ToolResult, error) {
	*t.calls = append(*t.calls, t.name)
	return &types.ToolResult{
		Content: []types.ContentBlock{{Type: types.ContentBlockText, Text: t.name + " ok"}},
	}, nil
}
func (t countingTool) IsConcurrencySafe(map[string]interface{}) bool { return true }
func (t countingTool) IsReadOnly(map[string]interface{}) bool        { return true }

func TestBuildSystemPromptIncludesActiveSkill(t *testing.T) {
	prompt := buildSystemPromptText("base prompt", &skillRuntimeState{
		Name:   "review",
		Prompt: "Inspect the current diff carefully.",
	})

	if !strings.Contains(prompt, "base prompt") {
		t.Fatalf("missing base prompt: %q", prompt)
	}
	if !strings.Contains(prompt, "Active Skill: review") {
		t.Fatalf("missing skill header: %q", prompt)
	}
	if !strings.Contains(prompt, "Inspect the current diff carefully.") {
		t.Fatalf("missing skill prompt: %q", prompt)
	}
}

func TestFilterToolsBySkillAllowedTools(t *testing.T) {
	all := []types.Tool{
		fakeTool{name: "Read"},
		fakeTool{name: "Write"},
		fakeTool{name: "Skill"},
	}

	filtered := filterToolsForSkill(all, &skillRuntimeState{
		AllowedTools: []string{"Read"},
	})

	if len(filtered) != 1 {
		t.Fatalf("got %d tools, want 1", len(filtered))
	}
	if filtered[0].Name() != "Read" {
		t.Fatalf("got tool %q, want Read", filtered[0].Name())
	}
}

func TestApplyInlineSkillRuntime(t *testing.T) {
	a := &Agent{}

	state, result, err := a.applySkillRuntime(context.Background(), nil, tools.ToolCallRequest{
		ToolName: "Skill",
	}, &types.ToolResult{
		Data: skills.Result{
			Success:      true,
			CommandName:  "review",
			Status:       "inline",
			Prompt:       "Review the current diff.",
			AllowedTools: []string{"Read", "Glob"},
			Model:        "opus-4-6",
		},
		Content: []types.ContentBlock{{Type: types.ContentBlockText, Text: `{"success":true}`}},
	}, testToolUseContext(t))
	if err != nil {
		t.Fatal(err)
	}
	if state == nil {
		t.Fatal("expected inline skill to activate runtime state")
	}
	if state.Name != "review" {
		t.Fatalf("got skill name %q, want review", state.Name)
	}
	if state.Model != "opus-4-6" {
		t.Fatalf("got model %q, want opus-4-6", state.Model)
	}
	if len(state.AllowedTools) != 2 {
		t.Fatalf("got %d allowed tools, want 2", len(state.AllowedTools))
	}
	if len(result.Content) == 0 || !strings.Contains(result.Content[0].Text, `Activated skill "review"`) {
		t.Fatalf("unexpected inline skill tool result: %+v", result.Content)
	}
}

func TestApplyForkSkillRuntime(t *testing.T) {
	a := &Agent{
		subagentSpawner: func(ctx context.Context, config tools.SubagentConfig) (string, error) {
			if config.Name != "researcher" {
				t.Fatalf("got agent %q, want researcher", config.Name)
			}
			if config.Model != "sonnet-4-6" {
				t.Fatalf("got model %q, want sonnet-4-6", config.Model)
			}
			if len(config.Tools) != 1 || config.Tools[0] != "Read" {
				t.Fatalf("unexpected tools: %+v", config.Tools)
			}
			if config.Prompt != "Investigate the bug." {
				t.Fatalf("got prompt %q", config.Prompt)
			}
			return "subagent result", nil
		},
	}

	state, result, err := a.applySkillRuntime(context.Background(), nil, tools.ToolCallRequest{
		ToolName: "Skill",
	}, &types.ToolResult{
		Data: skills.Result{
			Success:      true,
			CommandName:  "debug",
			Status:       "forked",
			Prompt:       "Investigate the bug.",
			AllowedTools: []string{"Read"},
			Model:        "sonnet-4-6",
			Agent:        "researcher",
		},
	}, testToolUseContext(t))
	if err != nil {
		t.Fatal(err)
	}
	if state != nil {
		t.Fatal("expected forked skill not to persist as active state")
	}
	if len(result.Content) == 0 || result.Content[0].Text != "subagent result" {
		t.Fatalf("unexpected forked skill result: %+v", result.Content)
	}
}

func TestExecuteToolCallsAppliesSkillToLaterCallsInSameBatch(t *testing.T) {
	var calls []string

	registry := tools.NewRegistry()
	registry.Register(countingTool{name: "Read", calls: &calls})
	registry.Register(tools.NewSkillTool())

	a := &Agent{toolRegistry: registry}
	executor := tools.NewExecutor(registry, nil, testToolUseContext(t))

	results, activeSkill, err := a.executeToolCallsWithSkillRuntime(context.Background(), executor, []tools.ToolCallRequest{
		{ToolUseID: "1", ToolName: "Skill", Input: map[string]interface{}{"skill": "review"}},
		{ToolUseID: "2", ToolName: "Read"},
		{ToolUseID: "3", ToolName: "Write"},
	}, nil, testToolUseContext(t))
	if err != nil {
		t.Fatal(err)
	}

	if activeSkill == nil || activeSkill.Name != "review" {
		t.Fatalf("expected active review skill, got %+v", activeSkill)
	}
	if len(calls) != 1 || calls[0] != "Read" {
		t.Fatalf("expected only Read to execute, got %+v", calls)
	}
	if len(results) != 3 {
		t.Fatalf("got %d results, want 3", len(results))
	}
	if results[2].Result == nil || !results[2].Result.IsError {
		t.Fatalf("expected disallowed Write call to be rejected, got %+v", results[2])
	}
	if !strings.Contains(results[2].Result.Content[0].Text, `not allowed by active skill "review"`) {
		t.Fatalf("unexpected rejection text: %+v", results[2].Result.Content)
	}
}

func textToolResult(text string) *types.ToolResult {
	return &types.ToolResult{
		Content: []types.ContentBlock{{Type: types.ContentBlockText, Text: text}},
	}
}

func testToolUseContext(t *testing.T) *types.ToolUseContext {
	return &types.ToolUseContext{
		WorkingDir: t.TempDir(),
		AbortCtx:   context.Background(),
	}
}
