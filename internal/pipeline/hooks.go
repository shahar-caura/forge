package pipeline

import (
	"context"
	"fmt"
	"log/slog"
	"os/exec"
	"strings"

	"github.com/shahar-caura/forge/internal/provider"
)

// runHook executes a shell command in the given directory.
// Used for lifecycle hooks like pre-commit formatting.
func runHook(ctx context.Context, command, dir string, logger *slog.Logger) error {
	logger.Info("running pre-commit hook", "cmd", command)
	cmd := exec.CommandContext(ctx, "sh", "-c", command)
	cmd.Dir = dir
	out, err := cmd.CombinedOutput()
	if err != nil {
		return fmt.Errorf("%w: %s", err, strings.TrimSpace(string(out)))
	}
	return nil
}

// runHookWithRetry runs the pre-commit hook, and on failure feeds the error
// output to the agent to fix. Retries up to maxRetries times.
// If agent is nil or maxRetries is 0, fails fast on first hook failure.
func runHookWithRetry(ctx context.Context, command, dir string, agent provider.Agent, maxRetries int, logger *slog.Logger) error {
	err := runHook(ctx, command, dir, logger)
	if err == nil {
		return nil
	}

	if agent == nil || maxRetries <= 0 {
		return err
	}

	for attempt := 1; attempt <= maxRetries; attempt++ {
		logger.Warn("pre-commit hook failed, asking agent to fix", "attempt", attempt, "max", maxRetries, "error", err)

		prompt := buildHookFixPrompt(command, err.Error())
		if _, agentErr := agent.Run(ctx, dir, prompt); agentErr != nil {
			return fmt.Errorf("agent fix attempt %d: %w", attempt, agentErr)
		}

		err = runHook(ctx, command, dir, logger)
		if err == nil {
			logger.Info("pre-commit hook passed after agent fix", "attempt", attempt)
			return nil
		}
	}

	return fmt.Errorf("pre-commit hook failed after %d retries: %w", maxRetries, err)
}

// buildHookFixPrompt constructs a prompt telling the agent to fix hook failures.
func buildHookFixPrompt(command, hookOutput string) string {
	// Truncate to last 4000 chars — the tail contains the actual errors,
	// the head is usually passing tests and noise.
	const maxOutput = 4000
	truncated := hookOutput
	if len(truncated) > maxOutput {
		truncated = "...[truncated]\n" + truncated[len(truncated)-maxOutput:]
	}

	return `The pre-commit hook failed. Fix ALL reported errors so the hook passes.

Hook command: ` + command + `

Error output (tail):
` + truncated + `

Instructions:
1. Focus on FAIL lines and error messages above — those are the failures to fix.
2. For "DATA RACE" errors: add sync.Mutex to the struct and Lock/Unlock in every method that mutates state.
3. For formatting errors: run the formatter (gofmt/goimports).
4. After fixing, verify by running ONLY the failing package, e.g.: go test -race ./path/to/pkg/
5. Make no unrelated changes — only fix what the hook reported.
`
}
