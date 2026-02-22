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

// batchMockVCS extends mockVCS with ListIssues and GetIssue support.
type batchMockVCS struct {
	mockVCS
	issues     []provider.GitHubIssue
	listErr    error
	listCalled bool
	listState  string
	listLabel  string
	// GetIssue support for expandDeps tests.
	getIssueMap map[int]*provider.GitHubIssue
	getIssueErr map[int]error
}

func (m *batchMockVCS) ListIssues(_ context.Context, state string, label string) ([]provider.GitHubIssue, error) {
	m.listCalled = true
	m.listState = state
	m.listLabel = label
	return m.issues, m.listErr
}

func (m *batchMockVCS) GetIssue(_ context.Context, number int) (*provider.GitHubIssue, error) {
	if m.getIssueErr != nil {
		if err, ok := m.getIssueErr[number]; ok {
			return nil, err
		}
	}
	if m.getIssueMap != nil {
		if iss, ok := m.getIssueMap[number]; ok {
			return iss, nil
		}
	}
	return nil, errors.New("issue not found")
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

// --- expandDeps tests ---

func TestExpandDeps_FetchMissing(t *testing.T) {
	vc := &batchMockVCS{
		getIssueMap: map[int]*provider.GitHubIssue{
			10: {Number: 10, Title: "Dep Issue", Body: "No further deps."},
		},
	}

	issueSet := map[int]bool{1: true}
	titleMap := map[int]string{1: "Feature"}
	bodyMap := map[int]string{1: "Depends on #10"}

	err := expandDeps(context.Background(), vc, issueSet, titleMap, bodyMap, batchLogger())

	require.NoError(t, err)
	assert.True(t, issueSet[10], "dep #10 should be added")
	assert.Equal(t, "Dep Issue", titleMap[10])
}

func TestExpandDeps_TransitiveDeps(t *testing.T) {
	vc := &batchMockVCS{
		getIssueMap: map[int]*provider.GitHubIssue{
			10: {Number: 10, Title: "Dep A", Body: "Depends on #20"},
			20: {Number: 20, Title: "Dep B", Body: "No deps."},
		},
	}

	issueSet := map[int]bool{1: true}
	titleMap := map[int]string{1: "Feature"}
	bodyMap := map[int]string{1: "Depends on #10"}

	err := expandDeps(context.Background(), vc, issueSet, titleMap, bodyMap, batchLogger())

	require.NoError(t, err)
	assert.True(t, issueSet[10], "dep #10 should be added")
	assert.True(t, issueSet[20], "transitive dep #20 should be added")
	assert.Equal(t, "Dep B", titleMap[20])
}

func TestExpandDeps_NoDeps(t *testing.T) {
	vc := &batchMockVCS{}

	issueSet := map[int]bool{1: true}
	titleMap := map[int]string{1: "Feature"}
	bodyMap := map[int]string{1: "No dependencies here."}

	err := expandDeps(context.Background(), vc, issueSet, titleMap, bodyMap, batchLogger())

	require.NoError(t, err)
	assert.Equal(t, 1, len(issueSet))
}

func TestExpandDeps_FetchErrorTreatedAsExternal(t *testing.T) {
	vc := &batchMockVCS{
		getIssueErr: map[int]error{10: errors.New("not found")},
	}

	issueSet := map[int]bool{1: true}
	titleMap := map[int]string{1: "Feature"}
	bodyMap := map[int]string{1: "Depends on #10"}

	err := expandDeps(context.Background(), vc, issueSet, titleMap, bodyMap, batchLogger())

	require.NoError(t, err, "fetch error should not fail the expansion")
	assert.False(t, issueSet[10], "failed dep should not be in set")
}

func TestExpandDeps_AlreadyInSet(t *testing.T) {
	vc := &batchMockVCS{}

	issueSet := map[int]bool{1: true, 10: true}
	titleMap := map[int]string{1: "Feature", 10: "Already Known"}
	bodyMap := map[int]string{1: "Depends on #10", 10: "No deps."}

	err := expandDeps(context.Background(), vc, issueSet, titleMap, bodyMap, batchLogger())

	require.NoError(t, err)
	assert.Equal(t, 2, len(issueSet), "no new issues should be added")
}

func TestRunBatch_DryRun_WithLabel_ExpandsDeps(t *testing.T) {
	vc := &batchMockVCS{
		issues: []provider.GitHubIssue{
			{Number: 2, Title: "Feature B", Body: "Depends on #1"},
		},
		getIssueMap: map[int]*provider.GitHubIssue{
			1: {Number: 1, Title: "Feature A", Body: "No deps."},
		},
	}

	cfg := testConfig()
	providers := Providers{VCS: vc}

	// With a label, expandDeps should fetch #1 even though it's not labeled.
	err := RunBatch(context.Background(), cfg, providers, "dashboard", true, batchLogger())

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
	assert.True(t, ag.Called())
}

// --- Multi-agent batch assignment tests ---

func TestFallbackAgent_DelegatesToPool(t *testing.T) {
	a1 := &mockAgent{output: "result from claude"}
	a2 := &mockAgent{output: "result from codex"}
	pool := NewAgentPool([]provider.Agent{a1, a2}, []string{"claude", "codex"})

	fa := NewFallbackAgent(pool, 0, batchLogger())
	output, err := fa.Run(context.Background(), "/dir", "prompt")

	require.NoError(t, err)
	assert.Equal(t, "result from claude", output)
	assert.True(t, a1.Called())
	assert.False(t, a2.Called())
}

func TestFallbackAgent_FallsBackOnRetryableError(t *testing.T) {
	a1 := &mockAgent{err: errors.New("rate limit exceeded")}
	a2 := &mockAgent{output: "result from codex"}
	pool := NewAgentPool([]provider.Agent{a1, a2}, []string{"claude", "codex"})

	fa := NewFallbackAgent(pool, 0, batchLogger())
	output, err := fa.Run(context.Background(), "/dir", "prompt")

	require.NoError(t, err)
	assert.Equal(t, "result from codex", output)
}

func TestFallbackAgent_PromptSuffix(t *testing.T) {
	a1 := &mockAgent{output: ""}
	a2 := &mockAgent{output: ""}
	pool := NewAgentPool([]provider.Agent{a1, a2}, []string{"claude", "codex"})

	fa0 := NewFallbackAgent(pool, 0, batchLogger())
	fa1 := NewFallbackAgent(pool, 1, batchLogger())

	// Both should return PromptSuffix from their assigned agent (mockAgent returns "").
	assert.Equal(t, "", fa0.PromptSuffix())
	assert.Equal(t, "", fa1.PromptSuffix())
}

func TestRunBatch_MultiAgentSpread(t *testing.T) {
	origDir, err := os.Getwd()
	require.NoError(t, err)
	tmpDir := t.TempDir()
	require.NoError(t, os.Chdir(tmpDir))
	defer func() {
		if err := os.Chdir(origDir); err != nil {
			t.Errorf("restoring working dir: %v", err)
		}
	}()

	// 3 independent issues, pool of 2 agents.
	a1 := &mockAgent{output: ""}
	a2 := &mockAgent{output: ""}
	pool := NewAgentPool([]provider.Agent{a1, a2}, []string{"claude", "codex"})

	wt := &mockWorktree{createPath: t.TempDir()}
	vc := &batchMockVCS{
		mockVCS: mockVCS{pr: &provider.PR{URL: "https://github.com/owner/repo/pull/1", Number: 1}},
		issues: []provider.GitHubIssue{
			{Number: 1, Title: "Issue A", Body: "No deps."},
			{Number: 2, Title: "Issue B", Body: "No deps."},
			{Number: 3, Title: "Issue C", Body: "No deps."},
		},
	}

	cfg := testConfig()
	providers := Providers{VCS: vc, Agent: a1, Worktree: wt, AgentPool: pool}

	err = RunBatch(context.Background(), cfg, providers, "", false, batchLogger())

	require.NoError(t, err)
	// Both agents should have been called (round-robin across 3 issues).
	assert.True(t, a1.Called(), "first agent should be called")
	assert.True(t, a2.Called(), "second agent should be called")
}

func TestRunBatch_SingleAgentPool_NoChange(t *testing.T) {
	origDir, err := os.Getwd()
	require.NoError(t, err)
	tmpDir := t.TempDir()
	require.NoError(t, os.Chdir(tmpDir))
	defer func() {
		if err := os.Chdir(origDir); err != nil {
			t.Errorf("restoring working dir: %v", err)
		}
	}()

	ag := &mockAgent{output: ""}
	pool := NewAgentPool([]provider.Agent{ag}, []string{"claude"})

	wt := &mockWorktree{createPath: t.TempDir()}
	vc := &batchMockVCS{
		mockVCS: mockVCS{pr: &provider.PR{URL: "https://github.com/owner/repo/pull/1", Number: 1}},
		issues: []provider.GitHubIssue{
			{Number: 1, Title: "Issue A", Body: "No deps."},
		},
	}

	cfg := testConfig()
	providers := Providers{VCS: vc, Agent: ag, Worktree: wt, AgentPool: pool}

	err = RunBatch(context.Background(), cfg, providers, "", false, batchLogger())

	require.NoError(t, err)
	assert.True(t, ag.Called())
}

func TestRunBatch_NilPool_BackwardsCompatible(t *testing.T) {
	origDir, err := os.Getwd()
	require.NoError(t, err)
	tmpDir := t.TempDir()
	require.NoError(t, os.Chdir(tmpDir))
	defer func() {
		if err := os.Chdir(origDir); err != nil {
			t.Errorf("restoring working dir: %v", err)
		}
	}()

	ag := &mockAgent{output: ""}
	wt := &mockWorktree{createPath: t.TempDir()}
	vc := &batchMockVCS{
		mockVCS: mockVCS{pr: &provider.PR{URL: "https://github.com/owner/repo/pull/1", Number: 1}},
		issues: []provider.GitHubIssue{
			{Number: 1, Title: "Issue A", Body: "No deps."},
		},
	}

	cfg := testConfig()
	// No AgentPool — should work as before.
	providers := Providers{VCS: vc, Agent: ag, Worktree: wt}

	err = RunBatch(context.Background(), cfg, providers, "", false, batchLogger())

	require.NoError(t, err)
	assert.True(t, ag.Called())
}
