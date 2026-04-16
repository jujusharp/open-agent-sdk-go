package tools

import (
	"context"
	"sync"

	"github.com/jujusharp/open-agent-sdk-go/types"
)

// ToolCallRequest represents a pending tool call.
type ToolCallRequest struct {
	ToolUseID string
	ToolName  string
	Input     map[string]interface{}
}

// ToolCallResponse is the result of a tool call execution.
type ToolCallResponse struct {
	ToolUseID string
	Result    *types.ToolResult
	Error     error
}

type indexedToolCall struct {
	index int
	call  ToolCallRequest
}

// Executor runs tool calls with concurrency management.
type Executor struct {
	registry         *Registry
	canUseTool       types.CanUseToolFn
	permissionPrompt types.PermissionPromptFn
	toolCtx          *types.ToolUseContext
}

// NewExecutor creates a new tool executor.
func NewExecutor(registry *Registry, canUseTool types.CanUseToolFn, toolCtx *types.ToolUseContext) *Executor {
	return NewExecutorWithPermissionPrompt(registry, canUseTool, nil, toolCtx)
}

// NewExecutorWithPermissionPrompt creates a new tool executor with an interactive permission prompt.
func NewExecutorWithPermissionPrompt(registry *Registry, canUseTool types.CanUseToolFn, permissionPrompt types.PermissionPromptFn, toolCtx *types.ToolUseContext) *Executor {
	return &Executor{
		registry:         registry,
		canUseTool:       canUseTool,
		permissionPrompt: permissionPrompt,
		toolCtx:          toolCtx,
	}
}

// RunTools executes a batch of tool calls, partitioning into concurrent
// and sequential groups based on tool properties.
func (e *Executor) RunTools(ctx context.Context, calls []ToolCallRequest) []ToolCallResponse {
	if len(calls) == 0 {
		return nil
	}

	// Partition into concurrent-safe and sequential groups
	var concurrent []indexedToolCall
	var sequential []indexedToolCall

	for i, call := range calls {
		tool := e.registry.Get(call.ToolName)
		if tool == nil {
			sequential = append(sequential, indexedToolCall{index: i, call: call})
			continue
		}
		if tool.IsConcurrencySafe(call.Input) {
			concurrent = append(concurrent, indexedToolCall{index: i, call: call})
		} else {
			sequential = append(sequential, indexedToolCall{index: i, call: call})
		}
	}

	results := make([]ToolCallResponse, len(calls))

	// Run concurrent tools in parallel
	if len(concurrent) > 0 {
		for idx, result := range e.runConcurrent(ctx, concurrent) {
			results[concurrent[idx].index] = result
		}
	}

	// Run sequential tools one at a time
	for _, item := range sequential {
		result := e.runSingle(ctx, item.call)
		results[item.index] = result
	}

	return results
}

func (e *Executor) runConcurrent(ctx context.Context, calls []indexedToolCall) []ToolCallResponse {
	results := make([]ToolCallResponse, len(calls))
	var wg sync.WaitGroup

	for i, item := range calls {
		wg.Add(1)
		go func(idx int, c ToolCallRequest) {
			defer wg.Done()
			results[idx] = e.runSingle(ctx, c)
		}(i, item.call)
	}

	wg.Wait()
	return results
}

func (e *Executor) runSingle(ctx context.Context, call ToolCallRequest) ToolCallResponse {
	tool := e.registry.Get(call.ToolName)
	if tool == nil {
		return ToolCallResponse{
			ToolUseID: call.ToolUseID,
			Result: &types.ToolResult{
				IsError: true,
				Error:   "Unknown tool: " + call.ToolName,
				Content: []types.ContentBlock{{
					Type: types.ContentBlockText,
					Text: "Error: tool '" + call.ToolName + "' not found",
				}},
			},
		}
	}

	// Check permissions
	if e.canUseTool != nil {
		decision, err := e.canUseTool(tool, call.Input)
		if err != nil {
			return ToolCallResponse{
				ToolUseID: call.ToolUseID,
				Result: &types.ToolResult{
					IsError: true,
					Error:   "Permission check failed: " + err.Error(),
				},
			}
		}
		if decision == nil {
			decision = &types.PermissionDecision{Behavior: types.PermissionAllow}
		}
		if decision.Behavior == types.PermissionAsk {
			promptInput := call.Input
			if decision.UpdatedInput != nil {
				promptInput = decision.UpdatedInput
			}
			if e.permissionPrompt == nil {
				return permissionDeniedResponse(call.ToolUseID, decision.Reason)
			}
			decision, err = e.permissionPrompt(ctx, types.PermissionPromptRequest{
				ToolName: call.ToolName,
				Input:    promptInput,
				Reason:   decision.Reason,
			})
			if err != nil {
				return ToolCallResponse{
					ToolUseID: call.ToolUseID,
					Result: &types.ToolResult{
						IsError: true,
						Error:   "Permission prompt failed: " + err.Error(),
					},
				}
			}
			if decision == nil {
				decision = &types.PermissionDecision{Behavior: types.PermissionDeny}
			}
		}
		if decision.Behavior == types.PermissionDeny || decision.Behavior == types.PermissionAsk {
			return permissionDeniedResponse(call.ToolUseID, decision.Reason)
		}
		// Apply updated input if permission handler modified it
		if decision.UpdatedInput != nil {
			call.Input = decision.UpdatedInput
		}
	}

	// Execute tool
	result, err := tool.Call(ctx, call.Input, e.toolCtx)
	if err != nil {
		return ToolCallResponse{
			ToolUseID: call.ToolUseID,
			Result: &types.ToolResult{
				IsError: true,
				Error:   err.Error(),
				Content: []types.ContentBlock{{
					Type: types.ContentBlockText,
					Text: "Error: " + err.Error(),
				}},
			},
		}
	}

	return ToolCallResponse{
		ToolUseID: call.ToolUseID,
		Result:    result,
	}
}

func permissionDeniedResponse(toolUseID, reason string) ToolCallResponse {
	if reason == "" {
		reason = "Permission denied"
	}
	return ToolCallResponse{
		ToolUseID: toolUseID,
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
