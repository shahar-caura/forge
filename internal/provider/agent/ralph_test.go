package agent

import (
	"bytes"
	"context"
	"os"
	"os/exec"
	"path/filepath"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// ralphStubs wraps a commandContext function to handle ralph-enable calls.
// For "ralph-enable", it creates scaffold files in dir and returns success.
// For all other commands, it delegates to fn.
func ralphStubs(dir string, fn func(ctx context.Context, name string, args ...string) *exec.Cmd) func(ctx context.Context, name string, args ...string) *exec.Cmd {
	return func(ctx context.Context, name string, args ...string) *exec.Cmd {
		if name == "ralph-enable" {
			ralphDir := filepath.Join(dir, ".ralph")
			_ = os.MkdirAll(ralphDir, 0o755)
			_ = os.WriteFile(filepath.Join(ralphDir, "PROMPT.md"), []byte("generic"), 0o644)
			_ = os.WriteFile(filepath.Join(ralphDir, "fix_plan.md"), []byte(""), 0o644)
			_ = os.WriteFile(filepath.Join(ralphDir, "AGENT.md"), []byte(""), 0o644)
			_ = os.WriteFile(filepath.Join(dir, ".ralphrc"), []byte(""), 0o644)
			return exec.CommandContext(ctx, "true")
		}
		return fn(ctx, name, args...)
	}
}

func TestRalph_Run_Success(t *testing.T) {
	dir := t.TempDir()
	r := NewRalph(5*time.Minute, "Read,Write,Edit", testLogger())
	r.commandContext = ralphStubs(dir, stubCommand(0, ""))

	output, err := r.Run(context.Background(), dir, "do something")
	require.NoError(t, err)
	_ = output
}

func TestRalph_Run_Timeout(t *testing.T) {
	dir := t.TempDir()
	r := NewRalph(50*time.Millisecond, "Read,Write", testLogger())
	r.commandContext = ralphStubs(dir, func(ctx context.Context, name string, args ...string) *exec.Cmd {
		return exec.CommandContext(ctx, "sleep", "60")
	})

	_, err := r.Run(context.Background(), dir, "do something slow")
	require.Error(t, err)
	assert.Contains(t, err.Error(), "timed out")
}

func TestRalph_Run_NonZeroExit(t *testing.T) {
	dir := t.TempDir()
	r := NewRalph(5*time.Minute, "Read,Write", testLogger())
	r.commandContext = ralphStubs(dir, stubCommand(1, "something went wrong"))

	_, err := r.Run(context.Background(), dir, "do something")
	require.Error(t, err)
	assert.Contains(t, err.Error(), "agent failed")
	assert.Contains(t, err.Error(), "something went wrong")
}

func TestRalph_Run_ContextCancelled(t *testing.T) {
	dir := t.TempDir()
	r := NewRalph(5*time.Minute, "Read,Write", testLogger())
	r.commandContext = ralphStubs(dir, func(ctx context.Context, name string, args ...string) *exec.Cmd {
		return exec.CommandContext(ctx, "sleep", "60")
	})

	ctx, cancel := context.WithCancel(context.Background())
	cancel()

	_, err := r.Run(ctx, dir, "do something")
	require.Error(t, err)
}

func TestRalph_Run_StreamingToLogWriter(t *testing.T) {
	dir := t.TempDir()
	var logBuf bytes.Buffer
	r := NewRalph(5*time.Minute, "Read,Write", testLogger())
	r.LogWriter = &logBuf
	r.commandContext = ralphStubs(dir, stubCommandWithOutput("hello from ralph"))

	output, err := r.Run(context.Background(), dir, "do something")
	require.NoError(t, err)

	assert.Contains(t, output, "hello from ralph")
	assert.Contains(t, logBuf.String(), "hello from ralph")
}

func TestRalph_Run_NilLogWriter_Fallback(t *testing.T) {
	dir := t.TempDir()
	r := NewRalph(5*time.Minute, "Read,Write", testLogger())
	r.LogWriter = nil
	r.commandContext = ralphStubs(dir, stubCommandWithOutput("buffered output"))

	output, err := r.Run(context.Background(), dir, "do something")
	require.NoError(t, err)
	assert.Contains(t, output, "buffered output")
}

func TestRalph_Run_Streaming_NonZeroExit(t *testing.T) {
	dir := t.TempDir()
	var logBuf bytes.Buffer
	r := NewRalph(5*time.Minute, "Read,Write", testLogger())
	r.LogWriter = &logBuf
	r.commandContext = ralphStubs(dir, func(ctx context.Context, name string, args ...string) *exec.Cmd {
		return exec.CommandContext(ctx, "sh", "-c", "echo 'partial output'; exit 1")
	})

	output, err := r.Run(context.Background(), dir, "do something")
	require.Error(t, err)
	assert.Contains(t, err.Error(), "agent failed")
	assert.Contains(t, output, "partial output")
	assert.Contains(t, logBuf.String(), "partial output")
}

func TestRalph_Run_Streaming_Timeout(t *testing.T) {
	dir := t.TempDir()
	var logBuf bytes.Buffer
	r := NewRalph(100*time.Millisecond, "Read,Write", testLogger())
	r.LogWriter = &logBuf
	r.commandContext = ralphStubs(dir, func(ctx context.Context, name string, args ...string) *exec.Cmd {
		return exec.CommandContext(ctx, "sh", "-c", "echo 'started'; sleep 60")
	})

	output, err := r.Run(context.Background(), dir, "slow task")
	require.Error(t, err)
	assert.Contains(t, err.Error(), "timed out")
	assert.Contains(t, output, "started")
	assert.Contains(t, logBuf.String(), "started")
}

func TestRalph_Run_ScaffoldAndPromptOverwrite(t *testing.T) {
	dir := t.TempDir()
	var enableArgs []string

	r := NewRalph(5*time.Minute, "Read,Write", testLogger())
	r.commandContext = func(ctx context.Context, name string, args ...string) *exec.Cmd {
		if name == "ralph-enable" {
			enableArgs = args
			// Simulate ralph-enable: create scaffold with a generic prompt.
			ralphDir := filepath.Join(dir, ".ralph")
			_ = os.MkdirAll(ralphDir, 0o755)
			_ = os.WriteFile(filepath.Join(ralphDir, "PROMPT.md"), []byte("generic prompt from ralph-enable"), 0o644)
			_ = os.WriteFile(filepath.Join(ralphDir, "fix_plan.md"), []byte(""), 0o644)
			_ = os.WriteFile(filepath.Join(ralphDir, "AGENT.md"), []byte(""), 0o644)
			_ = os.WriteFile(filepath.Join(dir, ".ralphrc"), []byte(""), 0o644)
			return exec.CommandContext(ctx, "true")
		}

		// For ralph: verify PROMPT.md was overwritten with forge's prompt.
		content, err := os.ReadFile(filepath.Join(dir, ".ralph", "PROMPT.md"))
		if err != nil || string(content) != "forge prompt" {
			return exec.CommandContext(ctx, "sh", "-c", "echo 'prompt not overwritten'; exit 1")
		}
		return exec.CommandContext(ctx, "true")
	}

	_, err := r.Run(context.Background(), dir, "forge prompt")
	require.NoError(t, err)
	assert.Equal(t, []string{"--non-interactive", "--force", "--skip-tasks"}, enableArgs)
}

func TestRalph_Run_RalphEnableFails(t *testing.T) {
	r := NewRalph(5*time.Minute, "Read,Write", testLogger())
	r.commandContext = func(ctx context.Context, name string, args ...string) *exec.Cmd {
		return exec.CommandContext(ctx, "sh", "-c", "echo 'ralph-enable: not found' >&2; exit 1")
	}

	_, err := r.Run(context.Background(), t.TempDir(), "test prompt")
	require.Error(t, err)
	assert.Contains(t, err.Error(), "ralph-enable failed")
}

func TestRalph_Run_AllowedToolsEnvVar(t *testing.T) {
	dir := t.TempDir()
	r := NewRalph(5*time.Minute, "Read,Write,Bash", testLogger())
	r.commandContext = ralphStubs(dir, func(ctx context.Context, name string, args ...string) *exec.Cmd {
		// Print the env var so we can verify it was set.
		return exec.CommandContext(ctx, "sh", "-c", "echo $CLAUDE_ALLOWED_TOOLS")
	})

	output, err := r.Run(context.Background(), dir, "test prompt")
	require.NoError(t, err)
	assert.Contains(t, output, "Read,Write,Bash")
}

func TestRalph_Run_CleanupOnSuccess(t *testing.T) {
	dir := t.TempDir()
	r := NewRalph(5*time.Minute, "Read,Write", testLogger())
	r.commandContext = ralphStubs(dir, stubCommand(0, ""))

	_, err := r.Run(context.Background(), dir, "test prompt")
	require.NoError(t, err)

	// .ralph/ and .ralphrc should be cleaned up after execution.
	_, statErr := os.Stat(filepath.Join(dir, ".ralph"))
	assert.True(t, os.IsNotExist(statErr), ".ralph dir should be removed after run")
	_, statErr = os.Stat(filepath.Join(dir, ".ralphrc"))
	assert.True(t, os.IsNotExist(statErr), ".ralphrc should be removed after run")
}

func TestRalph_Run_CleanupOnError(t *testing.T) {
	dir := t.TempDir()
	r := NewRalph(5*time.Minute, "Read,Write", testLogger())
	r.commandContext = ralphStubs(dir, stubCommand(1, "failed"))

	_, err := r.Run(context.Background(), dir, "test prompt")
	require.Error(t, err)

	// .ralph/ and .ralphrc should still be cleaned up on error.
	_, statErr := os.Stat(filepath.Join(dir, ".ralph"))
	assert.True(t, os.IsNotExist(statErr), ".ralph dir should be removed even on error")
	_, statErr = os.Stat(filepath.Join(dir, ".ralphrc"))
	assert.True(t, os.IsNotExist(statErr), ".ralphrc should be removed even on error")
}

func TestRalph_SetLogWriter(t *testing.T) {
	r := NewRalph(5*time.Minute, "Read,Write", testLogger())
	assert.Nil(t, r.LogWriter)

	var buf bytes.Buffer
	r.SetLogWriter(&buf)
	assert.Equal(t, &buf, r.LogWriter)

	r.ClearLogWriter()
	assert.Nil(t, r.LogWriter)
}
