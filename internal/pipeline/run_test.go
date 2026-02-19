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
	err       error
	called    bool
	prompts   []string
	callCount int
	output    string
	outputs   []string // per-call outputs; takes precedence over output when set
}

func (m *mockAgent) Run(_ context.Context, _, prompt string) (string, error) {
	m.called = true
	idx := m.callCount
	m.callCount++
	m.prompts = append(m.prompts, prompt)
	out := m.output
	if idx < len(m.outputs) {
		out = m.outputs[idx]
	}
	return out, m.err
}

type mockVCS struct {
	commitErr         error
	prErr             error
	pr                *provider.PR
	prBody            string // captured from CreatePR
	commitCalled      bool
	pushCalled        bool
	pushErr           error
	comments          []provider.Comment
	getCommentsErr    error
	getCommentsCalled bool
	postCommentCalled bool
	postCommentBody   string
	postCommentErr    error
	amendCalled       bool
	amendErr          error
	noChanges         bool // when true, HasChanges returns false
}

func (m *mockVCS) CommitAndPush(_ context.Context, _, _, _ string) error {
	m.commitCalled = true
	return m.commitErr
}

func (m *mockVCS) Push(_ context.Context, _, _ string) error {
	m.pushCalled = true
	return m.pushErr
}

func (m *mockVCS) CreatePR(_ context.Context, _, _, _, body string) (*provider.PR, error) {
	m.prBody = body
	if m.prErr != nil {
		return nil, m.prErr
	}
	return m.pr, nil
}

func (m *mockVCS) GetPRComments(_ context.Context, _ int) ([]provider.Comment, error) {
	m.getCommentsCalled = true
	return m.comments, m.getCommentsErr
}

func (m *mockVCS) PostPRComment(_ context.Context, _ int, body string) error {
	m.postCommentCalled = true
	m.postCommentBody = body
	return m.postCommentErr
}

func (m *mockVCS) AmendAndForcePush(_ context.Context, _, _ string) error {
	m.amendCalled = true
	return m.amendErr
}

func (m *mockVCS) AmendAndForcePushMsg(_ context.Context, _, _, _ string) error {
	m.amendCalled = true
	return m.amendErr
}

func (m *mockVCS) HasChanges(_ context.Context, _ string) (bool, error) {
	return !m.noChanges, nil
}

func (m *mockVCS) GetIssue(_ context.Context, _ int) (*provider.GitHubIssue, error) {
	return nil, nil
}

