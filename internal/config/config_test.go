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
  board_id: "42"
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

func TestLoad_TrackerAndNotifierFieldsParsed(t *testing.T) {
	path := writeConfig(t, validYAML)
	cfg, err := Load(path)
	require.NoError(t, err)

	assert.Equal(t, "jira", cfg.Tracker.Provider)
	assert.Equal(t, "PROJ", cfg.Tracker.Project)
	assert.Equal(t, "https://jira.example.com", cfg.Tracker.BaseURL)
	assert.Equal(t, "user@example.com", cfg.Tracker.Email)
	assert.Equal(t, "secret", cfg.Tracker.Token)
	assert.Equal(t, "slack", cfg.Notifier.Provider)
	assert.Equal(t, "https://hooks.slack.com/xxx", cfg.Notifier.WebhookURL)
}

func TestLoad_TrackerProviderSetMissingFields(t *testing.T) {
	yaml := `
vcs:
  provider: github
  repo: owner/repo
  base_branch: main
agent:
  provider: claude
worktree:
  create_cmd: "echo hello"
tracker:
  provider: jira
`
	path := writeConfig(t, yaml)
	_, err := Load(path)
	require.Error(t, err)

	assert.Contains(t, err.Error(), "tracker.project")
	assert.Contains(t, err.Error(), "tracker.base_url")
	assert.Contains(t, err.Error(), "tracker.email")
	assert.Contains(t, err.Error(), "tracker.token")
}

func TestLoad_NotifierProviderSetMissingWebhook(t *testing.T) {
	yaml := `
vcs:
  provider: github
  repo: owner/repo
  base_branch: main
agent:
  provider: claude
worktree:
  create_cmd: "echo hello"
notifier:
  provider: slack
`
	path := writeConfig(t, yaml)
	_, err := Load(path)
	require.Error(t, err)

	assert.Contains(t, err.Error(), "notifier.webhook_url")
}

func TestLoad_BoardIDParsed(t *testing.T) {
	path := writeConfig(t, validYAML)
	cfg, err := Load(path)
	require.NoError(t, err)

	assert.Equal(t, "42", cfg.Tracker.BoardID)
}

func TestLoad_BoardIDOptional(t *testing.T) {
	yaml := `
vcs:
  provider: github
  repo: owner/repo
  base_branch: main
agent:
  provider: claude
worktree:
  create_cmd: "echo hello"
tracker:
  provider: jira
  project: PROJ
  base_url: https://jira.example.com
  email: user@example.com
  token: secret
`
	path := writeConfig(t, yaml)
	cfg, err := Load(path)
	require.NoError(t, err)

	assert.Empty(t, cfg.Tracker.BoardID)
}

func TestLoad_UnconfiguredTrackerNotifierNoValidationErrors(t *testing.T) {
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

	assert.Empty(t, cfg.Tracker.Provider)
	assert.Empty(t, cfg.Notifier.Provider)
}

// --- CR Config tests ---

func TestLoad_CRConfigParsed(t *testing.T) {
	yaml := `
vcs:
  provider: github
  repo: owner/repo
  base_branch: main
agent:
  provider: claude
worktree:
  create_cmd: "echo hello"
cr:
  enabled: true
  poll_timeout: 10m
  poll_interval: 30s
  comment_pattern: "Claude finished"
  fix_strategy: amend
`
	path := writeConfig(t, yaml)
	cfg, err := Load(path)
	require.NoError(t, err)

	assert.True(t, cfg.CR.Enabled)
	assert.Equal(t, 10*time.Minute, cfg.CR.PollTimeout.Duration)
	assert.Equal(t, 30*time.Second, cfg.CR.PollInterval.Duration)
	assert.Equal(t, "Claude finished", cfg.CR.CommentPattern)
	assert.Equal(t, "amend", cfg.CR.FixStrategy)
}

func TestLoad_CRConfigDefaults(t *testing.T) {
	yaml := `
vcs:
  provider: github
  repo: owner/repo
  base_branch: main
agent:
  provider: claude
worktree:
  create_cmd: "echo hello"
cr:
  enabled: true
  comment_pattern: "review done"
`
	path := writeConfig(t, yaml)
	cfg, err := Load(path)
	require.NoError(t, err)

	assert.Equal(t, 5*time.Minute, cfg.CR.PollTimeout.Duration)
	assert.Equal(t, 15*time.Second, cfg.CR.PollInterval.Duration)
	assert.Equal(t, "amend", cfg.CR.FixStrategy)
}

func TestLoad_CRConfigDisabled_NoValidation(t *testing.T) {
	yaml := `
vcs:
  provider: github
  repo: owner/repo
  base_branch: main
agent:
  provider: claude
worktree:
  create_cmd: "echo hello"
cr:
  enabled: false
`
	path := writeConfig(t, yaml)
	_, err := Load(path)
	require.NoError(t, err, "disabled CR should not validate fields")
}

func TestLoad_CRConfigEnabled_MissingPattern(t *testing.T) {
	yaml := `
vcs:
  provider: github
  repo: owner/repo
  base_branch: main
agent:
  provider: claude
worktree:
  create_cmd: "echo hello"
cr:
  enabled: true
`
	path := writeConfig(t, yaml)
	_, err := Load(path)
	require.Error(t, err)
	assert.Contains(t, err.Error(), "cr.comment_pattern")
}

