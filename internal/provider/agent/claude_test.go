package agent

import (
	"context"
	"fmt"
	"log/slog"
	"os"
	"os/exec"
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

	err := c.Run(context.Background(), t.TempDir(), "do something")
	require.NoError(t, err)
}

func TestRun_Timeout(t *testing.T) {
	c := New(50*time.Millisecond, testLogger())
	c.commandContext = func(ctx context.Context, name string, args ...string) *exec.Cmd {
		return exec.CommandContext(ctx, "sleep", "60")
	}

	err := c.Run(context.Background(), t.TempDir(), "do something slow")
	require.Error(t, err)
	assert.Contains(t, err.Error(), "timed out")
}

func TestRun_NonZeroExit(t *testing.T) {
	c := New(5*time.Minute, testLogger())
	c.commandContext = stubCommand(1, "something went wrong")

	err := c.Run(context.Background(), t.TempDir(), "do something")
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

	err := c.Run(ctx, t.TempDir(), "do something")
	require.Error(t, err)
}
