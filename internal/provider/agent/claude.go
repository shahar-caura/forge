package agent

import (
	"context"
	"fmt"
	"log/slog"
	"os/exec"
	"time"
)

// Claude implements provider.Agent using the claude CLI.
type Claude struct {
	Timeout time.Duration
	Logger  *slog.Logger

	// commandContext is overridable for testing.
	commandContext func(ctx context.Context, name string, args ...string) *exec.Cmd
}

// New creates a new Claude agent provider.
func New(timeout time.Duration, logger *slog.Logger) *Claude {
	return &Claude{
		Timeout:        timeout,
		Logger:         logger,
		commandContext: exec.CommandContext,
	}
}

func (c *Claude) Run(ctx context.Context, dir, prompt string) error {
	ctx, cancel := context.WithTimeout(ctx, c.Timeout)
	defer cancel()

	c.Logger.Info("running agent", "dir", dir, "timeout", c.Timeout)

	cmd := c.commandContext(ctx, "claude", "-p", prompt, "--allowedTools", "Read,Write,Bash")
	cmd.Dir = dir

	out, err := cmd.CombinedOutput()
	if err != nil {
		if ctx.Err() == context.DeadlineExceeded {
			return fmt.Errorf("agent timed out after %s", c.Timeout)
		}
		return fmt.Errorf("agent failed: %w: %s", err, out)
	}

	c.Logger.Info("agent completed")
	return nil
}