func (m *mockVCS) ListIssues(_ context.Context, _ string, _ string) ([]provider.GitHubIssue, error) {
	return nil, nil
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

func testConfigWithCR() *config.Config {
	cfg := testConfig()
	cfg.CR = config.CRConfig{
		Enabled:        true,
		PollTimeout:    config.Duration{Duration: 100 * time.Millisecond},
		PollInterval:   config.Duration{Duration: 10 * time.Millisecond},
		CommentPattern: "Claude finished",
		FixStrategy:    "amend",
	}
	return cfg
}

func writePlan(t *testing.T, content string) string {
	t.Helper()
	dir := t.TempDir()
	path := filepath.Join(dir, "auth.md")
	require.NoError(t, os.WriteFile(path, []byte(content), 0o644))
	return path
}

func writePlanNamed(t *testing.T, name, content string) string {
	t.Helper()
	dir := t.TempDir()
	path := filepath.Join(dir, name)
	require.NoError(t, os.WriteFile(path, []byte(content), 0o644))
	return path
}

func newRunState(planPath string) *state.RunState {
	return state.New("20260217-120000-test", planPath)
}

func defaultProviders(wt *mockWorktree, ag *mockAgent, vc *mockVCS) Providers {
	return Providers{VCS: vc, Agent: ag, Worktree: wt}
}

// --- Core pipeline tests (step numbers: 1-based in errors) ---

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

func TestRun_AgentNoChanges(t *testing.T) {
	wt := &mockWorktree{createPath: "/tmp/wt"}
	ag := &mockAgent{}
	vc := &mockVCS{noChanges: true}

	planPath := writePlan(t, "plan")
	rs := newRunState(planPath)

	err := Run(context.Background(), testConfig(), defaultProviders(wt, ag, vc), planPath, rs, testLogger())

	require.Error(t, err)
	assert.Contains(t, err.Error(), "agent produced no file changes")
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

// --- Branch naming tests ---

func TestSlugFromTitle(t *testing.T) {
	tests := []struct {
		input string
		want  string
	}{
		{"Deploy Server", "deploy-server"},
		{"My Cool Feature", "my-cool-feature"},
		{"UPPER-case", "upper-case"},
		{"special!@#chars", "special-chars"},
		{"", "unnamed"},
		{"hello_world", "hello-world"},
	}
	for _, tt := range tests {
		t.Run(tt.input, func(t *testing.T) {
			assert.Equal(t, tt.want, SlugFromTitle(tt.input))
		})
	}
}

func TestTitleFromFilename(t *testing.T) {
	tests := []struct {
		input string
		want  string
	}{
		{"hello-world", "Hello World"},
		{"deploy_server", "Deploy Server"},
		{"auth", "Auth"},
		{"my-cool--feature", "My Cool Feature"},
		{"", ""},
	}
	for _, tt := range tests {
		t.Run(tt.input, func(t *testing.T) {
			assert.Equal(t, tt.want, TitleFromFilename(tt.input))
		})
	}
}

func TestBranchName(t *testing.T) {
	tests := []struct {
		issueKey string
		title    string
		want     string
	}{
		{"CAURA-288", "Deploy Server", "CAURA-288-deploy-server"},
		{"PROJ-42", "My Cool Feature", "PROJ-42-my-cool-feature"},
		{"", "Deploy Server", "forge/deploy-server"},
		{"", "", "forge/unnamed"},
	}
	for _, tt := range tests {
		name := tt.issueKey + "/" + tt.title
		t.Run(name, func(t *testing.T) {
			assert.Equal(t, tt.want, BranchName(tt.issueKey, tt.title))
		})
	}
}

func TestValidateBranchName(t *testing.T) {
	assert.NoError(t, ValidateBranchName("CAURA-288-deploy-server"))
	assert.NoError(t, ValidateBranchName("PROJ-42-my-cool-feature"))
	assert.Error(t, ValidateBranchName("forge/deploy-server"))
	assert.Error(t, ValidateBranchName("bad-branch"))
	assert.Error(t, ValidateBranchName("CAURA-288")) // needs at least one slug segment
}

func TestBranchName_NoIssueKey(t *testing.T) {
	branch := BranchName("", "Auth System")
	assert.Equal(t, "forge/auth-system", branch)
	assert.Error(t, ValidateBranchName(branch), "forge/ prefix should not match strict pattern")
}

// --- Frontmatter tests ---

func TestRun_FrontmatterTitle_UsedForPR(t *testing.T) {
	wt := &mockWorktree{createPath: "/tmp/wt"}
	ag := &mockAgent{}
	vc := &mockVCS{pr: &provider.PR{URL: "https://github.com/owner/repo/pull/1", Number: 1}}

	planPath := writePlan(t, "---\ntitle: Hello World\n---\nCreate a hello world server")
	rs := newRunState(planPath)

	err := Run(context.Background(), testConfig(), defaultProviders(wt, ag, vc), planPath, rs, testLogger())

	require.NoError(t, err)
	assert.Equal(t, "Hello World", rs.PlanTitle)
	// Branch should use the title (no issue key → forge/ prefix).
	assert.Equal(t, "forge/hello-world", rs.Branch)
}

func TestRun_FrontmatterTitle_WithTracker(t *testing.T) {
	wt := &mockWorktree{createPath: "/tmp/wt"}
	ag := &mockAgent{}
	vc := &mockVCS{pr: &provider.PR{URL: "https://github.com/owner/repo/pull/1", Number: 1}}
	tr := &mockTracker{issue: &provider.Issue{Key: "PROJ-42", URL: "https://jira.example.com/browse/PROJ-42"}}

	planPath := writePlan(t, "---\ntitle: Deploy Server\n---\nDeploy the server")
	rs := newRunState(planPath)

	err := Run(context.Background(), testConfig(), Providers{
		VCS: vc, Agent: ag, Worktree: wt, Tracker: tr,
	}, planPath, rs, testLogger())

	require.NoError(t, err)
	assert.Equal(t, "PROJ-42-deploy-server", rs.Branch)
}

func TestRun_NoFrontmatter_FallbackToFilename(t *testing.T) {
	wt := &mockWorktree{createPath: "/tmp/wt"}
	ag := &mockAgent{}
	vc := &mockVCS{pr: &provider.PR{URL: "https://github.com/owner/repo/pull/1", Number: 1}}

	planPath := writePlan(t, "implement auth without frontmatter")
	rs := newRunState(planPath)

	err := Run(context.Background(), testConfig(), defaultProviders(wt, ag, vc), planPath, rs, testLogger())

	require.NoError(t, err)
	// Filename is "auth.md" → slug is "auth".
	assert.Equal(t, "forge/auth", rs.Branch)
}

// --- Phase 1.5 tests (state tracking + resume) ---

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

func TestRun_ResumeWithMissingWorktree_ReCreates(t *testing.T) {
	newPath := t.TempDir()
	wt := &mockWorktree{createPath: newPath}
	ag := &mockAgent{}
	vc := &mockVCS{pr: &provider.PR{URL: "https://github.com/owner/repo/pull/1", Number: 1}}

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

	require.NoError(t, err)
	assert.True(t, wt.createCalled, "worktree should be re-created")
	assert.Equal(t, newPath, rs.WorktreePath, "worktree path should be updated")
}

func TestRun_ResumeWithMissingWorktree_ReCreateFails(t *testing.T) {
	wt := &mockWorktree{createErr: errors.New("branch not found")}
	ag := &mockAgent{}
	vc := &mockVCS{}

	planPath := writePlan(t, "plan")
	rs := newRunState(planPath)

	rs.Steps[0].Status = state.StepCompleted
	rs.Steps[1].Status = state.StepCompleted
	rs.Steps[2].Status = state.StepCompleted
	rs.Steps[3].Status = state.StepCompleted
	rs.Branch = "forge/auth"
	rs.WorktreePath = "/nonexistent/worktree/path"

	err := Run(context.Background(), testConfig(), defaultProviders(wt, ag, vc), planPath, rs, testLogger())

	require.Error(t, err)
	assert.Contains(t, err.Error(), "re-creating")
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
	assert.Contains(t, err.Error(), "step 11")
	assert.Contains(t, err.Error(), "notify")
}

// --- CR feedback loop tests ---

func TestRun_CRLoop_HappyPath(t *testing.T) {
	wt := &mockWorktree{createPath: "/tmp/wt"}
	ag := &mockAgent{}
	vc := &mockVCS{
		pr:       &provider.PR{URL: "https://github.com/owner/repo/pull/1", Number: 1},
		comments: []provider.Comment{{ID: "1", Author: "claude-bot", Body: "Claude finished reviewing"}},
	}

	planPath := writePlan(t, "implement auth")
	rs := newRunState(planPath)

	err := Run(context.Background(), testConfigWithCR(), defaultProviders(wt, ag, vc), planPath, rs, testLogger())

	require.NoError(t, err)
	assert.Equal(t, "Claude finished reviewing", rs.CRFeedback)
	assert.True(t, vc.getCommentsCalled, "should poll for comments")
	assert.True(t, vc.amendCalled, "should amend and force push")
	assert.True(t, vc.postCommentCalled, "should post reply comment")
	assert.Equal(t, 2, ag.callCount, "agent should run twice: once for plan, once for fix")
}

func TestRun_CRLoop_Disabled(t *testing.T) {
	wt := &mockWorktree{createPath: "/tmp/wt"}
	ag := &mockAgent{}
	vc := &mockVCS{pr: &provider.PR{URL: "https://github.com/owner/repo/pull/1", Number: 1}}

	planPath := writePlan(t, "implement auth")
	rs := newRunState(planPath)

	// CR disabled by default in testConfig().
	err := Run(context.Background(), testConfig(), defaultProviders(wt, ag, vc), planPath, rs, testLogger())

	require.NoError(t, err)
	assert.False(t, vc.getCommentsCalled, "should not poll when CR disabled")
	assert.False(t, vc.amendCalled, "should not amend when CR disabled")
	assert.Equal(t, 1, ag.callCount, "agent should run only once")
}

func TestRun_CRLoop_PollTimeout(t *testing.T) {
	wt := &mockWorktree{createPath: "/tmp/wt"}
	ag := &mockAgent{}
	vc := &mockVCS{
		pr:       &provider.PR{URL: "https://github.com/owner/repo/pull/1", Number: 1},
		comments: []provider.Comment{}, // no matching comments
	}

	planPath := writePlan(t, "implement auth")
	rs := newRunState(planPath)

	err := Run(context.Background(), testConfigWithCR(), defaultProviders(wt, ag, vc), planPath, rs, testLogger())

	require.Error(t, err)
	assert.Contains(t, err.Error(), "poll timeout")
	assert.Contains(t, err.Error(), "step 8")
}

func TestRun_CRLoop_NewCommitStrategy(t *testing.T) {
	wt := &mockWorktree{createPath: "/tmp/wt"}
	ag := &mockAgent{}
	vc := &mockVCS{
		pr:       &provider.PR{URL: "https://github.com/owner/repo/pull/1", Number: 1},
		comments: []provider.Comment{{ID: "1", Author: "bot", Body: "Claude finished reviewing"}},
	}

	planPath := writePlan(t, "implement auth")
	rs := newRunState(planPath)

	cfg := testConfigWithCR()
	cfg.CR.FixStrategy = "new-commit"

	err := Run(context.Background(), cfg, defaultProviders(wt, ag, vc), planPath, rs, testLogger())

	require.NoError(t, err)
	assert.False(t, vc.amendCalled, "should NOT amend with new-commit strategy")
	assert.True(t, vc.commitCalled, "should use CommitAndPush for new-commit")
}

func TestRun_CRLoop_Resume(t *testing.T) {
	wt := &mockWorktree{createPath: "/tmp/wt"}
	ag := &mockAgent{}
	vc := &mockVCS{
		pr:       &provider.PR{URL: "https://github.com/owner/repo/pull/1", Number: 1},
		comments: []provider.Comment{{ID: "1", Author: "bot", Body: "Claude finished reviewing"}},
	}

	planPath := writePlan(t, "implement auth")
	rs := newRunState(planPath)

	// Simulate steps 0-7 completed (through poll cr), step 8 (fix cr) failed.
	for i := 0; i <= 7; i++ {
		rs.Steps[i].Status = state.StepCompleted
	}
	rs.Steps[8].Status = state.StepFailed
	rs.Steps[8].Error = "agent crashed"
	rs.Branch = "forge/auth"
	rs.WorktreePath = t.TempDir()
	rs.PRUrl = "https://github.com/owner/repo/pull/1"
	rs.PRNumber = 1
	rs.CRFeedback = "Claude finished reviewing"
	rs.Status = state.RunActive

	err := Run(context.Background(), testConfigWithCR(), defaultProviders(wt, ag, vc), planPath, rs, testLogger())

	require.NoError(t, err)
	assert.True(t, ag.called, "agent should run for fix cr step")
	assert.Equal(t, state.RunCompleted, rs.Status)
}

// --- CR summary extraction tests ---

func TestAgentResultText(t *testing.T) {
	tests := []struct {
		name  string
		input string
		want  string
	}{
		{
			name:  "valid JSON with result",
			input: `{"result": "hello world"}`,
			want:  "hello world",
		},
		{
			name:  "empty result field",
			input: `{"result": ""}`,
			want:  `{"result": ""}`,
		},
		{
			name:  "non-JSON input",
			input: "just plain text",
			want:  "just plain text",
		},
		{
			name:  "missing result field",
			input: `{"other": "value"}`,
			want:  `{"other": "value"}`,
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			assert.Equal(t, tt.want, agentResultText(tt.input))
		})
	}
}

func TestExtractCRSummary(t *testing.T) {
	tests := []struct {
		name  string
		input string
		want  string
	}{
		{
			name:  "valid markers in JSON",
			input: `{"result": "some text\n---CRSUMMARY---\nFixed the auth bug.\n---CRSUMMARY---\nmore text"}`,
			want:  "Fixed the auth bug.",
		},
		{
			name:  "no markers",
			input: `{"result": "just fixed stuff"}`,
			want:  "",
		},
		{
			name:  "single marker only",
			input: `{"result": "---CRSUMMARY---\nno closing marker"}`,
			want:  "",
		},
		{
			name:  "raw text with markers",
			input: "before\n---CRSUMMARY---\nSummary here.\n---CRSUMMARY---\nafter",
			want:  "Summary here.",
		},
		{
			name:  "empty between markers",
			input: "---CRSUMMARY---\n\n---CRSUMMARY---",
			want:  "",
		},
		{
			name:  "multiline summary",
			input: "---CRSUMMARY---\n- Fixed auth\n- Updated tests\n---CRSUMMARY---",
			want:  "- Fixed auth\n- Updated tests",
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			assert.Equal(t, tt.want, extractCRSummary(tt.input))
		})
	}
}

