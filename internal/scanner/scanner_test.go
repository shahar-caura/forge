package scanner

import (
	"os"
	"path/filepath"
	"runtime"
	"testing"

	"github.com/shahar-caura/forge/internal/state"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestScanRepos_DiscoversNestedRepoRuns(t *testing.T) {
	root := t.TempDir()

	repoA := filepath.Join(root, "repo-a")
	repoB := filepath.Join(root, "nested", "repo-b")

	createRunFile(t, repoA, "run-a")
	createRunFile(t, repoB, "run-b")

	repos, err := ScanRepos([]string{root})
	require.NoError(t, err)
	require.Len(t, repos, 2)

	repoAResolved := mustResolvePath(t, repoA)
	repoBResolved := mustResolvePath(t, repoB)

	byPath := make(map[string]RepoRuns)
	for _, repo := range repos {
		byPath[repo.RepoPath] = repo
	}

	repoARuns, ok := byPath[repoAResolved]
	require.True(t, ok)
	assert.Equal(t, "repo-a", repoARuns.RepoName)
	require.Len(t, repoARuns.Runs, 1)
	assert.Equal(t, "run-a", repoARuns.Runs[0].ID)

	repoBRuns, ok := byPath[repoBResolved]
	require.True(t, ok)
	assert.Equal(t, "repo-b", repoBRuns.RepoName)
	require.Len(t, repoBRuns.Runs, 1)
	assert.Equal(t, "run-b", repoBRuns.Runs[0].ID)
}

func TestScanRepos_EmptyRunsAndHiddenDirectories(t *testing.T) {
	root := t.TempDir()

	emptyRepo := filepath.Join(root, "repo-empty")
	require.NoError(t, os.MkdirAll(filepath.Join(emptyRepo, ".forge", "runs"), 0o755))

	hiddenRepo := filepath.Join(root, ".hidden", "repo-hidden")
	createRunFile(t, hiddenRepo, "hidden-run")

	repos, err := ScanRepos([]string{root})
	require.NoError(t, err)
	require.Len(t, repos, 1)

	assert.Equal(t, mustResolvePath(t, emptyRepo), repos[0].RepoPath)
	assert.Equal(t, "repo-empty", repos[0].RepoName)
	assert.Empty(t, repos[0].Runs)
}

func TestScanRepos_SkipsSymlinkedDirectories(t *testing.T) {
	if runtime.GOOS == "windows" {
		t.Skip("symlink test is platform-specific")
	}

	root := t.TempDir()
	actualRepo := filepath.Join(root, "actual-repo")
	createRunFile(t, actualRepo, "run-actual")

	linkPath := filepath.Join(root, "repo-link")
	require.NoError(t, os.Symlink(actualRepo, linkPath))

	repos, err := ScanRepos([]string{root})
	require.NoError(t, err)
	require.Len(t, repos, 1)
	assert.Equal(t, mustResolvePath(t, actualRepo), repos[0].RepoPath)
}

func TestScanRepos_SkipsUnreadableDirectories(t *testing.T) {
	if runtime.GOOS == "windows" {
		t.Skip("chmod permission semantics differ on windows")
	}

	root := t.TempDir()
	goodRepo := filepath.Join(root, "good-repo")
	createRunFile(t, goodRepo, "good-run")

	blockedDir := filepath.Join(root, "blocked")
	require.NoError(t, os.MkdirAll(filepath.Join(blockedDir, "child"), 0o755))
	require.NoError(t, os.Chmod(blockedDir, 0o000))
	defer func() {
		_ = os.Chmod(blockedDir, 0o755)
	}()

	repos, err := ScanRepos([]string{root})
	require.NoError(t, err)
	require.Len(t, repos, 1)
	assert.Equal(t, mustResolvePath(t, goodRepo), repos[0].RepoPath)
	require.Len(t, repos[0].Runs, 1)
	assert.Equal(t, "good-run", repos[0].Runs[0].ID)
}

func createRunFile(t *testing.T, repoPath, id string) {
	t.Helper()

	runsDir := filepath.Join(repoPath, ".forge", "runs")
	require.NoError(t, os.MkdirAll(runsDir, 0o755))

	rs := state.New(id, "plans/test.md")

	cleanup := setRunsDirForTest(t, runsDir)
	defer cleanup()
	require.NoError(t, rs.Save())
}

func setRunsDirForTest(t *testing.T, dir string) func() {
	t.Helper()
	state.SetRunsDir(dir)
	return func() { state.SetRunsDir(".forge/runs") }
}

func mustResolvePath(t *testing.T, path string) string {
	t.Helper()
	resolved, err := filepath.EvalSymlinks(path)
	if err == nil {
		return resolved
	}
	abs, absErr := filepath.Abs(path)
	require.NoError(t, absErr)
	return abs
}
