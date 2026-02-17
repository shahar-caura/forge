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
	"github.com/shahar-caura/forge/internal/state"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// Mock providers for testing.

type mockWorktree struct {
	createPath   string
	createErr    error
	removeErr    error
	removeCalled bool
	createCalled bool
}

func (m *mockWorktree) Create(_ context.Context, _, _ string) (string, error) {
	m.createCalled = true
	return m.createPath, m.createErr
}

func (m *mockWorktree) Remove(_ context.Context, _ string) error {
	m.removeCalled = true
	return m.removeErr
}

type mockAgent struct {
	err    error
	called bool
}

func (m *mockAgent) Run(_ context.Context, _, _ string) error {
	m.called = true
	return m.err
}

type mockVCS struct {
	commitErr    error
	prErr        error
	pr           *provider.PR
	commitCalled bool
}

func (m *mockVCS) CommitAndPush(_ context.Context, _, _, _ string) error {
	m.commitCalled = true
	return m.commitErr
}

func (m *mockVCS) CreatePR(_ context.Context, _, _, _, _ string) (*provider.PR, error) {
	if m.prErr != nil {
		return nil, m.prErr
	}
	return m.pr, nil
}

type mockTracker struct {
	issue *provider.Issue
	err   error
}

func (m *mockTracker) CreateIssue(_ context.Context, _, _ string) (*provider.Issue, error) {
	if m.err != nil {
		return nil, m.err
	}
	return m.issue, nil
}

type mockNotifier struct {
	err      error
	messages []string
}

func (m *mockNotifier) Notify(_ context.Context, message string) error {
	m.messages = append(m.messages, message)
	return m.err
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

func newRunState(planPath string) *state.RunState {
	return state.New("20260217-120000-test", planPath)
}

func defaultProviders(wt *mockWorktree, ag *mockAgent, vc *mockVCS) Providers {
	return Providers{VCS: vc, Agent: ag, Worktree: wt}
}

// --- Existing tests (step numbers shifted +1 for steps after "read plan") ---

func TestRun_HappyPath(t *testing.T) {
	wt := &mockWorktree{createPath: "/tmp/wt"}
	ag := &mockAgent{}
	vc := &mockVCS{pr: &provider.PR{URL: "https://github.com/owner/repo/pull/1", Number: 1}}

	planPath := writePlan(t, "implement auth")
	rs := newRunState(planPath)

	err := Run(context.Background(), testConfig(), defaultProviders(wt, ag, vc), planPath, rs, testLogger())

	require.NoError(t, err)
	assert.True(t, wt.removeCalled, "cleanup should be called on success")
}

func TestRun_PlanNotFound(t *testing.T) {
	wt := &mockWorktree{createPath: "/tmp/wt"}
	ag := &mockAgent{}
	vc := &mockVCS{pr: &provider.PR{URL: "url", Number: 1}}

	planPath := "/nonexistent/plan.md"
	rs := newRunState(planPath)

	err := Run(context.Background(), testConfig(), defaultProviders(wt, ag, vc), planPath, rs, testLogger())

	require.Error(t, err)
	assert.Contains(t, err.Error(), "step 1")
	assert.False(t, wt.removeCalled)
}

func TestRun_WorktreeCreateFails(t *testing.T) {
	wt := &mockWorktree{createErr: errors.New("worktree failed")}
	ag := &mockAgent{}
	vc := &mockVCS{}

	planPath := writePlan(t, "plan")
	rs := newRunState(planPath)

	err := Run(context.Background(), testConfig(), defaultProviders(wt, ag, vc), planPath, rs, testLogger())

	require.Error(t, err)
	assert.Contains(t, err.Error(), "step 4")
	assert.False(t, wt.removeCalled, "no cleanup if create failed")
}

func TestRun_AgentFails(t *testing.T) {
	wt := &mockWorktree{createPath: "/tmp/wt"}
	ag := &mockAgent{err: errors.New("agent crashed")}
	vc := &mockVCS{}

	planPath := writePlan(t, "plan")
	rs := newRunState(planPath)

	err := Run(context.Background(), testConfig(), defaultProviders(wt, ag, vc), planPath, rs, testLogger())

	require.Error(t, err)
	assert.Contains(t, err.Error(), "step 5")
}

func TestRun_CommitFails(t *testing.T) {
	wt := &mockWorktree{createPath: "/tmp/wt"}
	ag := &mockAgent{}
	vc := &mockVCS{commitErr: errors.New("nothing to commit")}

	planPath := writePlan(t, "plan")
	rs := newRunState(planPath)

	err := Run(context.Background(), testConfig(), defaultProviders(wt, ag, vc), planPath, rs, testLogger())

	require.Error(t, err)
	assert.Contains(t, err.Error(), "step 6")
}

func TestRun_PRCreationFails(t *testing.T) {
	wt := &mockWorktree{createPath: "/tmp/wt"}
	ag := &mockAgent{}
	vc := &mockVCS{prErr: errors.New("gh auth required")}

	planPath := writePlan(t, "plan")
	rs := newRunState(planPath)

	err := Run(context.Background(), testConfig(), defaultProviders(wt, ag, vc), planPath, rs, testLogger())

	require.Error(t, err)
	assert.Contains(t, err.Error(), "step 7")
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
			assert.Equal(t, tt.want, BranchName(tt.input))
		})
	}
}

