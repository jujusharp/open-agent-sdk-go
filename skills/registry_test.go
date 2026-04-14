package skills

import (
	"testing"

	"github.com/codeany-ai/open-agent-sdk-go/types"
)

func TestRegistryLifecycle(t *testing.T) {
	ClearSkills()

	RegisterSkill(Definition{
		Name:          "lint",
		Description:   "Run lints against the current project.",
		Aliases:       []string{"check"},
		UserInvocable: true,
		GetPrompt: func(args string, _ *types.ToolUseContext) ([]types.ContentBlock, error) {
			return []types.ContentBlock{
				{Type: types.ContentBlockText, Text: "lint prompt"},
			}, nil
		},
	})

	if !HasSkill("lint") {
		t.Fatal("expected lint to be registered")
	}
	if !HasSkill("check") {
		t.Fatal("expected alias to be registered")
	}

	got := GetSkill("check")
	if got == nil {
		t.Fatal("expected alias lookup to resolve")
	}
	if got.Name != "lint" {
		t.Fatalf("got skill %q, want lint", got.Name)
	}

	all := GetAllSkills()
	if len(all) != 1 {
		t.Fatalf("got %d skills, want 1", len(all))
	}

	invocable := GetUserInvocableSkills()
	if len(invocable) != 1 {
		t.Fatalf("got %d invocable skills, want 1", len(invocable))
	}

	formatted := FormatSkillsForPrompt(100000)
	if formatted == "" {
		t.Fatal("expected formatted skills output")
	}

	if !UnregisterSkill("lint") {
		t.Fatal("expected skill to be removed")
	}
	if HasSkill("lint") || HasSkill("check") {
		t.Fatal("expected skill and alias to be removed")
	}
}

func TestInitBundledSkillsRegistersExpectedSkills(t *testing.T) {
	ClearSkills()

	InitBundledSkills()
	InitBundledSkills()

	expected := []string{"review", "debug", "test", "commit", "simplify"}
	for _, name := range expected {
		if !HasSkill(name) {
			t.Fatalf("expected bundled skill %q to be registered", name)
		}
	}

	all := GetAllSkills()
	if len(all) != len(expected) {
		t.Fatalf("got %d bundled skills, want %d", len(all), len(expected))
	}
}
