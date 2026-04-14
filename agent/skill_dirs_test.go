package agent

import (
	"os"
	"path/filepath"
	"testing"

	"github.com/codeany-ai/open-agent-sdk-go/skills"
)

func TestNewLoadsFileSkillsFromConfiguredDirs(t *testing.T) {
	skills.ClearSkills()

	root := t.TempDir()
	skillDir := filepath.Join(root, "skills", "triage")
	if err := os.MkdirAll(skillDir, 0o755); err != nil {
		t.Fatal(err)
	}
	content := `---
name: triage
description: Triage the current issue.
---

Triage the current issue before making changes.`
	if err := os.WriteFile(filepath.Join(skillDir, "SKILL.md"), []byte(content), 0o644); err != nil {
		t.Fatal(err)
	}

	a := New(Options{
		CWD:       root,
		MaxTurns:  1,
		SkillDirs: []string{filepath.Join(root, "skills")},
	})
	defer a.Close()

	if !skills.HasSkill("triage") {
		t.Fatal("expected configured skill dir to be loaded")
	}
}
