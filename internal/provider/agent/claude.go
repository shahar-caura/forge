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
	"strings"
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

// SetLogWriter sets the writer that receives a real-time copy of agent output.
func (c *Claude) SetLogWriter(w io.Writer) { c.LogWriter = w }

// ClearLogWriter removes the streaming log writer.
func (c *Claude) ClearLogWriter() { c.LogWriter = nil }

func (c *Claude) PromptSuffix() string { return "" }

func (c *Claude) Run(ctx context.Context, dir, prompt string) (string, error) {
	ctx, cancel := context.WithTimeout(ctx, c.Timeout)
	defer cancel()

	c.Logger.Info("running agent", "dir", dir, "timeout", c.Timeout)

	args := []string{
		"-p", prompt,
		"--allowedTools", "Edit,Read,Write,Bash",
		"--output-format", "json",
	}

	// Inject .claude/*.md project docs into the system prompt so the headless
	// agent has the same instruction set as an interactive session.
	if extra := loadProjectDocs(dir, c.Logger); extra != "" {
		args = append(args, "--append-system-prompt", extra)
	}

	cmd := c.commandContext(ctx, "claude", args...)
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

	c.Logger.Info("agent completed")
	return out, nil
}

// loadProjectDocs reads all .claude/*.md files from the working directory
// and returns their content as a single string for --append-system-prompt.
// These files contain project-specific instructions (linting rules, conventions)
// that claude -p doesn't auto-load into its system prompt — only CLAUDE.md is
// auto-loaded. Without this, headless agents miss project docs that interactive
// sessions discover organically.
func loadProjectDocs(dir string, logger *slog.Logger) string {
	pattern := filepath.Join(dir, ".claude", "*.md")
	matches, err := filepath.Glob(pattern)
	if err != nil || len(matches) == 0 {
		return ""
	}

	var parts []string
	for _, path := range matches {
		data, err := os.ReadFile(path)
		if err != nil {
			logger.Warn("skipping unreadable .claude doc", "path", path, "error", err)
			continue
		}
		content := strings.TrimSpace(string(data))
		if content == "" {
			continue
		}
		name := filepath.Base(path)
		parts = append(parts, fmt.Sprintf("# .claude/%s\n\n%s", name, content))
		logger.Info("loaded project doc", "file", name)
	}

	if len(parts) == 0 {
		return ""
	}

	return "The following project-specific instructions were loaded from .claude/ and are AUTHORITATIVE — follow them exactly.\n\n" +
		strings.Join(parts, "\n\n---\n\n")
}
