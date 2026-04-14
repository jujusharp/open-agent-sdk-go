package skills

import (
	"strings"

	"github.com/codeany-ai/open-agent-sdk-go/types"
)

const reviewPrompt = `Review the current code changes for potential issues. Follow these steps:

1. Run ` + "`git diff`" + ` to see uncommitted changes, or ` + "`git diff main...HEAD`" + ` for branch changes
2. For each changed file, analyze:
   - **Correctness**: Logic errors, edge cases, off-by-one errors
   - **Security**: Injection vulnerabilities, auth issues, data exposure
   - **Performance**: N+1 queries, unnecessary allocations, blocking I/O
   - **Style**: Naming, consistency with surrounding code, readability
   - **Testing**: Are the changes adequately tested?
3. Provide a summary with:
   - Critical issues (must fix)
   - Suggestions (nice to have)
   - Questions (need clarification)

Be specific: reference file names, line numbers, and suggest fixes.`

const debugPrompt = `Debug the described issue using a systematic approach:

1. **Reproduce**: Understand and reproduce the issue
   - Read relevant error messages or logs
   - Identify the failing component

2. **Investigate**: Trace the root cause
   - Read the relevant source code
   - Add logging or use debugging tools if needed
   - Check recent changes that might have introduced the issue (` + "`git log --oneline -20`" + `)

3. **Hypothesize**: Form a theory about the cause
   - State your hypothesis clearly before attempting a fix

4. **Fix**: Implement the minimal fix
   - Make the smallest change that resolves the issue
   - Don't refactor unrelated code

5. **Verify**: Confirm the fix works
   - Run relevant tests
   - Check for regressions`

const testPrompt = `Run the project's test suite and analyze the results:

1. **Discover**: Find the test runner configuration
   - Look for package.json scripts, jest.config, vitest.config, pytest.ini, etc.
   - Identify the appropriate test command

2. **Execute**: Run the tests
   - Run the full test suite or specific tests if specified
   - Capture output including failures and errors

3. **Analyze**: If tests fail:
   - Read the failing test to understand what it expects
   - Read the source code being tested
   - Identify why the test is failing
   - Fix the issue (in tests or source as appropriate)

4. **Re-verify**: Run the failing tests again to confirm the fix`

const commitPrompt = `Create a git commit for the current changes. Follow these steps:

1. Run ` + "`git status`" + ` and ` + "`git diff --cached`" + ` to understand what's staged
2. If nothing is staged, run ` + "`git diff`" + ` to see unstaged changes and suggest what to stage
3. Analyze the changes and draft a concise commit message that:
   - Uses imperative mood ("Add feature" not "Added feature")
   - Summarizes the "why" not just the "what"
   - Keeps the first line under 72 characters
   - Adds a body with details if the change is complex
4. Create the commit

Do NOT push to remote unless explicitly asked.`

const simplifyPrompt = `Review the recently changed code for three categories of improvements. Launch 3 parallel Agent sub-tasks:

## Task 1: Reuse Analysis
Look for:
- Duplicated code that could be consolidated
- Existing utilities or helpers that could replace new code
- Patterns that should be extracted into shared functions
- Re-implementations of functionality that already exists elsewhere

## Task 2: Code Quality
Look for:
- Overly complex logic that could be simplified
- Poor naming or unclear intent
- Missing edge case handling
- Unnecessary abstractions or over-engineering
- Dead code or unused imports

## Task 3: Efficiency
Look for:
- Unnecessary allocations or copies
- N+1 query patterns or redundant I/O
- Blocking operations that could be async
- Inefficient data structures for the access pattern
- Unnecessary re-computation

After all three analyses complete, fix any issues found. Prioritize by impact.`

// InitBundledSkills registers the built-in skill set.
func InitBundledSkills() {
	RegisterSkill(Definition{
		Name:          "review",
		Description:   "Review code changes for correctness, security, performance, and style issues.",
		Aliases:       []string{"review-pr", "cr"},
		AllowedTools:  []string{"Bash", "Read", "Glob", "Grep"},
		UserInvocable: true,
		GetPrompt: func(args string, _ *types.ToolUseContext) ([]types.ContentBlock, error) {
			prompt := reviewPrompt
			if trimmed := strings.TrimSpace(args); trimmed != "" {
				prompt += "\n\nFocus area: " + trimmed
			}
			return textBlocks(prompt), nil
		},
	})

	RegisterSkill(Definition{
		Name:          "debug",
		Description:   "Systematic debugging of an issue using structured investigation.",
		Aliases:       []string{"investigate", "diagnose"},
		UserInvocable: true,
		GetPrompt: func(args string, _ *types.ToolUseContext) ([]types.ContentBlock, error) {
			prompt := debugPrompt
			if trimmed := strings.TrimSpace(args); trimmed != "" {
				prompt += "\n\n## Issue Description\n" + trimmed
			} else {
				prompt += "\n\nAsk the user to describe the issue they're experiencing."
			}
			return textBlocks(prompt), nil
		},
	})

	RegisterSkill(Definition{
		Name:          "test",
		Description:   "Run tests and analyze failures, fixing any issues found.",
		Aliases:       []string{"run-tests"},
		AllowedTools:  []string{"Bash", "Read", "Write", "Edit", "Glob", "Grep"},
		UserInvocable: true,
		GetPrompt: func(args string, _ *types.ToolUseContext) ([]types.ContentBlock, error) {
			prompt := testPrompt
			if trimmed := strings.TrimSpace(args); trimmed != "" {
				prompt += "\n\nSpecific test target: " + trimmed
			}
			return textBlocks(prompt), nil
		},
	})

	RegisterSkill(Definition{
		Name:          "commit",
		Description:   "Create a git commit with a well-crafted message based on staged changes.",
		Aliases:       []string{"ci"},
		AllowedTools:  []string{"Bash", "Read", "Glob", "Grep"},
		UserInvocable: true,
		GetPrompt: func(args string, _ *types.ToolUseContext) ([]types.ContentBlock, error) {
			prompt := commitPrompt
			if trimmed := strings.TrimSpace(args); trimmed != "" {
				prompt += "\n\nAdditional instructions: " + trimmed
			}
			return textBlocks(prompt), nil
		},
	})

	RegisterSkill(Definition{
		Name:          "simplify",
		Description:   "Review changed code for reuse, quality, and efficiency, then fix any issues found.",
		UserInvocable: true,
		GetPrompt: func(args string, _ *types.ToolUseContext) ([]types.ContentBlock, error) {
			prompt := simplifyPrompt
			if trimmed := strings.TrimSpace(args); trimmed != "" {
				prompt += "\n\n## Additional Focus\n" + trimmed
			}
			return textBlocks(prompt), nil
		},
	})
}

func textBlocks(text string) []types.ContentBlock {
	return []types.ContentBlock{{Type: types.ContentBlockText, Text: text}}
}
