package tools

import (
	"context"
	"encoding/json"
	"sort"
	"strings"

	"github.com/codeany-ai/open-agent-sdk-go/skills"
	"github.com/codeany-ai/open-agent-sdk-go/types"
)

// SkillTool invokes prompt-based skills registered in the global skills registry.
type SkillTool struct{}

// NewSkillTool creates a new SkillTool.
func NewSkillTool() *SkillTool { return &SkillTool{} }

func (t *SkillTool) Name() string { return "Skill" }

func (t *SkillTool) Description() string {
	return "Execute a skill within the current conversation. Use the skill name and optional arguments when a specialized workflow matches the user's request."
}

func (t *SkillTool) InputSchema() types.ToolInputSchema {
	return types.ToolInputSchema{
		Type: "object",
		Properties: map[string]interface{}{
			"skill": map[string]interface{}{
				"type":        "string",
				"description": `The skill name to execute (for example "commit", "review", or "simplify")`,
			},
			"args": map[string]interface{}{
				"type":        "string",
				"description": "Optional arguments for the skill.",
			},
		},
		Required: []string{"skill"},
	}
}

func (t *SkillTool) IsConcurrencySafe(_ map[string]interface{}) bool { return false }
func (t *SkillTool) IsReadOnly(_ map[string]interface{}) bool        { return false }

func (t *SkillTool) Call(ctx context.Context, input map[string]interface{}, tCtx *types.ToolUseContext) (*types.ToolResult, error) {
	skillName, _ := input["skill"].(string)
	args, _ := input["args"].(string)
	if skillName == "" {
		return errorResult("skill name is required"), nil
	}

	skill := skills.GetSkill(skillName)
	if skill == nil {
		names := limitedSkillNames(12)
		msg := `Unknown skill "` + skillName + `". Available skills: `
		if len(names) == 0 {
			msg += "none"
		} else {
			msg += strings.Join(names, ", ")
		}
		return errorResult(msg), nil
	}

	if skill.IsEnabled != nil && !skill.IsEnabled() {
		return errorResult(`Skill "` + skillName + `" is currently disabled`), nil
	}

	content, err := skill.GetPrompt(args, tCtx)
	if err != nil {
		return errorResult(`Error executing skill "` + skillName + `": ` + err.Error()), nil
	}

	var promptParts []string
	for _, block := range content {
		if block.Type == types.ContentBlockText {
			promptParts = append(promptParts, block.Text)
		}
	}

	status := string(skill.Context)
	if status == "" {
		status = string(skills.ContextInline)
	} else if skill.Context == skills.ContextFork {
		status = "forked"
	}

	payload := skills.Result{
		Success:      true,
		CommandName:  skill.Name,
		Status:       status,
		Prompt:       strings.Join(promptParts, "\n\n"),
		AllowedTools: skill.AllowedTools,
		Model:        skill.Model,
		Agent:        skill.Agent,
	}

	data, err := json.Marshal(payload)
	if err != nil {
		return nil, err
	}

	return &types.ToolResult{
		Data: payload,
		Content: []types.ContentBlock{{
			Type: types.ContentBlockText,
			Text: string(data),
		}},
	}, nil
}

func limitedSkillNames(limit int) []string {
	available := skills.GetUserInvocableSkills()
	names := make([]string, 0, len(available))
	for _, def := range available {
		names = append(names, def.Name)
	}
	sort.Strings(names)
	if limit > 0 && len(names) > limit {
		return names[:limit]
	}
	return names
}
