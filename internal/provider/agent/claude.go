package agent

import (
	"bytes"
	"context"
	"fmt"
	"io"
	"log/slog"
	"os/exec"
	"sync"
	"time"
)

// Claude implements provider.Agent using the claude CLI.
type Claude struct {
	Timeout time.Duration
	Logger  *slog.Logger

	// LogWriter, when non-nil, receives a real-time copy of agent stdout+stderr.
	LogWriter io.Writer

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

func (c *Claude) Run(ctx context.Context, dir, prompt string) (string, error) {
	ctx, cancel := context.WithTimeout(ctx, c.Timeout)
	defer cancel()

	c.Logger.Info("running agent", "dir", dir, "timeout", c.Timeout)

	cmd := c.commandContext(ctx, "claude", "-p", prompt,
		"--allowedTools", "Read,Write,Bash",
		"--output-format", "json",
	)
	cmd.Dir = dir

	// When no LogWriter is set, use simple CombinedOutput (original behavior).
	if c.LogWriter == nil {
		out, err := cmd.CombinedOutput()
		if err != nil {
			if ctx.Err() == context.DeadlineExceeded {
				return string(out), fmt.Errorf("agent timed out after %s", c.Timeout)
			}
			return string(out), fmt.Errorf("agent failed: %w: %s", err, out)
		}
		c.Logger.Info("agent completed")
		return string(out), nil
	}

	// Streaming: tee stdout+stderr to both a buffer and LogWriter.
	stdout, err := cmd.StdoutPipe()
	if err != nil {
		return "", fmt.Errorf("agent stdout pipe: %w", err)
	}
	stderr, err := cmd.StderrPipe()
	if err != nil {
		return "", fmt.Errorf("agent stderr pipe: %w", err)
	}

	if err := cmd.Start(); err != nil {
		return "", fmt.Errorf("agent start: %w", err)
	}

	var buf bytes.Buffer
	w := io.MultiWriter(&buf, c.LogWriter)

	var wg sync.WaitGroup
	wg.Add(2)
	go func() { defer wg.Done(); io.Copy(w, stdout) }()
	go func() { defer wg.Done(); io.Copy(w, stderr) }()
	wg.Wait()

	err = cmd.Wait()
	out := buf.String()
	if err != nil {
		if ctx.Err() == context.DeadlineExceeded {
			return out, fmt.Errorf("agent timed out after %s", c.Timeout)
		}
		return out, fmt.Errorf("agent failed: %w: %s", err, out)
	}

	c.Logger.Info("agent completed")
	return out, nil
}
