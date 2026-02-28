package state

import (
	"os"
	"path/filepath"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"gopkg.in/yaml.v3"
)

func setupTestDir(t *testing.T) func() {
	t.Helper()
	orig := runsDir
	runsDir = filepath.Join(t.TempDir(), ".forge", "runs")
	return func() { runsDir = orig }
}

func TestNew_CorrectInitialState(t *testing.T) {
	rs := New("20260217-120000-auth", "plans/auth.md")

	assert.Equal(t, "20260217-120000-auth", rs.ID)
	assert.Equal(t, "plans/auth.md", rs.PlanPath)
	assert.Equal(t, RunActive, rs.Status)
	assert.False(t, rs.CreatedAt.IsZero())
	assert.False(t, rs.UpdatedAt.IsZero())
	require.Len(t, rs.Steps, 11)

	for i, step := range rs.Steps {
		assert.Equal(t, StepNames[i], step.Name)
		assert.Equal(t, StepPending, step.Status)
		assert.Empty(t, step.Error)
	}
}

func TestSave_CreatesRunsDirectory(t *testing.T) {
	cleanup := setupTestDir(t)
	defer cleanup()

	rs := New("20260217-120000-auth", "plans/auth.md")
	require.NoError(t, rs.Save())

	_, err := os.Stat(runsDir)
	require.NoError(t, err, "runs directory should exist")

	_, err = os.Stat(filepath.Join(runsDir, rs.ID+".yaml"))
	require.NoError(t, err, "state file should exist")
}

func TestSaveAndLoad_RoundTrip(t *testing.T) {
	cleanup := setupTestDir(t)
	defer cleanup()

	rs := New("20260217-120000-auth", "plans/auth.md")
	rs.Branch = "forge/auth"
	rs.WorktreePath = "/tmp/wt"
	rs.PRUrl = "https://github.com/owner/repo/pull/42"
	rs.PRNumber = 42
	rs.CRRetryCount = 3
	rs.Steps[0].Status = StepCompleted
	rs.Steps[1].Status = StepFailed
	rs.Steps[1].Error = "branch conflict"

	require.NoError(t, rs.Save())

	loaded, err := Load(rs.ID)
	require.NoError(t, err)

	assert.Equal(t, rs.ID, loaded.ID)
	assert.Equal(t, rs.PlanPath, loaded.PlanPath)
	assert.Equal(t, rs.Status, loaded.Status)
	assert.Equal(t, rs.Branch, loaded.Branch)
	assert.Equal(t, rs.WorktreePath, loaded.WorktreePath)
	assert.Equal(t, rs.PRUrl, loaded.PRUrl)
	assert.Equal(t, rs.PRNumber, loaded.PRNumber)
	assert.Equal(t, 3, loaded.CRRetryCount)
	require.Len(t, loaded.Steps, 11)
	assert.Equal(t, StepCompleted, loaded.Steps[0].Status)
	assert.Equal(t, StepFailed, loaded.Steps[1].Status)
	assert.Equal(t, "branch conflict", loaded.Steps[1].Error)
}

func TestLoad_Nonexistent(t *testing.T) {
	cleanup := setupTestDir(t)
	defer cleanup()

	_, err := Load("nonexistent-id")
	require.Error(t, err)
	assert.Contains(t, err.Error(), "loading run state")
}

func TestLoad_CorruptYAML(t *testing.T) {
	cleanup := setupTestDir(t)
	defer cleanup()

	require.NoError(t, os.MkdirAll(runsDir, 0o755))
	path := filepath.Join(runsDir, "corrupt.yaml")
	require.NoError(t, os.WriteFile(path, []byte(":\n\t- bad\n\t  yaml"), 0o644))

	_, err := Load("corrupt")
	require.Error(t, err)
	assert.Contains(t, err.Error(), "parsing run state")
}

func TestList_Empty(t *testing.T) {
	cleanup := setupTestDir(t)
	defer cleanup()

	runs, err := List()
	require.NoError(t, err)
	assert.Empty(t, runs)
}

func TestList_MultipleSortedByCreatedAtDesc(t *testing.T) {
	cleanup := setupTestDir(t)
	defer cleanup()

	// Create runs with different timestamps.
	older := New("20260215-100000-older", "plans/older.md")
	older.CreatedAt = time.Date(2026, 2, 15, 10, 0, 0, 0, time.UTC)
	require.NoError(t, older.Save())

	newer := New("20260217-100000-newer", "plans/newer.md")
	newer.CreatedAt = time.Date(2026, 2, 17, 10, 0, 0, 0, time.UTC)
	require.NoError(t, newer.Save())

	middle := New("20260216-100000-middle", "plans/middle.md")
	middle.CreatedAt = time.Date(2026, 2, 16, 10, 0, 0, 0, time.UTC)
	require.NoError(t, middle.Save())

	runs, err := List()
	require.NoError(t, err)
	require.Len(t, runs, 3)

	assert.Equal(t, "20260217-100000-newer", runs[0].ID)
	assert.Equal(t, "20260216-100000-middle", runs[1].ID)
	assert.Equal(t, "20260215-100000-older", runs[2].ID)
}

