package agent

import (
	"context"
	"fmt"
	"strings"

	"github.com/codeany-ai/open-agent-sdk-go/skills"
	"github.com/codeany-ai/open-agent-sdk-go/tools"
	"github.com/codeany-ai/open-agent-sdk-go/types"
)

type skillRuntimeState struct {
	Name         string
	Prompt       string
	AllowedTools []string
	Model        string
}

func buildSystemPromptText(base string, active *skillRuntimeState) string {
	if active == nil || strings.TrimSpace(active.Prompt) == "" {
		return base
	}

	var sb strings.Builder
	sb.WriteString(base)
	sb.WriteString("\n\n# Active Skill: ")
	sb.WriteString(active.Name)
	sb.WriteString("\n")
	sb.WriteString(active.Prompt)
	return sb.String()
}

func filterToolsForSkill(all []types.Tool, active *skillRuntimeState) []types.Tool {
	if active == nil || len(active.AllowedTools) == 0 {
		return all
	}

	allowed := make(map[string]bool, len(active.AllowedTools))
	for _, name := range active.AllowedTools {
		allowed[name] = true
	}

	filtered := make([]types.Tool, 0, len(all))
	for _, tool := range all {
		if allowed[tool.Name()] {
			filtered = append(filtered, tool)
		}
	}
	return filtered
}

func (a *Agent) applySkillRuntime(
	ctx context.Context,
	current *skillRuntimeState,
	call tools.ToolCallRequest,
	result *types.ToolResult,
	tCtx *types.ToolUseContext,
) (*skillRuntimeState, *types.ToolResult, error) {
	if call.ToolName != "Skill" || result == nil || result.IsError {
		return current, result, nil
	}

	payload, ok := result.Data.(skills.Result)
	if !ok || !payload.Success {
		return current, result, nil
	}

	switch payload.Status {
	case "forked":
		if a.subagentSpawner == nil {
			return current, &types.ToolResult{
				IsError: true,
				Error:   `Skill "` + payload.CommandName + `" cannot fork because subagent spawning is not configured`,
				Content: []types.ContentBlock{{
					Type: types.ContentBlockText,
					Text: `Error: Skill "` + payload.CommandName + `" cannot fork because subagent spawning is not configured`,
				}},
			}, nil
		}

		subagentResult, err := a.subagentSpawner(ctx, tools.SubagentConfig{
			Name:     payload.Agent,
			Prompt:   payload.Prompt,
			Tools:    payload.AllowedTools,
			Model:    payload.Model,
			CWD:      tCtx.WorkingDir,
			MaxTurns: 30,
		})
		if err != nil {
			return current, &types.ToolResult{
				IsError: true,
				Error:   fmt.Sprintf("Skill %q fork failed: %v", payload.CommandName, err),
				Content: []types.ContentBlock{{
					Type: types.ContentBlockText,
					Text: fmt.Sprintf("Error: skill %q fork failed: %v", payload.CommandName, err),
				}},
			}, nil
		}

		return current, &types.ToolResult{
			Data: result.Data,
			Content: []types.ContentBlock{{
				Type: types.ContentBlockText,
				Text: subagentResult,
			}},
		}, nil

	case "inline", "":
		state := &skillRuntimeState{
			Name:         payload.CommandName,
			Prompt:       payload.Prompt,
			AllowedTools: payload.AllowedTools,
			Model:        payload.Model,
		}
		return state, &types.ToolResult{
			Data: result.Data,
			Content: []types.ContentBlock{{
				Type: types.ContentBlockText,
				Text: fmt.Sprintf(`Activated skill "%s". Continue using the active skill instructions now applied to the system prompt.`, payload.CommandName),
			}},
		}, nil
	}

	return current, result, nil
}

func toolAllowedByActiveSkill(name string, active *skillRuntimeState) bool {
	if active == nil || len(active.AllowedTools) == 0 {
		return true
	}
	for _, allowed := range active.AllowedTools {
		if allowed == name {
			return true
		}
	}
	return false
}

func skillDeniedToolResult(call tools.ToolCallRequest, active *skillRuntimeState) tools.ToolCallResponse {
	reason := "Tool not allowed by active skill"
	if active != nil && active.Name != "" {
		reason = fmt.Sprintf(`Tool "%s" is not allowed by active skill "%s"`, call.ToolName, active.Name)
	}
	return tools.ToolCallResponse{
		ToolUseID: call.ToolUseID,
		Result: &types.ToolResult{
			IsError: true,
			Error:   reason,
			Content: []types.ContentBlock{{
				Type: types.ContentBlockText,
				Text: "Error: " + reason,
			}},
		},
	}
}

func (a *Agent) executeToolCallsWithSkillRuntime(
	ctx context.Context,
	executor *tools.Executor,
	calls []tools.ToolCallRequest,
	active *skillRuntimeState,
	tCtx *types.ToolUseContext,
) ([]tools.ToolCallResponse, *skillRuntimeState, error) {
	results := make([]tools.ToolCallResponse, 0, len(calls))

	for i := 0; i < len(calls); {
		call := calls[i]

		if call.ToolName == "Skill" {
			skillResult := executor.RunTools(ctx, []tools.ToolCallRequest{call})[0]
			nextSkill, updatedResult, err := a.applySkillRuntime(ctx, active, call, skillResult.Result, tCtx)
			if err != nil {
				return nil, active, err
			}
			skillResult.Result = updatedResult
			results = append(results, skillResult)
			active = nextSkill
			i++
			continue
		}

		if !toolAllowedByActiveSkill(call.ToolName, active) {
			results = append(results, skillDeniedToolResult(call, active))
			i++
			continue
		}

		tool := a.toolRegistry.Get(call.ToolName)
		if tool == nil || !tool.IsConcurrencySafe(call.Input) {
			single := executor.RunTools(ctx, []tools.ToolCallRequest{call})[0]
			results = append(results, single)
			i++
			continue
		}

		batch := make([]tools.ToolCallRequest, 0, len(calls)-i)
		for i < len(calls) {
			next := calls[i]
			if next.ToolName == "Skill" || !toolAllowedByActiveSkill(next.ToolName, active) {
				break
			}
			nextTool := a.toolRegistry.Get(next.ToolName)
			if nextTool == nil || !nextTool.IsConcurrencySafe(next.Input) {
				break
			}
			batch = append(batch, next)
			i++
		}

		results = append(results, executor.RunTools(ctx, batch)...)
	}

	return results, active, nil
}