// --- Phase 1.5 tests (step indices shifted +1) ---

func TestRun_StateTrackingHappyPath(t *testing.T) {
	wt := &mockWorktree{createPath: "/tmp/wt"}
	ag := &mockAgent{}
	vc := &mockVCS{pr: &provider.PR{URL: "https://github.com/owner/repo/pull/42", Number: 42}}

	planPath := writePlan(t, "implement auth")
	rs := newRunState(planPath)

	err := Run(context.Background(), testConfig(), defaultProviders(wt, ag, vc), planPath, rs, testLogger())

	require.NoError(t, err)
	assert.Equal(t, state.RunCompleted, rs.Status)
	for _, step := range rs.Steps {
		assert.Equal(t, state.StepCompleted, step.Status, "step %q should be completed", step.Name)
	}
}

func TestRun_ResumeSkipsCompletedSteps(t *testing.T) {
	wt := &mockWorktree{createPath: "/tmp/wt"}
	ag := &mockAgent{}
	vc := &mockVCS{pr: &provider.PR{URL: "https://github.com/owner/repo/pull/1", Number: 1}}

	planPath := writePlan(t, "implement auth")
	rs := newRunState(planPath)

	// Simulate steps 0-3 already completed (read plan, create issue, generate branch, create worktree).
	rs.Steps[0].Status = state.StepCompleted
	rs.Steps[1].Status = state.StepCompleted
	rs.Steps[2].Status = state.StepCompleted
	rs.Steps[3].Status = state.StepCompleted
	rs.Branch = "forge/auth"
	rs.WorktreePath = t.TempDir() // use a real dir so os.Stat passes

	err := Run(context.Background(), testConfig(), defaultProviders(wt, ag, vc), planPath, rs, testLogger())

	require.NoError(t, err)
	assert.False(t, wt.createCalled, "worktree.Create should NOT be called on resume")
	assert.True(t, ag.called, "agent should still be called")
}

func TestRun_ResumeAfterAgentFailure(t *testing.T) {
	wt := &mockWorktree{createPath: "/tmp/wt"}
	ag := &mockAgent{}
	vc := &mockVCS{pr: &provider.PR{URL: "https://github.com/owner/repo/pull/1", Number: 1}}

	planPath := writePlan(t, "implement auth")
	rs := newRunState(planPath)

	// Simulate steps 0-3 completed, step 4 (agent) failed.
	rs.Steps[0].Status = state.StepCompleted
	rs.Steps[1].Status = state.StepCompleted
	rs.Steps[2].Status = state.StepCompleted
	rs.Steps[3].Status = state.StepCompleted
	rs.Steps[4].Status = state.StepFailed
	rs.Steps[4].Error = "agent crashed"
	rs.Branch = "forge/auth"
	rs.WorktreePath = t.TempDir()
	rs.Status = state.RunActive // reset to active for resume

	err := Run(context.Background(), testConfig(), defaultProviders(wt, ag, vc), planPath, rs, testLogger())

	require.NoError(t, err)
	assert.True(t, ag.called, "agent should be re-run on resume")
	assert.Equal(t, state.StepCompleted, rs.Steps[4].Status)
	assert.Equal(t, state.RunCompleted, rs.Status)
}

func TestRun_WorktreePreservedOnFailure(t *testing.T) {
	wt := &mockWorktree{createPath: "/tmp/wt"}
	ag := &mockAgent{err: errors.New("agent crashed")}
	vc := &mockVCS{}

	planPath := writePlan(t, "plan")
	rs := newRunState(planPath)

	err := Run(context.Background(), testConfig(), defaultProviders(wt, ag, vc), planPath, rs, testLogger())

	require.Error(t, err)
	assert.False(t, wt.removeCalled, "worktree should be preserved on failure for resume")
	assert.Equal(t, state.RunFailed, rs.Status)
}

