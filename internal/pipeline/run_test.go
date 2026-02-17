package pipeline

import (
	"context"
	"errors"
	"log/slog"
	"os"
	"path/filepath"
	"testing"
	"time"

	"github.com/shahar-caura/forge/internal/config"
	"github.com/shahar-caura/forge/internal/provider"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// Mock providers for testing.

type mockWorktree struct {
	createPath string
	createErr  error
	removeErr  error
	removeCalled bool
}

func (m *mockWorktree) Create(_ context.Context, _, _ string) (string, error) {
	return m.createPath, m.createErr
}

func (m *mockWorktree) Remove(_ context.Context, _ string) error {
	m.removeCalled = true
	return m.removeErr
}

type mockAgent struct {
	err error
}

func (m *mockAgent) Run(_ context.Context, _, _ string) error {
	return m.err
}

type mockVCS struct {
	commitErr error
	prErr     error
	pr        *provider.PR
}

func (m *mockVCS) CommitAndPush(_ context.Context, _, _, _ string) error {
	return m.commitErr
}

func (m *mockVCS) CreatePR(_ context.Context, _, _, _, _ string) (*provider.PR, error) {
	if m.prErr != nil {
		return nil, m.prErr
	}
	return m.pr, nil
}

func testLogger() *slog.Logger {
	return slog.New(slog.NewTextHandler(os.Stderr, &slog.HandlerOptions{Level: slog.LevelDebug}))
}

func testConfig() *config.Config {
	return &config.Config{
		VCS: config.VCSConfig{
			Provider:   "github",
			Repo:       "owner/repo",
			BaseBranch: "main",
		},
		Agent: config.AgentConfig{
			Provider: "claude",
			Timeout:  config.Duration{Duration: 30 * time.Minute},
		},
	}
}

func writePlan(t *testing.T, content string) string {
	t.Helper()
	dir := t.TempDir()
	path := filepath.Join(dir, "auth.md")
	require.NoError(t, os.WriteFile(path, []byte(content), 0o644))
	return path
}

func TestRun_HappyPath(t *testing.T) {
	wt := &mockWorktree{createPath: "/tmp/wt"}
	ag := &mockAgent{}
	vc := &mockVCS{pr: &provider.PR{URL: "https://github.com/owner/repo/pull/1", Number: 1}}

	planPath := writePlan(t, "implement auth")

	err := Run(context.Background(), testConfig(), Providers{
		VCS: vc, Agent: ag, Worktree: wt,
	}, planPath, testLogger())

	require.NoError(t, err)
	assert.True(t, wt.removeCalled, "cleanup should be called")
}

func TestRun_PlanNotFound(t *testing.T) {
	wt := &mockWorktree{createPath: "/tmp/wt"}
	ag := &mockAgent{}
	vc := &mockVCS{pr: &provider.PR{URL: "url", Number: 1}}

	err := Run(context.Background(), testConfig(), Providers{
		VCS: vc, Agent: ag, Worktree: wt,
	}, "/nonexistent/plan.md", testLogger())

	require.Error(t, err)
	assert.Contains(t, err.Error(), "step 1")
	assert.False(t, wt.removeCalled)
}

func TestRun_WorktreeCreateFails(t *testing.T) {
	wt := &mockWorktree{createErr: errors.New("worktree failed")}
	ag := &mockAgent{}
	vc := &mockVCS{}

	planPath := writePlan(t, "plan")

	err := Run(context.Background(), testConfig(), Providers{
		VCS: vc, Agent: ag, Worktree: wt,
	}, planPath, testLogger())

	require.Error(t, err)
	assert.Contains(t, err.Error(), "step 3")
	assert.False(t, wt.removeCalled, "no cleanup if create failed")
}

func TestRun_AgentFails(t *testing.T) {
	wt := &mockWorktree{createPath: "/tmp/wt"}
	ag := &mockAgent{err: errors.New("agent crashed")}
	vc := &mockVCS{}

	planPath := writePlan(t, "plan")

	err := Run(context.Background(), testConfig(), Providers{
		VCS: vc, Agent: ag, Worktree: wt,
	}, planPath, testLogger())

	require.Error(t, err)
	assert.Contains(t, err.Error(), "step 4")
	assert.True(t, wt.removeCalled, "cleanup should run after agent failure")
}

func TestRun_CommitFails(t *testing.T) {
	wt := &mockWorktree{createPath: "/tmp/wt"}
	ag := &mockAgent{}
	vc := &mockVCS{commitErr: errors.New("nothing to commit")}

	planPath := writePlan(t, "plan")

	err := Run(context.Background(), testConfig(), Providers{
		VCS: vc, Agent: ag, Worktree: wt,
	}, planPath, testLogger())

	require.Error(t, err)
	assert.Contains(t, err.Error(), "step 5")
	assert.True(t, wt.removeCalled)
}

func TestRun_PRCreationFails(t *testing.T) {
	wt := &mockWorktree{createPath: "/tmp/wt"}
	ag := &mockAgent{}
	vc := &mockVCS{prErr: errors.New("gh auth required")}

	planPath := writePlan(t, "plan")

	err := Run(context.Background(), testConfig(), Providers{
		VCS: vc, Agent: ag, Worktree: wt,
	}, planPath, testLogger())

	require.Error(t, err)
	assert.Contains(t, err.Error(), "step 6")
	assert.True(t, wt.removeCalled)
}

func TestBranchName(t *testing.T) {
	tests := []struct {
		input string
		want  string
	}{
		{"plans/auth.md", "forge/auth"},
		{"plans/My Cool Feature.md", "forge/my-cool-feature"},
		{"plans/hello_world.txt", "forge/helloworld"},
		{"plans/UPPER-case.md", "forge/upper-case"},
		{"plans/special!@#chars.md", "forge/specialchars"},
		{"plans/.md", "forge/unnamed"},
	}

	for _, tt := range tests {
		t.Run(tt.input, func(t *testing.T) {
			assert.Equal(t, tt.want, branchName(tt.input))
		})
	}
}