func TestRun_CRLoop_IntelligentReplyComment(t *testing.T) {
	wt := &mockWorktree{createPath: "/tmp/wt"}
	ag := &mockAgent{
		outputs: []string{
			"", // step 4: initial agent run
			"I fixed things.\n---CRSUMMARY---\n**Fixed:** Renamed `foo` to `bar` per review.\n---CRSUMMARY---\nDone.", // step 8: fix CR
		},
	}
	vc := &mockVCS{
		pr:       &provider.PR{URL: "https://github.com/owner/repo/pull/1", Number: 1},
		comments: []provider.Comment{{ID: "1", Author: "reviewer", Body: "Claude finished reviewing: rename foo to bar"}},
	}

	planPath := writePlan(t, "implement auth")
	rs := newRunState(planPath)

	err := Run(context.Background(), testConfigWithCR(), defaultProviders(wt, ag, vc), planPath, rs, testLogger())

	require.NoError(t, err)
	assert.True(t, vc.postCommentCalled)
	assert.Equal(t, "**Fixed:** Renamed `foo` to `bar` per review.", vc.postCommentBody)
	assert.Equal(t, "**Fixed:** Renamed `foo` to `bar` per review.", rs.CRFixSummary)
}

// --- Source issue auto-close tests ---

func TestRun_SourceIssue_PRBodyContainsCloses(t *testing.T) {
	wt := &mockWorktree{createPath: "/tmp/wt"}
	ag := &mockAgent{}
	vc := &mockVCS{pr: &provider.PR{URL: "https://github.com/owner/repo/pull/1", Number: 1}}

	planPath := writePlan(t, "implement auth")
	rs := newRunState(planPath)
	rs.SourceIssue = 10

	err := Run(context.Background(), testConfig(), defaultProviders(wt, ag, vc), planPath, rs, testLogger())

	require.NoError(t, err)
	assert.Contains(t, vc.prBody, "Closes #10")
	assert.Contains(t, vc.prBody, "implement auth")
}

