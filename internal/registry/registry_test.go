package registry

import (
	"os"
	"path/filepath"
	"testing"

	"github.com/shahar-caura/forge/internal/state"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func setup(t *testing.T) string {
	t.Helper()
	dir := t.TempDir()
	SetPath(filepath.Join(dir, "repos.yaml"))
	t.Cleanup(func() { SetPath("") })
	return dir
}

func TestTouchAndList(t *testing.T) {
	setup(t)

	Touch("/tmp/repo-a")
	Touch("/tmp/repo-b")

	repos, err := List()
	require.NoError(t, err)
	assert.Len(t, repos, 2)

	// Most recently touched should be first.
	assert.Equal(t, "/tmp/repo-b", repos[0].Path)
	assert.Equal(t, "repo-b", repos[0].Name)
	assert.Equal(t, "/tmp/repo-a", repos[1].Path)
}

func TestTouchUpserts(t *testing.T) {
	setup(t)

	Touch("/tmp/repo-a")
	Touch("/tmp/repo-b")
	Touch("/tmp/repo-a") // update last_used

	repos, err := List()
	require.NoError(t, err)
	assert.Len(t, repos, 2)
	assert.Equal(t, "/tmp/repo-a", repos[0].Path) // most recent
}

func TestRemove(t *testing.T) {
	setup(t)

	Touch("/tmp/repo-a")
	Touch("/tmp/repo-b")

	err := Remove("/tmp/repo-a")
	require.NoError(t, err)

	repos, err := List()
	require.NoError(t, err)
	assert.Len(t, repos, 1)
	assert.Equal(t, "/tmp/repo-b", repos[0].Path)
}

func TestListRunsAcrossRepos(t *testing.T) {
	dir := setup(t)

	// Create two fake repo directories with runs.
	repoA := filepath.Join(dir, "repo-a")
	repoB := filepath.Join(dir, "repo-b")
	runsA := filepath.Join(repoA, ".forge", "runs")
	runsB := filepath.Join(repoB, ".forge", "runs")
	require.NoError(t, os.MkdirAll(runsA, 0o755))
	require.NoError(t, os.MkdirAll(runsB, 0o755))

	// Save runs into each repo's directory.
	origDir := ".forge/runs"
	state.SetRunsDir(runsA)
	rs1 := state.New("run-a1", "plans/a.md")
	require.NoError(t, rs1.Save())

	state.SetRunsDir(runsB)
	rs2 := state.New("run-b1", "plans/b.md")
	require.NoError(t, rs2.Save())

	state.SetRunsDir(origDir)
	t.Cleanup(func() { state.SetRunsDir(origDir) })

	// Register repos.
	Touch(repoA)
	Touch(repoB)

	repoRuns, err := ListRuns()
	require.NoError(t, err)
	assert.Len(t, repoRuns, 2)

	// Verify each repo has its run.
	totalRuns := 0
	for _, rr := range repoRuns {
		totalRuns += len(rr.Runs)
	}
	assert.Equal(t, 2, totalRuns)
}

func TestListEmptyRegistry(t *testing.T) {
	setup(t)

	repos, err := List()
	require.NoError(t, err)
	assert.Empty(t, repos)
}

func TestListRunsSkipsMissingRepos(t *testing.T) {
	setup(t)

	Touch("/nonexistent/repo")

	repoRuns, err := ListRuns()
	require.NoError(t, err)
	assert.Empty(t, repoRuns)
}
