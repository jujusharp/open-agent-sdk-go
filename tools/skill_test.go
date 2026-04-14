package tools

import (
	"context"
	"encoding/json"
	"strings"
	"testing"

	"github.com/codeany-ai/open-agent-sdk-go/skills"
	"github.com/codeany-ai/open-agent-sdk-go/types"
)

func TestSkillToolCall(t *testing.T) {
	skills.ClearSkills()
	skills.RegisterSkill(skills.Definition{
		Name:          "review",
		Description:   "Review current changes.",
		AllowedTools:  []string{"Read", "Glob"},
		Model:         "sonnet-4-6",
		UserInvocable: skills.Bool(true),
		GetPrompt: func(args string, _ *types.ToolUseContext) ([]types.ContentBlock, error) {
			return []types.ContentBlock{
				{Type: types.ContentBlockText, Text: "Review the current diff."},
				{Type: types.ContentBlockText, Text: "Focus area: " + args},
			}, nil
		},
	})

	tool := NewSkillTool()
	result, err := tool.Call(context.Background(), map[string]interface{}{
		"skill": "review",
		"args":  "error handling",
	}, testToolCtx(t))
	if err != nil {
		t.Fatal(err)
	}
	if result.IsError {
		t.Fatalf("unexpected tool error: %s", result.Error)
	}
	if len(result.Content) != 1 {
		t.Fatalf("got %d content blocks, want 1", len(result.Content))
	}

	var payload struct {
		Success      bool     `json:"success"`
		CommandName  string   `json:"commandName"`
		Status       string   `json:"status"`
		Prompt       string   `json:"prompt"`
		AllowedTools []string `json:"allowedTools"`
		Model        string   `json:"model"`
	}
	if err := json.Unmarshal([]byte(result.Content[0].Text), &payload); err != nil {
		t.Fatalf("failed to decode tool payload: %v", err)
	}

	if !payload.Success {
		t.Fatal("expected success=true")
	}
	if payload.CommandName != "review" {
		t.Fatalf("got commandName %q, want review", payload.CommandName)
	}
	if payload.Status != "inline" {
		t.Fatalf("got status %q, want inline", payload.Status)
	}
	if !strings.Contains(payload.Prompt, "Review the current diff.") {
		t.Fatalf("prompt %q missing expected text", payload.Prompt)
	}
	if !strings.Contains(payload.Prompt, "Focus area: error handling") {
		t.Fatalf("prompt %q missing args text", payload.Prompt)
	}
	if len(payload.AllowedTools) != 2 {
		t.Fatalf("got %d allowed tools, want 2", len(payload.AllowedTools))
	}
	if payload.Model != "sonnet-4-6" {
		t.Fatalf("got model %q, want sonnet-4-6", payload.Model)
	}
}

func TestSkillToolUnknownSkill(t *testing.T) {
	skills.ClearSkills()

	tool := NewSkillTool()
	result, err := tool.Call(context.Background(), map[string]interface{}{
		"skill": "missing",
	}, testToolCtx(t))
	if err != nil {
		t.Fatal(err)
	}
	if !result.IsError {
		t.Fatal("expected unknown skill to return an error result")
	}
	if len(result.Content) == 0 || !strings.Contains(result.Content[0].Text, `Unknown skill "missing"`) {
		t.Fatalf("unexpected error output: %+v", result.Content)
	}
}

func TestSkillToolDescriptionListsAvailableSkills(t *testing.T) {
	skills.ClearSkills()
	skills.RegisterSkill(skills.Definition{
		Name:          "debug",
		Description:   "Investigate a failure with a structured debugging flow.",
		UserInvocable: skills.Bool(true),
		GetPrompt: func(args string, _ *types.ToolUseContext) ([]types.ContentBlock, error) {
			return []types.ContentBlock{{Type: types.ContentBlockText, Text: args}}, nil
		},
	})

	desc := NewSkillTool().Description()
	if strings.Contains(desc, "debug") {
		t.Fatalf("description should stay compact and avoid enumerating skills, got %q", desc)
	}
	if len(desc) > 220 {
		t.Fatalf("description too large: %d chars", len(desc))
	}
}
