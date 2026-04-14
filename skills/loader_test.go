package skills

import (
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/codeany-ai/open-agent-sdk-go/types"
)

func TestLoadFromDirsRegistersMarkdownSkill(t *testing.T) {
	ClearSkills()

	root := t.TempDir()
	skillDir := filepath.Join(root, "skills", "release-notes")
	if err := os.MkdirAll(skillDir, 0o755); err != nil {
		t.Fatal(err)
	}

	content := `---
name: release-notes
description: Draft release notes from recent code changes.
aliases:
  - notes
allowed-tools:
  - Read
  - Glob
argument-hint: [focus]
context: inline
---

# Release Notes

Summarize the current code changes for end users.`
	if err := os.WriteFile(filepath.Join(skillDir, "SKILL.md"), []byte(content), 0o644); err != nil {
		t.Fatal(err)
	}

	loaded, err := LoadFromDirs([]string{filepath.Join(root, "skills")})
	if err != nil {
		t.Fatal(err)
	}
	if len(loaded) != 1 {
		t.Fatalf("got %d loaded skills, want 1", len(loaded))
	}

	def := GetSkill("notes")
	if def == nil {
		t.Fatal("expected alias lookup to work for loaded skill")
	}
	if def.Name != "release-notes" {
		t.Fatalf("got skill name %q", def.Name)
	}
	if len(def.AllowedTools) != 2 {
		t.Fatalf("got allowed tools %+v", def.AllowedTools)
	}
	if def.SourcePath == "" {
		t.Fatal("expected loaded skill to retain source path")
	}

	blocks, err := def.GetPrompt("focus on user impact", &types.ToolUseContext{})
	if err != nil {
		t.Fatal(err)
	}
	if len(blocks) != 1 {
		t.Fatalf("got %d prompt blocks, want 1", len(blocks))
	}
	if !strings.Contains(blocks[0].Text, "# Release Notes") {
		t.Fatalf("missing markdown body in prompt: %q", blocks[0].Text)
	}
	if !strings.Contains(blocks[0].Text, "focus on user impact") {
		t.Fatalf("missing args in prompt: %q", blocks[0].Text)
	}
}

func TestLoadedSkillReadsBodyOnDemand(t *testing.T) {
	ClearSkills()

	root := t.TempDir()
	skillDir := filepath.Join(root, "skills", "live")
	if err := os.MkdirAll(skillDir, 0o755); err != nil {
		t.Fatal(err)
	}
	path := filepath.Join(skillDir, "SKILL.md")
	if err := os.WriteFile(path, []byte(`---
name: live
description: Reads the current markdown body.
---

first body`), 0o644); err != nil {
		t.Fatal(err)
	}

	if _, err := LoadFromDirs([]string{filepath.Join(root, "skills")}); err != nil {
		t.Fatal(err)
	}

	def := GetSkill("live")
	if def == nil {
		t.Fatal("expected live skill to be loaded")
	}

	if err := os.WriteFile(path, []byte(`---
name: live
description: Reads the current markdown body.
---

second body`), 0o644); err != nil {
		t.Fatal(err)
	}

	blocks, err := def.GetPrompt("", &types.ToolUseContext{})
	if err != nil {
		t.Fatal(err)
	}
	if len(blocks) != 1 || !strings.Contains(blocks[0].Text, "second body") {
		t.Fatalf("expected on-demand body read, got %+v", blocks)
	}
}

func TestDefaultSkillDirsIncludesStandardLocations(t *testing.T) {
	dirs := buildDefaultSkillDirs("/repo/app", "/Users/tester", "/Users/tester/.codex")

	expected := []string{
		"/repo/app/.agents/skills",
		"/repo/app/.claude/skills",
		"/repo/app/.codex/skills",
		"/Users/tester/.agents/skills",
		"/Users/tester/.claude/skills",
		"/Users/tester/.codex/skills",
		"/Users/tester/.codex/skills",
	}

	for _, want := range expected {
		if !containsString(dirs, want) {
			t.Fatalf("expected %q in default dirs: %+v", want, dirs)
		}
	}
}
