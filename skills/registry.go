package skills

import (
	"strings"
	"sync"
)

var registry = struct {
	mu      sync.RWMutex
	skills  map[string]Definition
	aliases map[string]string
}{
	skills:  make(map[string]Definition),
	aliases: make(map[string]string),
}

// RegisterSkill adds or replaces a skill definition.
func RegisterSkill(def Definition) {
	registry.mu.Lock()
	defer registry.mu.Unlock()

	registry.skills[def.Name] = def
	for _, alias := range def.Aliases {
		registry.aliases[alias] = def.Name
	}
}

// GetSkill looks up a skill by name or alias.
func GetSkill(name string) *Definition {
	registry.mu.RLock()
	defer registry.mu.RUnlock()

	if def, ok := registry.skills[name]; ok {
		defCopy := def
		return &defCopy
	}
	if resolved, ok := registry.aliases[name]; ok {
		if def, ok := registry.skills[resolved]; ok {
			defCopy := def
			return &defCopy
		}
	}
	return nil
}

// GetAllSkills returns all registered skills.
func GetAllSkills() []Definition {
	registry.mu.RLock()
	defer registry.mu.RUnlock()

	result := make([]Definition, 0, len(registry.skills))
	for _, def := range registry.skills {
		result = append(result, def)
	}
	return result
}

// GetUserInvocableSkills returns skills visible to the model and user.
func GetUserInvocableSkills() []Definition {
	all := GetAllSkills()
	result := make([]Definition, 0, len(all))
	for _, def := range all {
		if !def.UserInvocable {
			continue
		}
		if def.IsEnabled != nil && !def.IsEnabled() {
			continue
		}
		result = append(result, def)
	}
	return result
}

// HasSkill reports whether the skill or alias exists.
func HasSkill(name string) bool {
	return GetSkill(name) != nil
}

// UnregisterSkill removes a skill and its aliases.
func UnregisterSkill(name string) bool {
	registry.mu.Lock()
	defer registry.mu.Unlock()

	def, ok := registry.skills[name]
	if !ok {
		return false
	}
	for _, alias := range def.Aliases {
		delete(registry.aliases, alias)
	}
	delete(registry.skills, name)
	return true
}

// ClearSkills removes all registered skills.
func ClearSkills() {
	registry.mu.Lock()
	defer registry.mu.Unlock()

	registry.skills = make(map[string]Definition)
	registry.aliases = make(map[string]string)
}

// FormatSkillsForPrompt renders a bounded text listing for prompt injection.
func FormatSkillsForPrompt(contextWindowTokens int) string {
	skills := GetUserInvocableSkills()
	if len(skills) == 0 {
		return ""
	}

	const (
		charsPerToken = 4
		defaultBudget = 8000
		maxDescChars  = 250
	)

	budget := defaultBudget
	if contextWindowTokens > 0 {
		budget = contextWindowTokens * charsPerToken / 100
	}

	lines := make([]string, 0, len(skills))
	used := 0
	for _, def := range skills {
		desc := strings.TrimSpace(def.Description)
		if len(desc) > maxDescChars {
			desc = desc[:maxDescChars] + "..."
		}
		trigger := ""
		if def.WhenToUse != "" {
			trigger = " TRIGGER when: " + strings.TrimSpace(def.WhenToUse)
		}

		line := "- " + def.Name + ": " + desc + trigger
		if used+len(line) > budget {
			break
		}
		lines = append(lines, line)
		used += len(line)
	}

	return strings.Join(lines, "\n")
}
