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

func TestGemini_Run_Success(t *testing.T) {
	g := NewGemini(5*time.Minute, testLogger())
	g.commandContext = stubCommand(0, "")

	output, err := g.Run(context.Background(), t.TempDir(), "do something")
	require.NoError(t, err)
	_ = output
}

func TestGemini_Run_Timeout(t *testing.T) {
	g := NewGemini(50*time.Millisecond, testLogger())
	g.commandContext = func(ctx context.Context, name string, args ...string) *exec.Cmd {
		return exec.CommandContext(ctx, "sleep", "60")
	}

	_, err := g.Run(context.Background(), t.TempDir(), "do something slow")
	require.Error(t, err)
	assert.Contains(t, err.Error(), "timed out")
}

func TestGemini_Run_NonZeroExit(t *testing.T) {
	g := NewGemini(5*time.Minute, testLogger())
	g.commandContext = stubCommand(1, "something went wrong")

	_, err := g.Run(context.Background(), t.TempDir(), "do something")
	require.Error(t, err)
	assert.Contains(t, err.Error(), "agent failed")
	assert.Contains(t, err.Error(), "something went wrong")
}

func TestGemini_Run_ContextCancelled(t *testing.T) {
	g := NewGemini(5*time.Minute, testLogger())
	g.commandContext = func(ctx context.Context, name string, args ...string) *exec.Cmd {
		return exec.CommandContext(ctx, "sleep", "60")
	}

	ctx, cancel := context.WithCancel(context.Background())
	cancel()

	_, err := g.Run(ctx, t.TempDir(), "do something")
	require.Error(t, err)
}

func TestGemini_Run_StreamingToLogWriter(t *testing.T) {
	var logBuf bytes.Buffer
	g := NewGemini(5*time.Minute, testLogger())
	g.LogWriter = &logBuf
	g.commandContext = stubCommandWithOutput("hello from gemini")

	output, err := g.Run(context.Background(), t.TempDir(), "do something")
	require.NoError(t, err)

	assert.Contains(t, output, "hello from gemini")
	assert.Contains(t, logBuf.String(), "hello from gemini")
}

func TestGemini_Run_NilLogWriter_Fallback(t *testing.T) {
	g := NewGemini(5*time.Minute, testLogger())
	g.LogWriter = nil
	g.commandContext = stubCommandWithOutput("buffered output")

	output, err := g.Run(context.Background(), t.TempDir(), "do something")
	require.NoError(t, err)
	assert.Contains(t, output, "buffered output")
}

func TestGemini_Run_Streaming_NonZeroExit(t *testing.T) {
	var logBuf bytes.Buffer
	g := NewGemini(5*time.Minute, testLogger())
	g.LogWriter = &logBuf
	g.commandContext = func(ctx context.Context, name string, args ...string) *exec.Cmd {
		return exec.CommandContext(ctx, "sh", "-c", "echo 'partial output'; exit 1")
	}

	output, err := g.Run(context.Background(), t.TempDir(), "do something")
	require.Error(t, err)
	assert.Contains(t, err.Error(), "agent failed")
	assert.Contains(t, output, "partial output")
	assert.Contains(t, logBuf.String(), "partial output")
}

func TestGemini_Run_Streaming_Timeout(t *testing.T) {
	var logBuf bytes.Buffer
	g := NewGemini(100*time.Millisecond, testLogger())
	g.LogWriter = &logBuf
	g.commandContext = func(ctx context.Context, name string, args ...string) *exec.Cmd {
		return exec.CommandContext(ctx, "sh", "-c", "echo 'started'; sleep 60")
	}

	output, err := g.Run(context.Background(), t.TempDir(), "slow task")
	require.Error(t, err)
	assert.Contains(t, err.Error(), "timed out")
	assert.Contains(t, output, "started")
	assert.Contains(t, logBuf.String(), "started")
}

func TestGemini_SetLogWriter(t *testing.T) {
	g := NewGemini(5*time.Minute, testLogger())
	assert.Nil(t, g.LogWriter)

	var buf bytes.Buffer
	g.SetLogWriter(&buf)
	assert.Equal(t, &buf, g.LogWriter)

	g.ClearLogWriter()
	assert.Nil(t, g.LogWriter)
}

func TestGemini_Run_VerifiesArgs(t *testing.T) {
	var capturedArgs []string
	g := NewGemini(5*time.Minute, testLogger())
	g.commandContext = func(ctx context.Context, name string, args ...string) *exec.Cmd {
		capturedArgs = args
		return exec.CommandContext(ctx, "true")
	}

	_, err := g.Run(context.Background(), t.TempDir(), "implement feature X")
	require.NoError(t, err)

	assert.Contains(t, capturedArgs, "--yolo")
	assert.Contains(t, capturedArgs, "--output-format")
	assert.Contains(t, capturedArgs, "json")
	// Prompt follows -p flag (index 0="-p", 1=prompt).
	assert.Equal(t, "-p", capturedArgs[0])
	assert.Equal(t, "implement feature X", capturedArgs[1])
}
