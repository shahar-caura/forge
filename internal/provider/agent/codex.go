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

// Codex implements provider.Agent using the OpenAI codex CLI.
type Codex struct {
	Timeout time.Duration
	Logger  *slog.Logger

	// LogWriter, when non-nil, receives a real-time copy of agent stdout+stderr.
	LogWriter io.Writer

	// commandContext is overridable for testing.
	commandContext func(ctx context.Context, name string, args ...string) *exec.Cmd
}

// NewCodex creates a new Codex agent provider.
func NewCodex(timeout time.Duration, logger *slog.Logger) *Codex {
	return &Codex{
		Timeout:        timeout,
		Logger:         logger,
		commandContext: exec.CommandContext,
	}
}

// SetLogWriter sets the writer that receives a real-time copy of agent output.
func (c *Codex) SetLogWriter(w io.Writer) { c.LogWriter = w }

// ClearLogWriter removes the streaming log writer.
func (c *Codex) ClearLogWriter() { c.LogWriter = nil }

func (c *Codex) PromptSuffix() string { return "" }

func (c *Codex) Run(ctx context.Context, dir, prompt string) (string, error) {
	ctx, cancel := context.WithTimeout(ctx, c.Timeout)
	defer cancel()

	c.Logger.Info("running codex agent", "dir", dir, "timeout", c.Timeout)

	args := []string{"exec",
		"--full-auto",
		"--json",
		"--cd", dir,
		prompt,
	}

	cmd := c.commandContext(ctx, "codex", args...)
	cmd.Dir = dir

	// When no LogWriter is set, use simple CombinedOutput.
	if c.LogWriter == nil {
		out, err := cmd.CombinedOutput()
		if err != nil {
			if ctx.Err() == context.DeadlineExceeded {
				return string(out), fmt.Errorf("agent timed out after %s", c.Timeout)
			}
			return string(out), fmt.Errorf("agent failed: %w: %s", err, out)
		}
		c.Logger.Info("codex agent completed")
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
	go func() { defer wg.Done(); _, _ = io.Copy(w, stdout) }()
	go func() { defer wg.Done(); _, _ = io.Copy(w, stderr) }()
	wg.Wait()

	err = cmd.Wait()
	out := buf.String()
	if err != nil {
		if ctx.Err() == context.DeadlineExceeded {
			return out, fmt.Errorf("agent timed out after %s", c.Timeout)
		}
		return out, fmt.Errorf("agent failed: %w: %s", err, out)
	}

	c.Logger.Info("codex agent completed")
	return out, nil
}
