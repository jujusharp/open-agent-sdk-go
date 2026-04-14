package skills

import (
	"github.com/codeany-ai/open-agent-sdk-go/hooks"
	"github.com/codeany-ai/open-agent-sdk-go/types"
)

// ContextType controls where the skill should execute.
type ContextType string

const (
	ContextInline ContextType = "inline"
	ContextFork   ContextType = "fork"
)

// Definition describes a reusable skill prompt.
type Definition struct {
	Name          string
	Description   string
	Aliases       []string
	WhenToUse     string
	ArgumentHint  string
	AllowedTools  []string
	Model         string
	UserInvocable *bool
	IsEnabled     func() bool
	Hooks         hooks.HookConfig
	Context       ContextType
	Agent         string
	GetPrompt     func(args string, ctx *types.ToolUseContext) ([]types.ContentBlock, error)
}

// Result is the structured payload returned by the Skill tool.
type Result struct {
	Success      bool     `json:"success"`
	CommandName  string   `json:"commandName"`
	Status       string   `json:"status"`
	Prompt       string   `json:"prompt,omitempty"`
	AllowedTools []string `json:"allowedTools,omitempty"`
	Model        string   `json:"model,omitempty"`
	Agent        string   `json:"agent,omitempty"`
	Result       string   `json:"result,omitempty"`
	Error        string   `json:"error,omitempty"`
}

// Bool returns a pointer to the provided boolean.
func Bool(v bool) *bool {
	return &v
}
