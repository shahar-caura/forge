package config

import (
	"os"
	"path/filepath"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

const validYAML = `
vcs:
  provider: github
  repo: owner/repo
  base_branch: main
agent:
  provider: claude
  timeout: 30m
worktree:
  create_cmd: "git worktree add {{.Branch}}"
  remove_cmd: "git worktree remove {{.Path}}"
  cleanup: true
tracker:
  provider: jira
  project: PROJ
  base_url: https://jira.example.com
  email: user@example.com
  token: secret
notifier:
  provider: slack
  webhook_url: https://hooks.slack.com/xxx
`

func writeConfig(t *testing.T, content string) string {
	t.Helper()
	dir := t.TempDir()
	path := filepath.Join(dir, "forge.yaml")
	require.NoError(t, os.WriteFile(path, []byte(content), 0o644))
	return path
}

func TestLoad_ValidConfig(t *testing.T) {
	path := writeConfig(t, validYAML)

	cfg, err := Load(path)
	require.NoError(t, err)

	assert.Equal(t, "github", cfg.VCS.Provider)
	assert.Equal(t, "owner/repo", cfg.VCS.Repo)
	assert.Equal(t, "main", cfg.VCS.BaseBranch)
	assert.Equal(t, "claude", cfg.Agent.Provider)
	assert.Equal(t, 30*time.Minute, cfg.Agent.Timeout.Duration)
	assert.Equal(t, "git worktree add {{.Branch}}", cfg.Worktree.CreateCmd)
	assert.Equal(t, "git worktree remove {{.Path}}", cfg.Worktree.RemoveCmd)
	assert.True(t, cfg.Worktree.Cleanup)
}

func TestLoad_EnvVarExpansion(t *testing.T) {
	t.Setenv("FORGE_REPO", "myorg/myrepo")
	t.Setenv("FORGE_TOKEN", "tok-123")

	yaml := `
vcs:
  provider: github
  repo: ${FORGE_REPO}
  base_branch: main
agent:
  provider: claude
  timeout: 10m
worktree:
  create_cmd: "git worktree add {{.Branch}}"
tracker:
  token: ${FORGE_TOKEN}
`
	path := writeConfig(t, yaml)
	cfg, err := Load(path)
	require.NoError(t, err)

	assert.Equal(t, "myorg/myrepo", cfg.VCS.Repo)
	assert.Equal(t, "tok-123", cfg.Tracker.Token)
}

func TestLoad_MissingRequiredFields(t *testing.T) {
	yaml := `
agent:
  timeout: 10m
`
	path := writeConfig(t, yaml)
	_, err := Load(path)
	require.Error(t, err)

	assert.Contains(t, err.Error(), "vcs.provider")
	assert.Contains(t, err.Error(), "vcs.repo")
	assert.Contains(t, err.Error(), "vcs.base_branch")
	assert.Contains(t, err.Error(), "agent.provider")
	assert.Contains(t, err.Error(), "worktree.create_cmd")
}

func TestLoad_InvalidTimeout(t *testing.T) {
	yaml := `
vcs:
  provider: github
  repo: owner/repo
  base_branch: main
agent:
  provider: claude
  timeout: not-a-duration
worktree:
  create_cmd: "echo hello"
`
	path := writeConfig(t, yaml)
	_, err := Load(path)
	require.Error(t, err)
	assert.Contains(t, err.Error(), "invalid duration")
}

func TestLoad_DefaultTimeout(t *testing.T) {
	yaml := `
vcs:
  provider: github
  repo: owner/repo
  base_branch: main
agent:
  provider: claude
worktree:
  create_cmd: "echo hello"
`
	path := writeConfig(t, yaml)
	cfg, err := Load(path)
	require.NoError(t, err)

	assert.Equal(t, 45*time.Minute, cfg.Agent.Timeout.Duration)
}

func TestLoad_FileNotFound(t *testing.T) {
	_, err := Load("/nonexistent/forge.yaml")
	require.Error(t, err)
	assert.Contains(t, err.Error(), "reading config")
}

func TestLoad_InvalidYAML(t *testing.T) {
	path := writeConfig(t, ":\n\t- :\n  bad:\n\t  indent")
	_, err := Load(path)
	require.Error(t, err)
	assert.Contains(t, err.Error(), "parsing config")
}

func TestLoad_Phase2FieldsParsedNotValidated(t *testing.T) {
	path := writeConfig(t, validYAML)
	cfg, err := Load(path)
	require.NoError(t, err)

	assert.Equal(t, "jira", cfg.Tracker.Provider)
	assert.Equal(t, "PROJ", cfg.Tracker.Project)
	assert.Equal(t, "https://jira.example.com", cfg.Tracker.BaseURL)
	assert.Equal(t, "slack", cfg.Notifier.Provider)
	assert.Equal(t, "https://hooks.slack.com/xxx", cfg.Notifier.WebhookURL)
}