func TestCleanup_DeletesOnlyCompletedOlderThanRetention(t *testing.T) {
	cleanup := setupTestDir(t)
	defer cleanup()

	retention := 24 * time.Hour

	// Old completed run — should be deleted.
	oldCompleted := New("old-completed", "plans/old.md")
	oldCompleted.Status = RunCompleted
	require.NoError(t, oldCompleted.Save())
	backdateRun(t, oldCompleted, time.Now().Add(-48*time.Hour))

	// Recent completed run — should NOT be deleted.
	recentCompleted := New("recent-completed", "plans/recent.md")
	recentCompleted.Status = RunCompleted
	require.NoError(t, recentCompleted.Save())

	// Old failed run — should NOT be deleted (not completed).
	oldFailed := New("old-failed", "plans/failed.md")
	oldFailed.Status = RunFailed
	require.NoError(t, oldFailed.Save())
	backdateRun(t, oldFailed, time.Now().Add(-48*time.Hour))

	// Old active run — should NOT be deleted (not completed).
	oldActive := New("old-active", "plans/active.md")
	oldActive.Status = RunActive
	require.NoError(t, oldActive.Save())
	backdateRun(t, oldActive, time.Now().Add(-48*time.Hour))

	deleted, err := Cleanup(retention)
	require.NoError(t, err)
	assert.Equal(t, 1, deleted)

	// Verify which files remain.
	runs, err := List()
	require.NoError(t, err)
	assert.Len(t, runs, 3)

	ids := make([]string, len(runs))
	for i, r := range runs {
		ids[i] = r.ID
	}
	assert.NotContains(t, ids, "old-completed")
	assert.Contains(t, ids, "recent-completed")
	assert.Contains(t, ids, "old-failed")
	assert.Contains(t, ids, "old-active")
}

func TestStepIndex_ExactMatch(t *testing.T) {
	idx, ok := StepIndex("commit and push")
	require.True(t, ok)
	assert.Equal(t, 5, idx)
}

func TestStepIndex_Hyphenated(t *testing.T) {
	idx, ok := StepIndex("commit-and-push")
	require.True(t, ok)
	assert.Equal(t, 5, idx)
}

func TestStepIndex_CaseInsensitive(t *testing.T) {
	idx, ok := StepIndex("Create PR")
	require.True(t, ok)
	assert.Equal(t, 6, idx)
}

func TestStepIndex_NotFound(t *testing.T) {
	_, ok := StepIndex("nonexistent")
	assert.False(t, ok)
}

func TestResetFrom_MiddleStep(t *testing.T) {
	rs := New("test", "plan.md")
	// Mark all steps completed first.
	for i := range rs.Steps {
		rs.Steps[i].Status = StepCompleted
	}
	rs.Status = RunCompleted

	rs.ResetFrom(5) // "commit and push"

	// Steps 0-4 should be completed, 5+ should be pending.
	for i := 0; i < 5; i++ {
		assert.Equal(t, StepCompleted, rs.Steps[i].Status, "step %d (%s)", i, rs.Steps[i].Name)
	}
	for i := 5; i < len(rs.Steps); i++ {
		assert.Equal(t, StepPending, rs.Steps[i].Status, "step %d (%s)", i, rs.Steps[i].Name)
	}
	assert.Equal(t, RunActive, rs.Status)
}

func TestResetFrom_FirstStep(t *testing.T) {
	rs := New("test", "plan.md")
	for i := range rs.Steps {
		rs.Steps[i].Status = StepCompleted
	}

	rs.ResetFrom(0)

	for _, step := range rs.Steps {
		assert.Equal(t, StepPending, step.Status)
	}
}

func TestResetFrom_ClearsErrors(t *testing.T) {
	rs := New("test", "plan.md")
	rs.Steps[5].Status = StepFailed
	rs.Steps[5].Error = "push failed"

	rs.ResetFrom(5)

	assert.Equal(t, StepPending, rs.Steps[5].Status)
	assert.Empty(t, rs.Steps[5].Error)
}

// backdateRun re-writes a run state file with a specific UpdatedAt timestamp.
func backdateRun(t *testing.T, rs *RunState, updatedAt time.Time) {
	t.Helper()
	rs.UpdatedAt = updatedAt
	data, err := yaml.Marshal(rs)
	require.NoError(t, err)
	require.NoError(t, os.WriteFile(filepath.Join(runsDir, rs.ID+".yaml"), data, 0o644))
}
