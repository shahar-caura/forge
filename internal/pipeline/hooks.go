package pipeline

import (
	"context"
	"fmt"
	"log/slog"
	"os/exec"
	"strings"
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
