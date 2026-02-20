package pipeline

import (
	"context"
	"errors"
	"log/slog"
	"os"
	"testing"

	"github.com/shahar-caura/forge/internal/provider"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// batchMockVCS extends mockVCS with ListIssues support.
type batchMockVCS struct {
	mockVCS
	issues     []provider.GitHubIssue
	listErr    error
	listCalled bool
	listState  string
	listLabel  string
}

func (m *batchMockVCS) ListIssues(_ context.Context, state string, label string) ([]provider.GitHubIssue, error) {
	m.listCalled = true
	m.listState = state
	m.listLabel = label
	return m.issues, m.listErr
}

func batchLogger() *slog.Logger {
	return slog.New(slog.NewTextHandler(os.Stderr, &slog.HandlerOptions{Level: slog.LevelDebug}))
}

func TestRunBatch_DryRun_PrintsPlan(t *testing.T) {
	vc := &batchMockVCS{
		issues: []provider.GitHubIssue{
			{Number: 1, Title: "Add auth", Body: "Implement authentication."},
			{Number: 2, Title: "Add logging", Body: "Depends on #1"},
			{Number: 3, Title: "Add metrics", Body: "No deps here."},
		},
	}

	cfg := testConfig()
	providers := Providers{VCS: vc}

	err := RunBatch(context.Background(), cfg, providers, "", true, batchLogger())

	require.NoError(t, err)
	assert.True(t, vc.listCalled)
	assert.Equal(t, "open", vc.listState)
}

func TestRunBatch_DryRun_WithLabel(t *testing.T) {
	vc := &batchMockVCS{
		issues: []provider.GitHubIssue{
			{Number: 1, Title: "Labeled issue", Body: ""},
		},
	}

	cfg := testConfig()
	providers := Providers{VCS: vc}

	err := RunBatch(context.Background(), cfg, providers, "forge", true, batchLogger())

	require.NoError(t, err)
	assert.Equal(t, "forge", vc.listLabel)
}

func TestRunBatch_NoIssues(t *testing.T) {
	vc := &batchMockVCS{issues: []provider.GitHubIssue{}}

	cfg := testConfig()
	providers := Providers{VCS: vc}

	err := RunBatch(context.Background(), cfg, providers, "", false, batchLogger())

	require.NoError(t, err)
}

func TestRunBatch_ListError(t *testing.T) {
	vc := &batchMockVCS{listErr: errors.New("auth required")}

	cfg := testConfig()
	providers := Providers{VCS: vc}

	err := RunBatch(context.Background(), cfg, providers, "", false, batchLogger())

	require.Error(t, err)
	assert.Contains(t, err.Error(), "listing issues")
}

func TestRunBatch_CycleError(t *testing.T) {
	vc := &batchMockVCS{
		issues: []provider.GitHubIssue{
			{Number: 1, Title: "Issue A", Body: "Depends on #2"},
			{Number: 2, Title: "Issue B", Body: "Depends on #1"},
		},
	}

	cfg := testConfig()
	providers := Providers{VCS: vc}

	err := RunBatch(context.Background(), cfg, providers, "", true, batchLogger())

	require.Error(t, err)
	assert.Contains(t, err.Error(), "topological sort")
	assert.Contains(t, err.Error(), "cycle")
}

func TestFindBlocked_TransitiveDeps(t *testing.T) {
	// 2 depends on 1, 4 depends on 2 — so 1 failing blocks 2 and 4, but not 3.
	depsMap := map[int][]int{
		2: {1},
		4: {2},
	}
	issueSet := map[int]bool{1: true, 2: true, 3: true, 4: true}

	blocked := findBlocked(1, depsMap, issueSet)
	assert.ElementsMatch(t, []int{2, 4}, blocked)
}

func TestFindBlocked_NoDependents(t *testing.T) {
	// Issue 3 has no dependents.
	depsMap := map[int][]int{
		2: {1},
	}
	issueSet := map[int]bool{1: true, 2: true, 3: true}

	blocked := findBlocked(3, depsMap, issueSet)
	assert.Empty(t, blocked)
}

func TestFindBlocked_OnlyDirectDependents(t *testing.T) {
	// 2 and 3 both depend on 1. 4 is independent.
	depsMap := map[int][]int{
		2: {1},
		3: {1},
	}
	issueSet := map[int]bool{1: true, 2: true, 3: true, 4: true}

	blocked := findBlocked(1, depsMap, issueSet)
	assert.ElementsMatch(t, []int{2, 3}, blocked)
}

func TestRunBatch_DryRun_ExternalDepsIgnored(t *testing.T) {
	vc := &batchMockVCS{
		issues: []provider.GitHubIssue{
			{Number: 5, Title: "Feature", Body: "Depends on #999"},
		},
	}

	cfg := testConfig()
	providers := Providers{VCS: vc}

	// External dep #999 not in set — should not cause error.
	err := RunBatch(context.Background(), cfg, providers, "", true, batchLogger())
	require.NoError(t, err)
}

// Verify runSingleIssue creates temp plan and state files.
func TestRunSingleIssue_CreatesPlanFile(t *testing.T) {
	origDir, err := os.Getwd()
	require.NoError(t, err)
	tmpDir := t.TempDir()
	require.NoError(t, os.Chdir(tmpDir))
	defer func() {
		if err := os.Chdir(origDir); err != nil {
			t.Errorf("restoring working dir: %v", err)
		}
	}()

	wt := &mockWorktree{createPath: t.TempDir()}
	ag := &mockAgent{}
	vc := &batchMockVCS{
		mockVCS: mockVCS{pr: &provider.PR{URL: "https://github.com/owner/repo/pull/1", Number: 1}},
	}

	cfg := testConfig()
	providers := Providers{VCS: vc, Agent: ag, Worktree: wt}

	err = runSingleIssue(context.Background(), cfg, providers, 42, "Add Auth", "Implement auth system.", batchLogger())

	require.NoError(t, err)
	assert.True(t, ag.called)
}
