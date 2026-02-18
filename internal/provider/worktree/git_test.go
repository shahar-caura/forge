package worktree

import (
	"context"
	"log/slog"
	"os"
	"os/exec"
	"path/filepath"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func initBareRepo(t *testing.T) string {
	t.Helper()
	dir := t.TempDir()

	run(t, dir, "git", "init")
	run(t, dir, "git", "config", "user.email", "test@test.com")
	run(t, dir, "git", "config", "user.name", "Test")
	require.NoError(t, os.WriteFile(filepath.Join(dir, "README.md"), []byte("init"), 0o644))
	run(t, dir, "git", "add", ".")
	run(t, dir, "git", "commit", "-m", "init")

	return dir
}

func run(t *testing.T, dir, name string, args ...string) {
	t.Helper()
	cmd := exec.Command(name, args...)
	cmd.Dir = dir
	out, err := cmd.CombinedOutput()
	require.NoError(t, err, "command %s %v failed: %s", name, args, out)
}

func testLogger() *slog.Logger {
	return slog.New(slog.NewTextHandler(os.Stderr, &slog.HandlerOptions{Level: slog.LevelDebug}))
}

func TestCreate_WithGitWorktreeAdd(t *testing.T) {
	repoDir := initBareRepo(t)

	g := New(
		"git worktree add -b {{.Branch}} {{.Path}} {{.BaseBranch}}",
		"git worktree remove --force {{.Path}}",
		true,
		repoDir,
		testLogger(),
	)

	path, err := g.Create(context.Background(), "test-branch", "master")
	require.NoError(t, err)

	expectedPath := filepath.Join(repoDir, ".worktrees", "test-branch")
	assert.Equal(t, expectedPath, path)

	info, err := os.Stat(path)
	require.NoError(t, err)
	assert.True(t, info.IsDir())
}

func TestCreate_WithScript(t *testing.T) {
	repoDir := initBareRepo(t)

	scriptPath := filepath.Join(repoDir, "create-wt.sh")
	script := `#!/bin/sh
set -e
BRANCH="$1"
BASE="$2"
WTPATH="$3"
git worktree add -b "$BRANCH" "$WTPATH" "$BASE"
`
	require.NoError(t, os.WriteFile(scriptPath, []byte(script), 0o755))

	g := New(
		scriptPath+" {{.Branch}} {{.BaseBranch}} {{.Path}}",
		"git worktree remove --force {{.Path}}",
		true,
		repoDir,
		testLogger(),
	)

	path, err := g.Create(context.Background(), "test-branch", "master")
	require.NoError(t, err)

	expectedPath := filepath.Join(repoDir, ".worktrees", "test-branch")
	assert.Equal(t, expectedPath, path)

	info, err := os.Stat(path)
	require.NoError(t, err)
	assert.True(t, info.IsDir())
}

func TestRemove_RealWorktree(t *testing.T) {
	repoDir := initBareRepo(t)

	wtPath := filepath.Join(repoDir, ".worktrees", "rm-branch")
	run(t, repoDir, "git", "worktree", "add", "-b", "rm-branch", wtPath, "master")

	g := New(
		"echo placeholder",
		"git worktree remove --force {{.Path}}",
		true,
		repoDir,
		testLogger(),
	)

	err := g.Remove(context.Background(), wtPath)
	require.NoError(t, err)

	_, err = os.Stat(wtPath)
	assert.True(t, os.IsNotExist(err))
}

func TestRemove_CleanupDisabled(t *testing.T) {
	repoDir := initBareRepo(t)

	wtPath := filepath.Join(repoDir, ".worktrees", "keep-branch")
	run(t, repoDir, "git", "worktree", "add", "-b", "keep-branch", wtPath, "master")

	g := New(
		"echo placeholder",
		"git worktree remove --force {{.Path}}",
		false,
		repoDir,
		testLogger(),
	)

	err := g.Remove(context.Background(), wtPath)
	require.NoError(t, err)

	info, err := os.Stat(wtPath)
	require.NoError(t, err)
	assert.True(t, info.IsDir())
}

func TestCreate_Failure(t *testing.T) {
	repoDir := initBareRepo(t)

	g := New(
		"false",
		"echo ok",
		true,
		repoDir,
		testLogger(),
	)

	_, err := g.Create(context.Background(), "branch", "master")
	require.Error(t, err)
	assert.Contains(t, err.Error(), "worktree create")
}

func TestRenderTemplate_ExpandsTilde(t *testing.T) {
	home, err := os.UserHomeDir()
	require.NoError(t, err)

	fields, err := renderTemplate("~/bin/my-script {{.Branch}} {{.Path}}", templateData{
		Branch: "feat-1",
		Path:   "/tmp/wt",
	})
	require.NoError(t, err)

	assert.Equal(t, filepath.Join(home, "bin/my-script"), fields[0])
	assert.Equal(t, "feat-1", fields[1])
	assert.Equal(t, "/tmp/wt", fields[2])
}

func TestCreate_ContextCancelled(t *testing.T) {
	repoDir := initBareRepo(t)

	g := New(
		"sleep 60",
		"echo ok",
		true,
		repoDir,
		testLogger(),
	)

	ctx, cancel := context.WithCancel(context.Background())
	cancel()

	_, err := g.Create(ctx, "branch", "master")
	require.Error(t, err)
}
