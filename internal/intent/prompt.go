package intent

import (
	"fmt"
	"strings"
)

// BuildPrompt constructs the full classification prompt for the LLM.
func BuildPrompt(query string, dc DynamicContext) string {
	const maxQueryLen = 500
	if len(query) > maxQueryLen {
		query = query[:maxQueryLen]
	}

	var sb strings.Builder

	sb.WriteString(`You are a CLI intent classifier for the "forge" tool.
Given a natural language query, determine which forge subcommand the user intends to run.

## Forge subcommands

- forge run [plan.md]         — Execute a plan file (--issue N, --all-issues, --label L, --dry-run)
- forge push <run-id>         — Force push changes for a run
- forge resume <run-id>       — Resume a failed run
- forge runs                  — List all runs (--limit N)
- forge status <run-id>       — Show run status
- forge logs <run-id>         — Stream agent logs (--follow, --step S)
- forge steps                 — Show step details
- forge edit <run-id>         — Edit run state
- forge cleanup               — Clean old runs (--dry-run, --before DATE)
- forge init                  — Initialize forge.yaml
- forge completion [shell]    — Generate shell completion
- forge serve                 — Start local web server (--addr, --open)
- forge version               — Show forge version

`)

	if ctx := FormatForPrompt(dc); ctx != "" {
		sb.WriteString(ctx)
	}

	sb.WriteString(`## Rules

1. Map the query to exactly one subcommand with appropriate flags/arguments.
2. Resolve partial or fuzzy names against the plan files and run IDs listed above.
3. NEVER invent file paths or run IDs that are not in the lists above.
4. If the query is ambiguous or cannot map to a subcommand, return empty argv.
5. Return ONLY bare JSON — no markdown, no code fences, no explanation outside the JSON.

## Output format

{"argv": ["subcommand", "arg1", ...], "confidence": 0.0-1.0, "reasoning": "brief explanation"}

`)

	fmt.Fprintf(&sb, "## User query\n\n%s\n", query)

	return sb.String()
}