func TestRun_NoSourceIssue_PRBodyUnchanged(t *testing.T) {
	wt := &mockWorktree{createPath: "/tmp/wt"}
	ag := &mockAgent{}
	vc := &mockVCS{pr: &provider.PR{URL: "https://github.com/owner/repo/pull/1", Number: 1}}

	planPath := writePlan(t, "implement auth")
	rs := newRunState(planPath)

	err := Run(context.Background(), testConfig(), defaultProviders(wt, ag, vc), planPath, rs, testLogger())

	require.NoError(t, err)
	assert.Equal(t, "implement auth", vc.prBody)
	assert.NotContains(t, vc.prBody, "Closes")
}

func TestRun_CRLoop_FallbackComment(t *testing.T) {
	wt := &mockWorktree{createPath: "/tmp/wt"}
	ag := &mockAgent{
		outputs: []string{
			"",                  // step 4: initial agent run
			"fixed everything.", // step 8: no markers
		},
	}
	vc := &mockVCS{
		pr:       &provider.PR{URL: "https://github.com/owner/repo/pull/1", Number: 1},
		comments: []provider.Comment{{ID: "1", Author: "reviewer", Body: "Claude finished reviewing: fix stuff"}},
	}

	planPath := writePlan(t, "implement auth")
	rs := newRunState(planPath)

	err := Run(context.Background(), testConfigWithCR(), defaultProviders(wt, ag, vc), planPath, rs, testLogger())

	require.NoError(t, err)
	assert.True(t, vc.postCommentCalled)
	assert.Equal(t, "CR feedback addressed. Changes pushed.", vc.postCommentBody)
	assert.Empty(t, rs.CRFixSummary)
}
