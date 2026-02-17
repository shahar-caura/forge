package vcs

import (
	"context"
	"fmt"
	"log/slog"
	"os"
	"os/exec"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func testLogger() *slog.Logger {
	return slog.New(slog.NewTextHandler(os.Stderr, &slog.HandlerOptions{Level: slog.LevelDebug}))
}

// callTracker records which commands were invoked and returns scripted results.
type callTracker struct {
	calls   []string
	results map[string]stubResult
}

type stubResult struct {
	stdout   string
	exitCode int
}

func (ct *callTracker) commandContext(ctx context.Context, name string, args ...string) *exec.Cmd {
	key := name + " " + joinArgs(args)
	ct.calls = append(ct.calls, key)

	r, ok := ct.results[name]
	if !ok {
		// Default: succeed silently.
		return exec.CommandContext(ctx, "true")
	}

	if r.exitCode != 0 {
		return exec.CommandContext(ctx, "sh", "-c", fmt.Sprintf("echo '%s' >&2; exit %d", r.stdout, r.exitCode))
	}
	return exec.CommandContext(ctx, "echo", "-n", r.stdout)
}

func joinArgs(args []string) string {
	s := ""
	for i, a := range args {
		if i > 0 {
			s += " "
		}
		s += a
	}
	return s
}

func TestCommitAndPush_Success(t *testing.T) {
	ct := &callTracker{results: map[string]stubResult{}}
	g := New("owner/repo", testLogger())
	g.commandContext = ct.commandContext

	err := g.CommitAndPush(context.Background(), t.TempDir(), "feat-branch", "add feature")
	require.NoError(t, err)
	assert.Len(t, ct.calls, 3)
}

func TestCommitAndPush_CommitFails(t *testing.T) {
	ct := &callTracker{results: map[string]stubResult{
		"git": {stdout: "nothing to commit", exitCode: 1},
	}}
	g := New("owner/repo", testLogger())
	g.commandContext = ct.commandContext

	err := g.CommitAndPush(context.Background(), t.TempDir(), "feat-branch", "add feature")
	require.Error(t, err)
	assert.Contains(t, err.Error(), "git add")
}

func TestCreatePR_Success(t *testing.T) {
	ct := &callTracker{results: map[string]stubResult{
		"gh": {stdout: "https://github.com/owner/repo/pull/42", exitCode: 0},
	}}
	g := New("owner/repo", testLogger())
	g.commandContext = ct.commandContext

	pr, err := g.CreatePR(context.Background(), "feat-branch", "main", "Add feature", "body text")
	require.NoError(t, err)
	assert.Equal(t, "https://github.com/owner/repo/pull/42", pr.URL)
	assert.Equal(t, 42, pr.Number)
}

func TestCreatePR_Failure(t *testing.T) {
	ct := &callTracker{results: map[string]stubResult{
		"gh": {stdout: "auth required", exitCode: 1},
	}}
	g := New("owner/repo", testLogger())
	g.commandContext = ct.commandContext

	_, err := g.CreatePR(context.Background(), "feat-branch", "main", "title", "body")
	require.Error(t, err)
	assert.Contains(t, err.Error(), "gh pr create")
}