func TestRun_WorktreeCleanedOnSuccess(t *testing.T) {
	wt := &mockWorktree{createPath: "/tmp/wt"}
	ag := &mockAgent{}
	vc := &mockVCS{pr: &provider.PR{URL: "https://github.com/owner/repo/pull/1", Number: 1}}

	planPath := writePlan(t, "plan")
	rs := newRunState(planPath)

	err := Run(context.Background(), testConfig(), defaultProviders(wt, ag, vc), planPath, rs, testLogger())

	require.NoError(t, err)
	assert.True(t, wt.removeCalled, "worktree should be cleaned on success")
}

func TestRun_ResumeWithMissingWorktree(t *testing.T) {
	wt := &mockWorktree{}
	ag := &mockAgent{}
	vc := &mockVCS{}

	planPath := writePlan(t, "plan")
	rs := newRunState(planPath)

	// Simulate steps 0-3 completed but worktree dir was deleted.
	rs.Steps[0].Status = state.StepCompleted
	rs.Steps[1].Status = state.StepCompleted
	rs.Steps[2].Status = state.StepCompleted
	rs.Steps[3].Status = state.StepCompleted
	rs.Branch = "forge/auth"
	rs.WorktreePath = "/nonexistent/worktree/path"

	err := Run(context.Background(), testConfig(), defaultProviders(wt, ag, vc), planPath, rs, testLogger())

	require.Error(t, err)
	assert.Contains(t, err.Error(), "no longer exists")
}

func TestRun_ArtifactsStoredInState(t *testing.T) {
	wt := &mockWorktree{createPath: "/tmp/wt-artifacts"}
	ag := &mockAgent{}
	vc := &mockVCS{pr: &provider.PR{URL: "https://github.com/owner/repo/pull/99", Number: 99}}

	planPath := writePlan(t, "implement auth")
	rs := newRunState(planPath)

	err := Run(context.Background(), testConfig(), defaultProviders(wt, ag, vc), planPath, rs, testLogger())

	require.NoError(t, err)
	assert.Equal(t, "forge/auth", rs.Branch)
	assert.Equal(t, "/tmp/wt-artifacts", rs.WorktreePath)
	assert.Equal(t, "https://github.com/owner/repo/pull/99", rs.PRUrl)
	assert.Equal(t, 99, rs.PRNumber)
}

// --- Phase 2 tests: Tracker ---

func TestRun_TrackerNil_SkipsIssueCreation(t *testing.T) {
	wt := &mockWorktree{createPath: "/tmp/wt"}
	ag := &mockAgent{}
	vc := &mockVCS{pr: &provider.PR{URL: "https://github.com/owner/repo/pull/1", Number: 1}}

	planPath := writePlan(t, "plan")
	rs := newRunState(planPath)

	err := Run(context.Background(), testConfig(), Providers{
		VCS: vc, Agent: ag, Worktree: wt, Tracker: nil,
	}, planPath, rs, testLogger())

	require.NoError(t, err)
	assert.Empty(t, rs.IssueKey)
	assert.Empty(t, rs.IssueURL)
}

func TestRun_TrackerCreatesIssue(t *testing.T) {
	wt := &mockWorktree{createPath: "/tmp/wt"}
	ag := &mockAgent{}
	vc := &mockVCS{pr: &provider.PR{URL: "https://github.com/owner/repo/pull/1", Number: 1}}
	tr := &mockTracker{issue: &provider.Issue{Key: "PROJ-42", URL: "https://jira.example.com/browse/PROJ-42"}}

	planPath := writePlan(t, "plan")
	rs := newRunState(planPath)

	err := Run(context.Background(), testConfig(), Providers{
		VCS: vc, Agent: ag, Worktree: wt, Tracker: tr,
	}, planPath, rs, testLogger())

	require.NoError(t, err)
	assert.Equal(t, "PROJ-42", rs.IssueKey)
	assert.Equal(t, "https://jira.example.com/browse/PROJ-42", rs.IssueURL)
}

