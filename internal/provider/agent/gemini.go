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

// Gemini implements provider.Agent using the Google gemini CLI.
type Gemini struct {
	Timeout time.Duration
	Logger  *slog.Logger

	// LogWriter, when non-nil, receives a real-time copy of agent stdout+stderr.
	LogWriter io.Writer

	// commandContext is overridable for testing.
	commandContext func(ctx context.Context, name string, args ...string) *exec.Cmd
}

// NewGemini creates a new Gemini agent provider.
func NewGemini(timeout time.Duration, logger *slog.Logger) *Gemini {
	return &Gemini{
		Timeout:        timeout,
		Logger:         logger,
		commandContext: exec.CommandContext,
	}
}

// SetLogWriter sets the writer that receives a real-time copy of agent output.
func (g *Gemini) SetLogWriter(w io.Writer) { g.LogWriter = w }

// ClearLogWriter removes the streaming log writer.
func (g *Gemini) ClearLogWriter() { g.LogWriter = nil }

func (g *Gemini) PromptSuffix() string { return "" }

func (g *Gemini) Run(ctx context.Context, dir, prompt string) (string, error) {
	ctx, cancel := context.WithTimeout(ctx, g.Timeout)
	defer cancel()

	g.Logger.Info("running gemini agent", "dir", dir, "timeout", g.Timeout)

	args := []string{
		"-p", prompt,
		"--yolo",
		"--output-format", "json",
	}

	cmd := g.commandContext(ctx, "gemini", args...)
	cmd.Dir = dir

	// When no LogWriter is set, use simple CombinedOutput.
	if g.LogWriter == nil {
		out, err := cmd.CombinedOutput()
		if err != nil {
			if ctx.Err() == context.DeadlineExceeded {
				return string(out), fmt.Errorf("agent timed out after %s", g.Timeout)
			}
			return string(out), fmt.Errorf("agent failed: %w: %s", err, out)
		}
		g.Logger.Info("gemini agent completed")
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
	w := io.MultiWriter(&buf, g.LogWriter)

	var wg sync.WaitGroup
	wg.Add(2)
	go func() { defer wg.Done(); _, _ = io.Copy(w, stdout) }()
	go func() { defer wg.Done(); _, _ = io.Copy(w, stderr) }()
	wg.Wait()

	err = cmd.Wait()
	out := buf.String()
	if err != nil {
		if ctx.Err() == context.DeadlineExceeded {
			return out, fmt.Errorf("agent timed out after %s", g.Timeout)
		}
		return out, fmt.Errorf("agent failed: %w: %s", err, out)
	}

	g.Logger.Info("gemini agent completed")
	return out, nil
}
