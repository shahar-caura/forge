package pipeline

import (
	"context"
	"log/slog"
	"os"
	"testing"

	"github.com/shahar-caura/forge/internal/provider"
	"github.com/shahar-caura/forge/internal/state"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func pushProviders(vc *mockVCS) Providers {
	return Providers{VCS: vc}
}

func newPushState() *state.RunState {
	rs := state.New("20260219-120000-push-test", "")
	rs.Mode = "push"
	return rs
}

func defaultPushOpts() PushOpts {
	return PushOpts{
		Title:   "My Feature",
		Message: "Implement my feature",
		Dir:     "/tmp/repo",
		Branch:  "forge/my-feature",
	}
}

func TestPush_HappyPath_CommitAndPR(t *testing.T) {
	vc := &mockVCS{pr: &provider.PR{URL: "https://github.com/owner/repo/pull/5", Number: 5}}
	n := &mockNotifier{}

	rs := newPushState()
	opts := defaultPushOpts()

	err := Push(context.Background(), testConfig(), Providers{VCS: vc, Notifier: n}, opts, rs, testLogger())

	require.NoError(t, err)
	assert.True(t, vc.commitCalled, "should commit changes")
	assert.False(t, vc.pushCalled, "should not call Push separately when CommitAndPush is used")
	assert.Equal(t, state.RunCompleted, rs.Status)
	assert.Equal(t, "https://github.com/owner/repo/pull/5", rs.PRUrl)
	assert.Equal(t, 5, rs.PRNumber)
	assert.Equal(t, "forge/my-feature", rs.Branch)
	assert.Equal(t, "My Feature", rs.PlanTitle)

	// All 11 steps should be completed.
	for _, step := range rs.Steps {
		assert.Equal(t, state.StepCompleted, step.Status, "step %q should be completed", step.Name)
	}

	// Notifier called.
	require.Len(t, n.messages, 1)
	assert.Contains(t, n.messages[0], "PR ready for review")
}

func TestPush_PushOnly_NoUncommittedChanges(t *testing.T) {
	vc := &mockVCS{
		noChanges: true,
		pr:        &provider.PR{URL: "https://github.com/owner/repo/pull/6", Number: 6},
	}

	rs := newPushState()
	opts := defaultPushOpts()

	err := Push(context.Background(), testConfig(), pushProviders(vc), opts, rs, testLogger())

	require.NoError(t, err)
	assert.False(t, vc.commitCalled, "should NOT call CommitAndPush when no changes")
	assert.True(t, vc.pushCalled, "should call Push for existing commits")
	assert.Equal(t, state.RunCompleted, rs.Status)
}

func TestPush_TitleFromFlag(t *testing.T) {
	vc := &mockVCS{pr: &provider.PR{URL: "url", Number: 1}}
	rs := newPushState()
	opts := PushOpts{
		Title:   "Custom Title",
		Message: "body",
		Dir:     "/tmp/repo",
		Branch:  "forge/something",
	}

	err := Push(context.Background(), testConfig(), pushProviders(vc), opts, rs, testLogger())

	require.NoError(t, err)
	assert.Equal(t, "Custom Title", rs.PlanTitle)
}

func TestPush_TitleInferredFromBranch(t *testing.T) {
	vc := &mockVCS{pr: &provider.PR{URL: "url", Number: 1}}
	rs := newPushState()
	opts := PushOpts{
		Title:   "", // no title flag
		Message: "body",
		Dir:     "/tmp/repo",
		Branch:  "forge/my-cool-feature",
	}

	err := Push(context.Background(), testConfig(), pushProviders(vc), opts, rs, testLogger())

	require.NoError(t, err)
	assert.Equal(t, "My Cool Feature", rs.PlanTitle)
}

func TestPush_SkipIssueWhenNoTracker(t *testing.T) {
	vc := &mockVCS{pr: &provider.PR{URL: "url", Number: 1}}
	rs := newPushState()
	opts := defaultPushOpts()

	err := Push(context.Background(), testConfig(), Providers{VCS: vc, Tracker: nil}, opts, rs, testLogger())

	require.NoError(t, err)
	assert.Empty(t, rs.IssueKey)
}

func TestPush_WithTracker(t *testing.T) {
	vc := &mockVCS{pr: &provider.PR{URL: "url", Number: 1}}
	tr := &mockTracker{issue: &provider.Issue{Key: "PROJ-99", URL: "https://jira.example.com/browse/PROJ-99"}}
	rs := newPushState()
	opts := defaultPushOpts()

	err := Push(context.Background(), testConfig(), Providers{VCS: vc, Tracker: tr}, opts, rs, testLogger())

	require.NoError(t, err)
	assert.Equal(t, "PROJ-99", rs.IssueKey)
	assert.Equal(t, "https://jira.example.com/browse/PROJ-99", rs.IssueURL)
}

func TestPush_SkipNotifyWhenNoNotifier(t *testing.T) {
	vc := &mockVCS{pr: &provider.PR{URL: "url", Number: 1}}
	rs := newPushState()
	opts := defaultPushOpts()

	err := Push(context.Background(), testConfig(), Providers{VCS: vc, Notifier: nil}, opts, rs, testLogger())

	require.NoError(t, err)
	assert.Equal(t, state.RunCompleted, rs.Status)
}

func TestPush_StateCompatibleWithStatus(t *testing.T) {
	vc := &mockVCS{pr: &provider.PR{URL: "https://github.com/owner/repo/pull/1", Number: 1}}
	rs := newPushState()
	opts := defaultPushOpts()

	err := Push(context.Background(), testConfig(), pushProviders(vc), opts, rs, testLogger())

	require.NoError(t, err)
	assert.Equal(t, "push", rs.Mode)
	assert.Equal(t, state.RunCompleted, rs.Status)
	require.Len(t, rs.Steps, 11, "push should use the same 11-step array")
}

func TestPush_PRCreateFails(t *testing.T) {
	vc := &mockVCS{prErr: assert.AnError}
	rs := newPushState()
	opts := defaultPushOpts()

	err := Push(context.Background(), testConfig(), pushProviders(vc), opts, rs, testLogger())

	require.Error(t, err)
	assert.Contains(t, err.Error(), "step 7")
	assert.Contains(t, err.Error(), "create pr")
	assert.Equal(t, state.RunFailed, rs.Status)
}

func TestPush_FailureNotification(t *testing.T) {
	vc := &mockVCS{prErr: assert.AnError}
	n := &mockNotifier{}
	rs := newPushState()
	opts := defaultPushOpts()

	err := Push(context.Background(), testConfig(), Providers{VCS: vc, Notifier: n}, opts, rs, testLogger())

	require.Error(t, err)
	require.Len(t, n.messages, 1)
	assert.Contains(t, n.messages[0], "forge push failed")
}

// --- TitleFromBranch tests ---

func TestTitleFromBranch(t *testing.T) {
	tests := []struct {
		branch string
		want   string
	}{
		{"forge/my-feature", "My Feature"},
		{"forge/deploy-server", "Deploy Server"},
		{"PROJ-123-add-auth", "Add Auth"},
		{"CAURA-42-fix-bug", "Fix Bug"},
		{"feature/custom-branch", "Feature/custom Branch"},
		{"plain-branch", "Plain Branch"},
	}
	for _, tt := range tests {
		t.Run(tt.branch, func(t *testing.T) {
			assert.Equal(t, tt.want, TitleFromBranch(tt.branch))
		})
	}
}

// --- Resume compatibility test ---

func TestPush_ResumeFromStep6(t *testing.T) {
	vc := &mockVCS{pr: &provider.PR{URL: "https://github.com/owner/repo/pull/10", Number: 10}}
	rs := newPushState()
	opts := defaultPushOpts()

	// Simulate steps 0-5 completed, step 6 (create pr) failed.
	for i := 0; i <= 5; i++ {
		rs.Steps[i].Status = state.StepCompleted
	}
	rs.Steps[6].Status = state.StepFailed
	rs.Branch = "forge/my-feature"
	rs.PlanTitle = "My Feature"
	rs.Status = state.RunActive

	err := Push(context.Background(), testConfig(), pushProviders(vc), opts, rs, testLogger())

	require.NoError(t, err)
	assert.False(t, vc.commitCalled, "should not re-commit on resume")
	assert.Equal(t, state.RunCompleted, rs.Status)
	assert.Equal(t, "https://github.com/owner/repo/pull/10", rs.PRUrl)
}

// --- CR loop tests for push ---

func TestPush_CREnabled_PollFindsComment(t *testing.T) {
	vc := &mockVCS{
		pr:       &provider.PR{URL: "https://github.com/owner/repo/pull/1", Number: 1},
		comments: []provider.Comment{{ID: "1", Author: "reviewer", Body: "Claude finished reviewing"}},
	}
	rs := newPushState()
	opts := defaultPushOpts()

	err := Push(context.Background(), testConfigWithCR(), pushProviders(vc), opts, rs, testLogger())

	require.NoError(t, err)
	assert.True(t, vc.getCommentsCalled, "should poll for comments")
	assert.Equal(t, "Claude finished reviewing", rs.CRFeedback)
	assert.Equal(t, state.RunCompleted, rs.Status)
	// Steps 8-9 should be completed (auto-complete, no agent for push).
	assert.Equal(t, state.StepCompleted, rs.Steps[8].Status)
	assert.Equal(t, state.StepCompleted, rs.Steps[9].Status)
}

func TestPush_CREnabled_PollTimeout(t *testing.T) {
	vc := &mockVCS{
		pr:       &provider.PR{URL: "https://github.com/owner/repo/pull/1", Number: 1},
		comments: []provider.Comment{}, // no matching comments
	}
	rs := newPushState()
	opts := defaultPushOpts()

	err := Push(context.Background(), testConfigWithCR(), pushProviders(vc), opts, rs, testLogger())

	require.Error(t, err)
	assert.Contains(t, err.Error(), "poll timeout")
	assert.Contains(t, err.Error(), "step 8") // step 7 is 0-indexed, error is 1-based
	assert.Equal(t, state.RunFailed, rs.Status)
}

func TestPush_CRDisabled_SkipsSteps7to9(t *testing.T) {
	vc := &mockVCS{pr: &provider.PR{URL: "https://github.com/owner/repo/pull/1", Number: 1}}
	rs := newPushState()
	opts := defaultPushOpts()

	// testConfig() has CR disabled by default.
	err := Push(context.Background(), testConfig(), pushProviders(vc), opts, rs, testLogger())

	require.NoError(t, err)
	assert.False(t, vc.getCommentsCalled, "should not poll when CR disabled")
	assert.Equal(t, state.RunCompleted, rs.Status)
	// All steps completed.
	for _, step := range rs.Steps {
		assert.Equal(t, state.StepCompleted, step.Status, "step %q should be completed", step.Name)
	}
}

func TestPush_LoggerOutput(t *testing.T) {
	// Just verify Push doesn't panic with debug logger.
	vc := &mockVCS{pr: &provider.PR{URL: "url", Number: 1}}
	rs := newPushState()
	opts := defaultPushOpts()
	logger := slog.New(slog.NewTextHandler(os.Stderr, &slog.HandlerOptions{Level: slog.LevelDebug}))

	err := Push(context.Background(), testConfig(), pushProviders(vc), opts, rs, logger)
	require.NoError(t, err)
}
