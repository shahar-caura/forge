package pipeline

import (
	"context"
	"log/slog"
	"os"
	"testing"

	"github.com/shahar-caura/forge/internal/state"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

type cleanupMockVCS struct {
	mockVCS
	prStates map[int]string
}

func (m *cleanupMockVCS) GetPRState(_ context.Context, prNumber int) (string, error) {
	s, ok := m.prStates[prNumber]
	if !ok {
		return "OPEN", nil
	}
	return s, nil
}

type cleanupMockWorktree struct {
	removed []string
}

func (m *cleanupMockWorktree) Create(_ context.Context, _, _ string) (string, error) {
	return "", nil
}

func (m *cleanupMockWorktree) Remove(_ context.Context, path string) error {
	m.removed = append(m.removed, path)
	return nil
}

func TestCleanupMergedWorktrees_RemovesMerged(t *testing.T) {
	// Use a temp dir for run state files so tests don't pollute the real .forge/runs.
	tmpDir := t.TempDir()
	origDir, _ := os.Getwd()
	require.NoError(t, os.Chdir(tmpDir))
	defer func() { _ = os.Chdir(origDir) }()

	// Create run states: one merged, one open, one with no PR.
	merged := state.New("run-merged", "plan.md")
	merged.PRNumber = 10
	merged.WorktreePath = "/tmp/wt-merged"
	require.NoError(t, merged.Save())

	open := state.New("run-open", "plan.md")
	open.PRNumber = 20
	open.WorktreePath = "/tmp/wt-open"
	require.NoError(t, open.Save())

	noPR := state.New("run-nopr", "plan.md")
	noPR.WorktreePath = "/tmp/wt-nopr"
	require.NoError(t, noPR.Save())

	vc := &cleanupMockVCS{prStates: map[int]string{
		10: "MERGED",
		20: "OPEN",
	}}
	wt := &cleanupMockWorktree{}
	logger := slog.New(slog.NewTextHandler(os.Stderr, &slog.HandlerOptions{Level: slog.LevelDebug}))

	cleaned, err := CleanupMergedWorktrees(context.Background(), vc, wt, logger)
	require.NoError(t, err)
	assert.Equal(t, 1, cleaned)
	assert.Equal(t, []string{"/tmp/wt-merged"}, wt.removed)

	// Verify merged state has WorktreePath cleared.
	reloaded, err := state.Load("run-merged")
	require.NoError(t, err)
	assert.Empty(t, reloaded.WorktreePath)

	// Verify open state unchanged.
	reloaded, err = state.Load("run-open")
	require.NoError(t, err)
	assert.Equal(t, "/tmp/wt-open", reloaded.WorktreePath)
}

func TestCleanupMergedWorktrees_NoWorktrees(t *testing.T) {
	tmpDir := t.TempDir()
	origDir, _ := os.Getwd()
	require.NoError(t, os.Chdir(tmpDir))
	defer func() { _ = os.Chdir(origDir) }()

	// No run states at all.
	vc := &cleanupMockVCS{prStates: map[int]string{}}
	wt := &cleanupMockWorktree{}
	logger := slog.New(slog.NewTextHandler(os.Stderr, &slog.HandlerOptions{Level: slog.LevelDebug}))

	cleaned, err := CleanupMergedWorktrees(context.Background(), vc, wt, logger)
	require.NoError(t, err)
	assert.Equal(t, 0, cleaned)
	assert.Empty(t, wt.removed)
}

func TestCleanupMergedWorktrees_SkipsEmptyWorktreePath(t *testing.T) {
	tmpDir := t.TempDir()
	origDir, _ := os.Getwd()
	require.NoError(t, os.Chdir(tmpDir))
	defer func() { _ = os.Chdir(origDir) }()

	// Run state with PR but no worktree path (already cleaned).
	rs := state.New("run-no-wt", "plan.md")
	rs.PRNumber = 30
	rs.WorktreePath = ""
	require.NoError(t, rs.Save())

	vc := &cleanupMockVCS{prStates: map[int]string{30: "MERGED"}}
	wt := &cleanupMockWorktree{}
	logger := slog.New(slog.NewTextHandler(os.Stderr, &slog.HandlerOptions{Level: slog.LevelDebug}))

	cleaned, err := CleanupMergedWorktrees(context.Background(), vc, wt, logger)
	require.NoError(t, err)
	assert.Equal(t, 0, cleaned)
	assert.Empty(t, wt.removed)
}