func TestLoad_CRConfigEnabled_InvalidStrategy(t *testing.T) {
	yaml := `
vcs:
  provider: github
  repo: owner/repo
  base_branch: main
agent:
  provider: claude
worktree:
  create_cmd: "echo hello"
cr:
  enabled: true
  comment_pattern: "done"
  fix_strategy: squash
`
	path := writeConfig(t, yaml)
	_, err := Load(path)
	require.Error(t, err)
	assert.Contains(t, err.Error(), "cr.fix_strategy")
}

func TestLoad_CRConfigNewCommitStrategy(t *testing.T) {
	yaml := `
vcs:
  provider: github
  repo: owner/repo
  base_branch: main
agent:
  provider: claude
worktree:
  create_cmd: "echo hello"
cr:
  enabled: true
  comment_pattern: "review done"
  fix_strategy: new-commit
`
	path := writeConfig(t, yaml)
	cfg, err := Load(path)
	require.NoError(t, err)
	assert.Equal(t, "new-commit", cfg.CR.FixStrategy)
}

// --- CR Mode & MaxRounds tests ---

func TestLoad_CRMode_DefaultsToPoll(t *testing.T) {
	yaml := `
vcs:
  provider: github
  repo: owner/repo
  base_branch: main
agent:
  provider: claude
worktree:
  create_cmd: "echo hello"
cr:
  enabled: true
  comment_pattern: "review done"
`
	path := writeConfig(t, yaml)
	cfg, err := Load(path)
	require.NoError(t, err)
	assert.Equal(t, "poll", cfg.CR.Mode)
}

func TestLoad_CRMaxRounds_DefaultsTo2(t *testing.T) {
	yaml := `
vcs:
  provider: github
  repo: owner/repo
  base_branch: main
agent:
  provider: claude
worktree:
  create_cmd: "echo hello"
cr:
  enabled: true
  comment_pattern: "review done"
`
	path := writeConfig(t, yaml)
	cfg, err := Load(path)
	require.NoError(t, err)
	assert.Equal(t, 2, cfg.CR.MaxRounds)
}

func TestLoad_CRMode_InvalidValue(t *testing.T) {
	yaml := `
vcs:
  provider: github
  repo: owner/repo
  base_branch: main
agent:
  provider: claude
worktree:
  create_cmd: "echo hello"
cr:
  enabled: true
  mode: hybrid
  comment_pattern: "done"
`
	path := writeConfig(t, yaml)
	_, err := Load(path)
	require.Error(t, err)
	assert.Contains(t, err.Error(), "cr.mode")
}

func TestLoad_CRMaxRounds_Invalid(t *testing.T) {
	yaml := `
vcs:
  provider: github
  repo: owner/repo
  base_branch: main
agent:
  provider: claude
worktree:
  create_cmd: "echo hello"
cr:
  enabled: true
  mode: local
  max_rounds: -1
`
	path := writeConfig(t, yaml)
	_, err := Load(path)
	require.Error(t, err)
	assert.Contains(t, err.Error(), "cr.max_rounds")
}

func TestLoad_CRLocalMode_CommentPatternNotRequired(t *testing.T) {
	yaml := `
vcs:
  provider: github
  repo: owner/repo
  base_branch: main
agent:
  provider: claude
worktree:
  create_cmd: "echo hello"
cr:
  enabled: true
  mode: local
`
	path := writeConfig(t, yaml)
	cfg, err := Load(path)
	require.NoError(t, err)
	assert.Equal(t, "local", cfg.CR.Mode)
	assert.Empty(t, cfg.CR.CommentPattern)
}

// --- Agent providers list tests ---

func TestLoad_AgentProviders_Parsed(t *testing.T) {
	yaml := `
vcs:
  provider: github
  repo: owner/repo
  base_branch: main
agent:
  provider: claude
  providers: [claude, gemini, codex]
worktree:
  create_cmd: "echo hello"
`
	path := writeConfig(t, yaml)
	cfg, err := Load(path)
	require.NoError(t, err)
	assert.Equal(t, []string{"claude", "gemini", "codex"}, cfg.Agent.Providers)
}

func TestLoad_AgentProviders_Empty(t *testing.T) {
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
	assert.Empty(t, cfg.Agent.Providers)
}

func TestLoad_AgentProviders_UnrecognizedAgent(t *testing.T) {
	yaml := `
vcs:
  provider: github
  repo: owner/repo
  base_branch: main
agent:
  provider: claude
  providers: [claude, unknown]
worktree:
  create_cmd: "echo hello"
`
	path := writeConfig(t, yaml)
	_, err := Load(path)
	require.Error(t, err)
	assert.Contains(t, err.Error(), "unrecognized agent")
	assert.Contains(t, err.Error(), "unknown")
}

func TestLoad_CRPollMode_CommentPatternRequired(t *testing.T) {
	yaml := `
vcs:
  provider: github
  repo: owner/repo
  base_branch: main
agent:
  provider: claude
worktree:
  create_cmd: "echo hello"
cr:
  enabled: true
  mode: poll
`
	path := writeConfig(t, yaml)
	_, err := Load(path)
	require.Error(t, err)
	assert.Contains(t, err.Error(), "cr.comment_pattern")
}
