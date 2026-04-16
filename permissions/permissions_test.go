package permissions

import (
	"context"
	"testing"

	"github.com/jujusharp/open-agent-sdk-go/types"
)

type permissionTestTool struct {
	name     string
	readOnly bool
}

func (t permissionTestTool) Name() string { return t.name }
func (t permissionTestTool) Description() string {
	return t.name
}
func (t permissionTestTool) InputSchema() types.ToolInputSchema {
	return types.ToolInputSchema{Type: "object"}
}
func (t permissionTestTool) Call(context.Context, map[string]interface{}, *types.ToolUseContext) (*types.ToolResult, error) {
	return &types.ToolResult{}, nil
}
func (t permissionTestTool) IsConcurrencySafe(map[string]interface{}) bool { return true }
func (t permissionTestTool) IsReadOnly(map[string]interface{}) bool        { return t.readOnly }

func TestDefaultModeAsksForMutatingTools(t *testing.T) {
	canUseTool := NewCanUseToolFn(&Config{Mode: types.PermissionModeDefault}, nil)

	decision, err := canUseTool(permissionTestTool{name: "Bash"}, map[string]interface{}{"command": "touch file"})
	if err != nil {
		t.Fatal(err)
	}
	if decision.Behavior != types.PermissionAsk {
		t.Fatalf("expected ask, got %s", decision.Behavior)
	}
}

func TestDefaultModeAllowsReadOnlyTools(t *testing.T) {
	canUseTool := NewCanUseToolFn(&Config{Mode: types.PermissionModeDefault}, nil)

	decision, err := canUseTool(permissionTestTool{name: "Read", readOnly: true}, map[string]interface{}{"file_path": "go.mod"})
	if err != nil {
		t.Fatal(err)
	}
	if decision.Behavior != types.PermissionAllow {
		t.Fatalf("expected allow, got %s", decision.Behavior)
	}
}

func TestAcceptEditsModeAsksForNonEditMutations(t *testing.T) {
	canUseTool := NewCanUseToolFn(&Config{Mode: types.PermissionModeAcceptEdits}, nil)

	decision, err := canUseTool(permissionTestTool{name: "Bash"}, map[string]interface{}{"command": "touch file"})
	if err != nil {
		t.Fatal(err)
	}
	if decision.Behavior != types.PermissionAsk {
		t.Fatalf("expected ask, got %s", decision.Behavior)
	}
}