func TestRun_TrackerFails_PipelineFails(t *testing.T) {
	wt := &mockWorktree{createPath: "/tmp/wt"}
	ag := &mockAgent{}
	vc := &mockVCS{}
	tr := &mockTracker{err: errors.New("jira auth failed")}

	planPath := writePlan(t, "plan")
	rs := newRunState(planPath)

	err := Run(context.Background(), testConfig(), Providers{
		VCS: vc, Agent: ag, Worktree: wt, Tracker: tr,
	}, planPath, rs, testLogger())

	require.Error(t, err)
	assert.Contains(t, err.Error(), "step 2")
	assert.Contains(t, err.Error(), "create issue")
	assert.False(t, ag.called, "agent should not run if tracker fails")
}

// --- Phase 2 tests: Notifier ---

func TestRun_NotifierNil_SkipsNotification(t *testing.T) {
	wt := &mockWorktree{createPath: "/tmp/wt"}
	ag := &mockAgent{}
	vc := &mockVCS{pr: &provider.PR{URL: "https://github.com/owner/repo/pull/1", Number: 1}}

	planPath := writePlan(t, "plan")
	rs := newRunState(planPath)

	err := Run(context.Background(), testConfig(), Providers{
		VCS: vc, Agent: ag, Worktree: wt, Notifier: nil,
	}, planPath, rs, testLogger())

	require.NoError(t, err)
}

func TestRun_NotifierCalled_OnSuccess(t *testing.T) {
	wt := &mockWorktree{createPath: "/tmp/wt"}
	ag := &mockAgent{}
	vc := &mockVCS{pr: &provider.PR{URL: "https://github.com/owner/repo/pull/1", Number: 1}}
	n := &mockNotifier{}

	planPath := writePlan(t, "plan")
	rs := newRunState(planPath)

	err := Run(context.Background(), testConfig(), Providers{
		VCS: vc, Agent: ag, Worktree: wt, Notifier: n,
	}, planPath, rs, testLogger())

	require.NoError(t, err)
	require.Len(t, n.messages, 1)
	assert.Contains(t, n.messages[0], "PR ready for review")
	assert.Contains(t, n.messages[0], "https://github.com/owner/repo/pull/1")
}

func TestRun_NotifierCalled_OnSuccess_WithIssue(t *testing.T) {
	wt := &mockWorktree{createPath: "/tmp/wt"}
	ag := &mockAgent{}
	vc := &mockVCS{pr: &provider.PR{URL: "https://github.com/owner/repo/pull/1", Number: 1}}
	tr := &mockTracker{issue: &provider.Issue{Key: "PROJ-42", URL: "https://jira.example.com/browse/PROJ-42"}}
	n := &mockNotifier{}

	planPath := writePlan(t, "plan")
	rs := newRunState(planPath)

	err := Run(context.Background(), testConfig(), Providers{
		VCS: vc, Agent: ag, Worktree: wt, Tracker: tr, Notifier: n,
	}, planPath, rs, testLogger())

	require.NoError(t, err)
	require.Len(t, n.messages, 1)
	assert.Contains(t, n.messages[0], "PR ready for review")
	assert.Contains(t, n.messages[0], "PROJ-42")
}

func TestRun_NotifierCalled_OnFailure(t *testing.T) {
	wt := &mockWorktree{createPath: "/tmp/wt"}
	ag := &mockAgent{err: errors.New("agent crashed")}
	vc := &mockVCS{}
	n := &mockNotifier{}

	planPath := writePlan(t, "plan")
	rs := newRunState(planPath)

	err := Run(context.Background(), testConfig(), Providers{
		VCS: vc, Agent: ag, Worktree: wt, Notifier: n,
	}, planPath, rs, testLogger())

	require.Error(t, err)
	// Best-effort failure notification.
	require.Len(t, n.messages, 1)
	assert.Contains(t, n.messages[0], "forge pipeline failed")
	assert.Contains(t, n.messages[0], "agent crashed")
}

func TestRun_NotifierFailure_FailsPipeline(t *testing.T) {
	wt := &mockWorktree{createPath: "/tmp/wt"}
	ag := &mockAgent{}
	vc := &mockVCS{pr: &provider.PR{URL: "https://github.com/owner/repo/pull/1", Number: 1}}
	n := &mockNotifier{err: errors.New("webhook failed")}

	planPath := writePlan(t, "plan")
	rs := newRunState(planPath)

	err := Run(context.Background(), testConfig(), Providers{
		VCS: vc, Agent: ag, Worktree: wt, Notifier: n,
	}, planPath, rs, testLogger())

	require.Error(t, err)
	assert.Contains(t, err.Error(), "step 8")
	assert.Contains(t, err.Error(), "notify")
}
