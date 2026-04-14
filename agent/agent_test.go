package agent

import (
	"testing"

	"github.com/codeany-ai/open-agent-sdk-go/skills"
)

func TestNewInitializesBundledSkills(t *testing.T) {
	skills.ClearSkills()

	a := New(Options{
		CWD:      t.TempDir(),
		MaxTurns: 1,
	})
	defer a.Close()

	if !skills.HasSkill("review") {
		t.Fatal("expected bundled skills to be initialized")
	}
}
