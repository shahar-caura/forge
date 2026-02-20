package agent

import (
	"bytes"
	"context"
	"os/exec"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestCodex_Run_Success(t *testing.T) {
	c := NewCodex(5*time.Minute, testLogger())
	c.commandContext = stubCommand(0, "")

	output, err := c.Run(context.Background(), t.TempDir(), "do something")
	require.NoError(t, err)
	_ = output
}

func TestCodex_Run_Timeout(t *testing.T) {
	c := NewCodex(50*time.Millisecond, testLogger())
	c.commandContext = func(ctx context.Context, name string, args ...string) *exec.Cmd {
		return exec.CommandContext(ctx, "sleep", "60")
	}

	_, err := c.Run(context.Background(), t.TempDir(), "do something slow")
	require.Error(t, err)
	assert.Contains(t, err.Error(), "timed out")
}

func TestCodex_Run_NonZeroExit(t *testing.T) {
	c := NewCodex(5*time.Minute, testLogger())
	c.commandContext = stubCommand(1, "something went wrong")

	_, err := c.Run(context.Background(), t.TempDir(), "do something")
	require.Error(t, err)
	assert.Contains(t, err.Error(), "agent failed")
	assert.Contains(t, err.Error(), "something went wrong")
}

func TestCodex_Run_ContextCancelled(t *testing.T) {
	c := NewCodex(5*time.Minute, testLogger())
	c.commandContext = func(ctx context.Context, name string, args ...string) *exec.Cmd {
		return exec.CommandContext(ctx, "sleep", "60")
	}

	ctx, cancel := context.WithCancel(context.Background())
	cancel()

	_, err := c.Run(ctx, t.TempDir(), "do something")
	require.Error(t, err)
}

func TestCodex_Run_StreamingToLogWriter(t *testing.T) {
	var logBuf bytes.Buffer
	c := NewCodex(5*time.Minute, testLogger())
	c.LogWriter = &logBuf
	c.commandContext = stubCommandWithOutput("hello from codex")

	output, err := c.Run(context.Background(), t.TempDir(), "do something")
	require.NoError(t, err)

	assert.Contains(t, output, "hello from codex")
	assert.Contains(t, logBuf.String(), "hello from codex")
}

func TestCodex_Run_NilLogWriter_Fallback(t *testing.T) {
	c := NewCodex(5*time.Minute, testLogger())
	c.LogWriter = nil
	c.commandContext = stubCommandWithOutput("buffered output")

	output, err := c.Run(context.Background(), t.TempDir(), "do something")
	require.NoError(t, err)
	assert.Contains(t, output, "buffered output")
}

func TestCodex_Run_Streaming_NonZeroExit(t *testing.T) {
	var logBuf bytes.Buffer
	c := NewCodex(5*time.Minute, testLogger())
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

func TestCodex_Run_Streaming_Timeout(t *testing.T) {
	var logBuf bytes.Buffer
	c := NewCodex(100*time.Millisecond, testLogger())
	c.LogWriter = &logBuf
	c.commandContext = func(ctx context.Context, name string, args ...string) *exec.Cmd {
		return exec.CommandContext(ctx, "sh", "-c", "echo 'started'; sleep 60")
	}

	output, err := c.Run(context.Background(), t.TempDir(), "slow task")
	require.Error(t, err)
	assert.Contains(t, err.Error(), "timed out")
	assert.Contains(t, output, "started")
	assert.Contains(t, logBuf.String(), "started")
}

func TestCodex_SetLogWriter(t *testing.T) {
	c := NewCodex(5*time.Minute, testLogger())
	assert.Nil(t, c.LogWriter)

	var buf bytes.Buffer
	c.SetLogWriter(&buf)
	assert.Equal(t, &buf, c.LogWriter)

	c.ClearLogWriter()
	assert.Nil(t, c.LogWriter)
}

func TestCodex_Run_VerifiesArgs(t *testing.T) {
	dir := t.TempDir()
	var capturedArgs []string
	c := NewCodex(5*time.Minute, testLogger())
	c.commandContext = func(ctx context.Context, name string, args ...string) *exec.Cmd {
		capturedArgs = args
		return exec.CommandContext(ctx, "true")
	}

	_, err := c.Run(context.Background(), dir, "implement feature X")
	require.NoError(t, err)

	assert.Equal(t, "exec", capturedArgs[0])
	assert.Contains(t, capturedArgs, "--full-auto")
	assert.Contains(t, capturedArgs, "--json")
	assert.Contains(t, capturedArgs, "--cd")
	assert.Contains(t, capturedArgs, dir)
	assert.Equal(t, "implement feature X", capturedArgs[len(capturedArgs)-1])
}
