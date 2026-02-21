package pipeline

import (
	"context"
	"errors"
	"os"
	"testing"

	"log/slog"

	"github.com/shahar-caura/forge/internal/provider"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func poolLogger() *slog.Logger {
	return slog.New(slog.NewTextHandler(os.Stderr, &slog.HandlerOptions{Level: slog.LevelDebug}))
}

func TestAgentPool_RoundRobin(t *testing.T) {
	a1 := &mockAgent{output: "a1"}
	a2 := &mockAgent{output: "a2"}
	a3 := &mockAgent{output: "a3"}

	pool := NewAgentPool([]provider.Agent{a1, a2, a3}, []string{"claude", "codex", "gemini"})

	assert.Equal(t, a1, pool.Assign(0))
	assert.Equal(t, a2, pool.Assign(1))
	assert.Equal(t, a3, pool.Assign(2))
	assert.Equal(t, a1, pool.Assign(3)) // wraps around
	assert.Equal(t, a2, pool.Assign(4))

	assert.Equal(t, "claude", pool.AssignName(0))
	assert.Equal(t, "codex", pool.AssignName(1))
	assert.Equal(t, "gemini", pool.AssignName(2))
	assert.Equal(t, "claude", pool.AssignName(3))
}

func TestAgentPool_SingleAgent(t *testing.T) {
	a := &mockAgent{output: "only"}
	pool := NewAgentPool([]provider.Agent{a}, []string{"claude"})

	assert.Equal(t, a, pool.Assign(0))
	assert.Equal(t, a, pool.Assign(1))
	assert.Equal(t, a, pool.Assign(5))
	assert.Equal(t, "claude", pool.AssignName(0))
	assert.Equal(t, "claude", pool.AssignName(99))
}

func TestAgentPool_Primary(t *testing.T) {
	a1 := &mockAgent{output: "primary"}
	a2 := &mockAgent{output: "secondary"}
	pool := NewAgentPool([]provider.Agent{a1, a2}, []string{"claude", "codex"})

	assert.Equal(t, a1, pool.Primary())
}

func TestAgentPool_Len(t *testing.T) {
	a1 := &mockAgent{}
	a2 := &mockAgent{}
	pool := NewAgentPool([]provider.Agent{a1, a2}, []string{"claude", "codex"})
	assert.Equal(t, 2, pool.Len())

	singlePool := NewAgentPool([]provider.Agent{a1}, []string{"claude"})
	assert.Equal(t, 1, singlePool.Len())
}

// --- retryableError tests ---

func TestRetryableError(t *testing.T) {
	tests := []struct {
		name string
		err  error
		want bool
	}{
		{"nil error", nil, false},
		{"rate limit", errors.New("rate limit exceeded"), true},
		{"429 status", errors.New("HTTP 429 Too Many Requests"), true},
		{"quota exceeded", errors.New("quota exceeded for model"), true},
		{"unauthorized", errors.New("unauthorized: invalid API key"), true},
		{"403 forbidden", errors.New("HTTP 403 Forbidden"), true},
		{"credentials", errors.New("invalid credentials"), true},
		{"timed out", errors.New("agent timed out after 45m"), true},
		{"exceeded", errors.New("token limit exceeded"), true},
		{"generic error", errors.New("syntax error in file"), false},
		{"no changes", errors.New("agent produced no file changes"), false},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			assert.Equal(t, tt.want, retryableError(tt.err))
		})
	}
}

// --- RunWithFallback tests ---

func TestRunWithFallback_PrimarySucceeds(t *testing.T) {
	a1 := &mockAgent{output: "result"}
	a2 := &mockAgent{output: "backup"}
	pool := NewAgentPool([]provider.Agent{a1, a2}, []string{"claude", "codex"})

	output, name, err := pool.RunWithFallback(context.Background(), 0, "/dir", "prompt", poolLogger())

	require.NoError(t, err)
	assert.Equal(t, "result", output)
	assert.Equal(t, "claude", name)
	assert.True(t, a1.called)
	assert.False(t, a2.called)
}

func TestRunWithFallback_PrimaryRateLimited_FallbackSucceeds(t *testing.T) {
	a1 := &mockAgent{err: errors.New("rate limit exceeded")}
	a2 := &mockAgent{output: "backup result"}
	pool := NewAgentPool([]provider.Agent{a1, a2}, []string{"claude", "codex"})

	output, name, err := pool.RunWithFallback(context.Background(), 0, "/dir", "prompt", poolLogger())

	require.NoError(t, err)
	assert.Equal(t, "backup result", output)
	assert.Equal(t, "codex", name)
	assert.True(t, a1.called)
	assert.True(t, a2.called)
}

func TestRunWithFallback_AllFail(t *testing.T) {
	a1 := &mockAgent{err: errors.New("rate limit exceeded")}
	a2 := &mockAgent{err: errors.New("quota exceeded")}
	pool := NewAgentPool([]provider.Agent{a1, a2}, []string{"claude", "codex"})

	_, _, err := pool.RunWithFallback(context.Background(), 0, "/dir", "prompt", poolLogger())

	require.Error(t, err)
	assert.Contains(t, err.Error(), "agent codex")
	assert.True(t, a1.called)
	assert.True(t, a2.called)
}

func TestRunWithFallback_NonRetryable_NoFallback(t *testing.T) {
	a1 := &mockAgent{err: errors.New("syntax error in generated code")}
	a2 := &mockAgent{output: "should not be called"}
	pool := NewAgentPool([]provider.Agent{a1, a2}, []string{"claude", "codex"})

	_, _, err := pool.RunWithFallback(context.Background(), 0, "/dir", "prompt", poolLogger())

	require.Error(t, err)
	assert.Contains(t, err.Error(), "agent claude")
	assert.True(t, a1.called)
	assert.False(t, a2.called, "non-retryable error should not trigger fallback")
}

func TestRunWithFallback_SingleAgentPool(t *testing.T) {
	a := &mockAgent{err: errors.New("rate limit exceeded")}
	pool := NewAgentPool([]provider.Agent{a}, []string{"claude"})

	_, _, err := pool.RunWithFallback(context.Background(), 0, "/dir", "prompt", poolLogger())

	require.Error(t, err)
	assert.Contains(t, err.Error(), "agent claude")
}

func TestRunWithFallback_StartIdxWraps(t *testing.T) {
	a1 := &mockAgent{err: errors.New("rate limit exceeded")}
	a2 := &mockAgent{output: "result from codex"}
	a3 := &mockAgent{output: "result from gemini"}
	pool := NewAgentPool([]provider.Agent{a1, a2, a3}, []string{"claude", "codex", "gemini"})

	// Start at index 2 (gemini), which rate-limits, then wraps to claude (also rate-limits), then codex succeeds.
	a3.err = errors.New("429 too many requests")
	output, name, err := pool.RunWithFallback(context.Background(), 2, "/dir", "prompt", poolLogger())

	require.NoError(t, err)
	assert.Equal(t, "result from codex", output)
	assert.Equal(t, "codex", name)
}
