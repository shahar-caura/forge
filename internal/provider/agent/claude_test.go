package agent

import (
	"bytes"
	"context"
	"fmt"
	"log/slog"
	"os"
	"os/exec"
	"path/filepath"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func testLogger() *slog.Logger {
	return slog.New(slog.NewTextHandler(os.Stderr, &slog.HandlerOptions{Level: slog.LevelDebug}))
}

// stubCommand returns a commandContext that creates a command running the test binary
// with the given exit code and stderr output.
func stubCommand(exitCode int, stderr string) func(ctx context.Context, name string, args ...string) *exec.Cmd {
	return func(ctx context.Context, name string, args ...string) *exec.Cmd {
		if exitCode == 0 {
			return exec.CommandContext(ctx, "true")
		}
		// Use sh -c to produce stderr and exit with code.
		cmd := exec.CommandContext(ctx, "sh", "-c", fmt.Sprintf("echo '%s' >&2; exit %d", stderr, exitCode))
		return cmd
	}
}

func TestRun_Success(t *testing.T) {
	c := New(5*time.Minute, testLogger())
	c.commandContext = stubCommand(0, "")

	output, err := c.Run(context.Background(), t.TempDir(), "do something")
	require.NoError(t, err)
	_ = output
}

func TestRun_Timeout(t *testing.T) {
	c := New(50*time.Millisecond, testLogger())
	c.commandContext = func(ctx context.Context, name string, args ...string) *exec.Cmd {
		return exec.CommandContext(ctx, "sleep", "60")
	}

	_, err := c.Run(context.Background(), t.TempDir(), "do something slow")
	require.Error(t, err)
	assert.Contains(t, err.Error(), "timed out")
}

func TestRun_NonZeroExit(t *testing.T) {
	c := New(5*time.Minute, testLogger())
	c.commandContext = stubCommand(1, "something went wrong")

	_, err := c.Run(context.Background(), t.TempDir(), "do something")
	require.Error(t, err)
	assert.Contains(t, err.Error(), "agent failed")
	assert.Contains(t, err.Error(), "something went wrong")
}

func TestRun_ContextCancelled(t *testing.T) {
	c := New(5*time.Minute, testLogger())
	c.commandContext = func(ctx context.Context, name string, args ...string) *exec.Cmd {
		return exec.CommandContext(ctx, "sleep", "60")
	}

	ctx, cancel := context.WithCancel(context.Background())
	cancel()

	_, err := c.Run(ctx, t.TempDir(), "do something")
	require.Error(t, err)
}

// stubCommandWithOutput returns a commandContext that echoes the given stdout text.
func stubCommandWithOutput(stdout string) func(ctx context.Context, name string, args ...string) *exec.Cmd {
	return func(ctx context.Context, name string, args ...string) *exec.Cmd {
		return exec.CommandContext(ctx, "sh", "-c", fmt.Sprintf("echo '%s'", stdout))
	}
}

func TestRun_StreamingToLogWriter(t *testing.T) {
	var logBuf bytes.Buffer
	c := New(5*time.Minute, testLogger())
	c.LogWriter = &logBuf
	c.commandContext = stubCommandWithOutput("hello from agent")

	output, err := c.Run(context.Background(), t.TempDir(), "do something")
	require.NoError(t, err)

	assert.Contains(t, output, "hello from agent")
	assert.Contains(t, logBuf.String(), "hello from agent")
}

func TestRun_NilLogWriter_Fallback(t *testing.T) {
	c := New(5*time.Minute, testLogger())
	c.LogWriter = nil // explicitly nil — should use CombinedOutput path
	c.commandContext = stubCommandWithOutput("buffered output")

	output, err := c.Run(context.Background(), t.TempDir(), "do something")
	require.NoError(t, err)
	assert.Contains(t, output, "buffered output")
}

func TestRun_Streaming_NonZeroExit(t *testing.T) {
	var logBuf bytes.Buffer
	c := New(5*time.Minute, testLogger())
	c.LogWriter = &logBuf
	c.commandContext = func(ctx context.Context, name string, args ...string) *exec.Cmd {
		return exec.CommandContext(ctx, "sh", "-c", "echo 'partial output'; exit 1")
	}

	output, err := c.Run(context.Background(), t.TempDir(), "do something")
	require.Error(t, err)
	assert.Contains(t, err.Error(), "agent failed")
	assert.Contains(t, output, "partial output")
	assert.Contains(t, logBuf.String(), "partial output")
}

func TestLoadProjectDocs_WithFiles(t *testing.T) {
	dir := t.TempDir()
	claudeDir := filepath.Join(dir, ".claude")
	require.NoError(t, os.MkdirAll(claudeDir, 0o755))

	require.NoError(t, os.WriteFile(filepath.Join(claudeDir, "linting.md"), []byte("Use ruff check --fix"), 0o644))
	require.NoError(t, os.WriteFile(filepath.Join(claudeDir, "conventions.md"), []byte("Follow DRY"), 0o644))
	// Non-md files should be ignored.
	require.NoError(t, os.WriteFile(filepath.Join(claudeDir, "settings.json"), []byte("{}"), 0o644))
	// Empty md files should be skipped.
	require.NoError(t, os.WriteFile(filepath.Join(claudeDir, "empty.md"), []byte(""), 0o644))

	result := loadProjectDocs(dir, testLogger())

	assert.Contains(t, result, "AUTHORITATIVE")
	assert.Contains(t, result, "Use ruff check --fix")
	assert.Contains(t, result, "Follow DRY")
	assert.Contains(t, result, ".claude/linting.md")
	assert.Contains(t, result, ".claude/conventions.md")
	assert.NotContains(t, result, "settings.json")
	assert.NotContains(t, result, "empty.md")
}

func TestLoadProjectDocs_NoClaude(t *testing.T) {
	dir := t.TempDir()
	result := loadProjectDocs(dir, testLogger())
	assert.Empty(t, result)
}

func TestRun_Streaming_Timeout(t *testing.T) {
	var logBuf bytes.Buffer
	c := New(100*time.Millisecond, testLogger())
	c.LogWriter = &logBuf
	c.commandContext = func(ctx context.Context, name string, args ...string) *exec.Cmd {
		// Print something then hang.
		return exec.CommandContext(ctx, "sh", "-c", "echo 'started'; sleep 60")
	}

	output, err := c.Run(context.Background(), t.TempDir(), "slow task")
	require.Error(t, err)
	assert.Contains(t, err.Error(), "timed out")
	// Partial output should be captured.
	assert.Contains(t, output, "started")
	assert.Contains(t, logBuf.String(), "started")
}
