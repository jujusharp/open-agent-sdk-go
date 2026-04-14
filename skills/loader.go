package skills

import (
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"strings"

	"gopkg.in/yaml.v3"

	"github.com/codeany-ai/open-agent-sdk-go/types"
)

// LoadFromDirs loads skills from <dir>/<skill>/SKILL.md directories.
func LoadFromDirs(dirs []string) ([]string, error) {
	var loaded []string
	var errs []error

	for _, rawDir := range dirs {
		dir := expandSkillPath(rawDir)
		if dir == "" {
			continue
		}

		entries, err := os.ReadDir(dir)
		if err != nil {
			if errors.Is(err, os.ErrNotExist) {
				continue
			}
			errs = append(errs, err)
			continue
		}

		for _, entry := range entries {
			if !entry.IsDir() {
				continue
			}
			skillPath := filepath.Join(dir, entry.Name(), "SKILL.md")
			def, err := loadSkillFile(skillPath, entry.Name())
			if err != nil {
				if errors.Is(err, os.ErrNotExist) {
					continue
				}
				errs = append(errs, err)
				continue
			}
			RegisterSkill(def)
			loaded = append(loaded, def.Name)
		}
	}

	return loaded, errors.Join(errs...)
}

// DefaultSkillDirs returns the standard local and user skill directories.
func DefaultSkillDirs(cwd string) []string {
	home, _ := os.UserHomeDir()
	return buildDefaultSkillDirs(cwd, home, os.Getenv("CODEX_HOME"))
}

func buildDefaultSkillDirs(cwd, home, codexHome string) []string {
	var dirs []string

	if cwd != "" {
		dirs = append(dirs,
			filepath.Join(cwd, ".agents", "skills"),
			filepath.Join(cwd, ".claude", "skills"),
			filepath.Join(cwd, ".codex", "skills"),
		)
	}
	if home != "" {
		dirs = append(dirs,
			filepath.Join(home, ".agents", "skills"),
			filepath.Join(home, ".claude", "skills"),
			filepath.Join(home, ".codex", "skills"),
		)
	}
	if codexHome != "" {
		dirs = append(dirs, filepath.Join(codexHome, "skills"))
	}

	return uniqueStrings(dirs)
}

func loadSkillFile(path, fallbackName string) (Definition, error) {
	data, err := os.ReadFile(path)
	if err != nil {
		return Definition{}, err
	}

	meta, body, err := splitFrontmatter(string(data))
	if err != nil {
		return Definition{}, err
	}

	var fm map[string]interface{}
	if strings.TrimSpace(meta) != "" {
		if err := yaml.Unmarshal([]byte(meta), &fm); err != nil {
			return Definition{}, err
		}
	}

	name := strings.TrimSpace(stringValue(fm, "name"))
	if name == "" {
		name = fallbackName
	}
	description := strings.TrimSpace(stringValue(fm, "description"))
	if description == "" {
		description = firstNonEmptyLine(body)
	}

	contextType := ContextInline
	switch strings.TrimSpace(stringValue(fm, "context")) {
	case string(ContextFork):
		contextType = ContextFork
	case "", string(ContextInline):
		contextType = ContextInline
	default:
		return Definition{}, fmt.Errorf("invalid skill context in %s", path)
	}

	promptBody := strings.TrimSpace(body)
	def := Definition{
		Name:          name,
		Description:   description,
		Aliases:       stringSliceValue(fm, "aliases"),
		WhenToUse:     strings.TrimSpace(stringValue(fm, "when_to_use", "when-to-use", "whenToUse")),
		ArgumentHint:  strings.TrimSpace(stringValue(fm, "argument_hint", "argument-hint", "argumentHint")),
		AllowedTools:  stringSliceValue(fm, "allowed_tools", "allowed-tools", "allowedTools"),
		Model:         strings.TrimSpace(stringValue(fm, "model")),
		UserInvocable: boolPointerValue(fm, "user_invocable", "user-invocable", "userInvocable"),
		Context:       contextType,
		Agent:         strings.TrimSpace(stringValue(fm, "agent")),
		SourcePath:    path,
	}
	_ = promptBody
	def.GetPrompt = filePromptLoader(path)
	return def, nil
}

func filePromptLoader(path string) func(args string, ctx *types.ToolUseContext) ([]types.ContentBlock, error) {
	return func(args string, _ *types.ToolUseContext) ([]types.ContentBlock, error) {
		data, err := os.ReadFile(path)
		if err != nil {
			return nil, err
		}

		_, body, err := splitFrontmatter(string(data))
		if err != nil {
			return nil, err
		}

		prompt := strings.TrimSpace(body)
		if trimmed := strings.TrimSpace(args); trimmed != "" {
			if prompt != "" {
				prompt += "\n\n"
			}
			prompt += "## Arguments\n" + trimmed
		}
		return []types.ContentBlock{{Type: types.ContentBlockText, Text: prompt}}, nil
	}
}

func splitFrontmatter(content string) (string, string, error) {
	if !strings.HasPrefix(content, "---\n") {
		return "", content, nil
	}

	rest := strings.TrimPrefix(content, "---\n")
	idx := strings.Index(rest, "\n---\n")
	if idx < 0 {
		return "", "", errors.New("invalid skill frontmatter")
	}
	return rest[:idx], rest[idx+5:], nil
}

func expandSkillPath(path string) string {
	path = strings.TrimSpace(path)
	if path == "" {
		return ""
	}
	if strings.HasPrefix(path, "~/") {
		if home, err := os.UserHomeDir(); err == nil {
			return filepath.Join(home, path[2:])
		}
	}
	return path
}

func uniqueStrings(values []string) []string {
	seen := make(map[string]bool, len(values))
	result := make([]string, 0, len(values))
	for _, value := range values {
		if value == "" || seen[value] {
			continue
		}
		seen[value] = true
		result = append(result, value)
	}
	return result
}

func containsString(values []string, target string) bool {
	for _, value := range values {
		if value == target {
			return true
		}
	}
	return false
}

func firstNonEmptyLine(text string) string {
	for _, line := range strings.Split(text, "\n") {
		trimmed := strings.TrimSpace(line)
		if trimmed != "" {
			return trimmed
		}
	}
	return ""
}

func stringValue(values map[string]interface{}, keys ...string) string {
	for _, key := range keys {
		if value, ok := values[key]; ok {
			if str, ok := value.(string); ok {
				return str
			}
		}
	}
	return ""
}

func stringSliceValue(values map[string]interface{}, keys ...string) []string {
	for _, key := range keys {
		value, ok := values[key]
		if !ok {
			continue
		}
		switch typed := value.(type) {
		case []interface{}:
			result := make([]string, 0, len(typed))
			for _, item := range typed {
				if str, ok := item.(string); ok {
					result = append(result, str)
				}
			}
			return result
		case []string:
			return typed
		case string:
			parts := strings.Split(typed, ",")
			result := make([]string, 0, len(parts))
			for _, part := range parts {
				if trimmed := strings.TrimSpace(part); trimmed != "" {
					result = append(result, trimmed)
				}
			}
			return result
		}
	}
	return nil
}

func boolPointerValue(values map[string]interface{}, keys ...string) *bool {
	for _, key := range keys {
		value, ok := values[key]
		if !ok {
			continue
		}
		if flag, ok := value.(bool); ok {
			return Bool(flag)
		}
	}
	return nil
}
