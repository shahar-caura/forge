package pipeline

import (
	"context"
	"os"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestRunHookWithRetry_PassesFirstTime(t *testing.T) {
	agent := &mockAgent{}
	err := runHookWithRetry(context.Background(), "true", t.TempDir(), agent, 2, testLogger())
	require.NoError(t, err)
	assert.False(t, agent.called, "agent should not be called when hook passes")
}

func TestRunHookWithRetry_AgentFixes(t *testing.T) {
	// Hook fails on first run, agent fixes it, hook passes on retry.
	dir := t.TempDir()
	marker := dir + "/fixed"

	// Hook: check if marker file exists.
	hookCmd := "test -f " + marker

	agent := &mockAgent{}
	// Agent "fixes" by creating the marker file.
	agent.output = "fixed"
	originalRun := agent.Run
	_ = originalRun
	callCount := 0
	fixAgent := &funcAgent{
		runFn: func(ctx context.Context, d, prompt string) (string, error) {
			callCount++
			// Simulate agent fixing the issue by creating the marker.
			_ = os.WriteFile(marker, []byte("ok"), 0o644)
			return "fixed", nil
		},
	}

	err := runHookWithRetry(context.Background(), hookCmd, dir, fixAgent, 2, testLogger())
	require.NoError(t, err)
	assert.Equal(t, 1, callCount, "agent should be called once to fix")
}

func TestRunHookWithRetry_RetriesExhausted(t *testing.T) {
	// Hook always fails, agent can't fix it.
	agent := &funcAgent{
		runFn: func(ctx context.Context, dir, prompt string) (string, error) {
			return "tried to fix", nil
		},
	}

	err := runHookWithRetry(context.Background(), "false", t.TempDir(), agent, 2, testLogger())
	require.Error(t, err)
	assert.Contains(t, err.Error(), "pre-commit hook failed after 2 retries")
}

func TestRunHookWithRetry_NilAgent(t *testing.T) {
	err := runHookWithRetry(context.Background(), "false", t.TempDir(), nil, 2, testLogger())
	require.Error(t, err)
	// Should fail fast without retrying.
	assert.NotContains(t, err.Error(), "retries")
}

func TestRunHookWithRetry_ZeroRetries(t *testing.T) {
	agent := &mockAgent{}
	err := runHookWithRetry(context.Background(), "false", t.TempDir(), agent, 0, testLogger())
	require.Error(t, err)
	assert.False(t, agent.called, "agent should not be called with 0 retries")
}

func TestBuildHookFixPrompt(t *testing.T) {
	prompt := buildHookFixPrompt("make fmt && make vet", "vet: unused variable x")
	assert.Contains(t, prompt, "make fmt && make vet")
	assert.Contains(t, prompt, "vet: unused variable x")
	assert.Contains(t, prompt, "Fix ALL reported errors")
}

// funcAgent is a test helper that wraps a function as a provider.Agent.
type funcAgent struct {
	runFn func(ctx context.Context, dir, prompt string) (string, error)
}

func (f *funcAgent) Run(ctx context.Context, dir, prompt string) (string, error) {
	return f.runFn(ctx, dir, prompt)
}

func (f *funcAgent) PromptSuffix() string { return "" }
