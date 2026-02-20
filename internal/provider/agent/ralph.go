package agent

import (
	"bytes"
	"context"
	"fmt"
	"io"
	"log/slog"
	"os"
	"os/exec"
	"path/filepath"
	"sync"
	"time"
)

// Ralph implements provider.Agent using the ralph CLI.
type Ralph struct {
	Timeout      time.Duration
	Logger       *slog.Logger
	AllowedTools string

	// LogWriter, when non-nil, receives a real-time copy of agent stdout+stderr.
	LogWriter io.Writer

	// commandContext is overridable for testing.
	commandContext func(ctx context.Context, name string, args ...string) *exec.Cmd
}

// NewRalph creates a new Ralph agent provider.
func NewRalph(timeout time.Duration, allowedTools string, logger *slog.Logger) *Ralph {
	return &Ralph{
		Timeout:        timeout,
		Logger:         logger,
		AllowedTools:   allowedTools,
		commandContext: exec.CommandContext,
	}
}

// SetLogWriter sets the writer that receives a real-time copy of agent output.
func (r *Ralph) SetLogWriter(w io.Writer) { r.LogWriter = w }

// ClearLogWriter removes the streaming log writer.
func (r *Ralph) ClearLogWriter() { r.LogWriter = nil }

func (r *Ralph) PromptSuffix() string {
	return `
5. When you have completed all changes and verified build+tests pass, include this block at the END of your final response:

---RALPH_STATUS---
STATUS: COMPLETE
EXIT_SIGNAL: true
---RALPH_STATUS---
`
}

func (r *Ralph) Run(ctx context.Context, dir, prompt string) (string, error) {
	ctx, cancel := context.WithTimeout(ctx, r.Timeout)
	defer cancel()

	r.Logger.Info("running ralph agent", "dir", dir, "timeout", r.Timeout)

	// Clean up ralph scaffolding regardless of outcome.
	defer func() { _ = os.RemoveAll(filepath.Join(dir, ".ralph")) }()
	defer func() { _ = os.Remove(filepath.Join(dir, ".ralphrc")) }()

	// Scaffold ralph project via ralph-enable (creates .ralph/, .ralphrc, etc.
	// with proper project detection for build/test commands).
	setupCmd := r.commandContext(ctx, "ralph-enable",
		"--non-interactive", "--force", "--skip-tasks")
	setupCmd.Dir = dir
	if out, err := setupCmd.CombinedOutput(); err != nil {
		return "", fmt.Errorf("ralph-enable failed: %w: %s", err, out)
	}

	// Overwrite the generic PROMPT.md with forge's prompt.
	if err := os.WriteFile(filepath.Join(dir, ".ralph", "PROMPT.md"), []byte(prompt), 0o644); err != nil {
		return "", fmt.Errorf("writing prompt file: %w", err)
	}

	timeoutMinutes := fmt.Sprintf("%d", int(r.Timeout.Minutes()))

	args := []string{
		"--prompt", ".ralph/PROMPT.md",
		"--timeout", timeoutMinutes,
		"--no-continue",
		"--output-format", "json",
	}

	cmd := r.commandContext(ctx, "ralph", args...)
	cmd.Dir = dir
	// Pass allowed tools via env var so ralph forwards them as a single
	// comma-separated string to claude --allowedTools (matching CLI format).
	if r.AllowedTools != "" {
		cmd.Env = append(os.Environ(), "CLAUDE_ALLOWED_TOOLS="+r.AllowedTools)
	}

	// When no LogWriter is set, use simple CombinedOutput (original behavior).
	if r.LogWriter == nil {
		out, err := cmd.CombinedOutput()
		if err != nil {
			if ctx.Err() == context.DeadlineExceeded {
				return string(out), fmt.Errorf("agent timed out after %s", r.Timeout)
			}
			return string(out), fmt.Errorf("agent failed: %w: %s", err, out)
		}
		r.Logger.Info("ralph agent completed")
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
	w := io.MultiWriter(&buf, r.LogWriter)

	var wg sync.WaitGroup
	wg.Add(2)
	go func() { defer wg.Done(); _, _ = io.Copy(w, stdout) }()
	go func() { defer wg.Done(); _, _ = io.Copy(w, stderr) }()
	wg.Wait()

	err = cmd.Wait()
	out := buf.String()
	if err != nil {
		if ctx.Err() == context.DeadlineExceeded {
			return out, fmt.Errorf("agent timed out after %s", r.Timeout)
		}
		return out, fmt.Errorf("agent failed: %w: %s", err, out)
	}

	r.Logger.Info("ralph agent completed")
	return out, nil
}
