package vcs

import (
	"context"
	"encoding/json"
	"fmt"
	"log/slog"
	"os"
	"os/exec"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func testLogger() *slog.Logger {
	return slog.New(slog.NewTextHandler(os.Stderr, &slog.HandlerOptions{Level: slog.LevelDebug}))
}

// callTracker records which commands were invoked and returns scripted results.
type callTracker struct {
	calls   []string
	results map[string]stubResult
}

type stubResult struct {
	stdout   string
	exitCode int
}

func (ct *callTracker) commandContext(ctx context.Context, name string, args ...string) *exec.Cmd {
	key := name + " " + joinArgs(args)
	ct.calls = append(ct.calls, key)

	r, ok := ct.results[name]
	if !ok {
		// Default: succeed silently.
		return exec.CommandContext(ctx, "true")
	}

	if r.exitCode != 0 {
		return exec.CommandContext(ctx, "sh", "-c", fmt.Sprintf("echo '%s' >&2; exit %d", r.stdout, r.exitCode))
	}
	return exec.CommandContext(ctx, "echo", "-n", r.stdout)
}

func joinArgs(args []string) string {
	s := ""
	for i, a := range args {
		if i > 0 {
			s += " "
		}
		s += a
	}
	return s
}

func TestCommitAndPush_Success(t *testing.T) {
	ct := &callTracker{results: map[string]stubResult{}}
	g := New("owner/repo", testLogger())
	g.commandContext = ct.commandContext

	err := g.CommitAndPush(context.Background(), t.TempDir(), "feat-branch", "add feature")
	require.NoError(t, err)
	assert.Len(t, ct.calls, 3)
}

func TestCommitAndPush_CommitFails(t *testing.T) {
	ct := &callTracker{results: map[string]stubResult{
		"git": {stdout: "nothing to commit", exitCode: 1},
	}}
	g := New("owner/repo", testLogger())
	g.commandContext = ct.commandContext

	err := g.CommitAndPush(context.Background(), t.TempDir(), "feat-branch", "add feature")
	require.Error(t, err)
	assert.Contains(t, err.Error(), "git add")
}

func TestCreatePR_Success(t *testing.T) {
	ct := &callTracker{results: map[string]stubResult{
		"gh": {stdout: "https://github.com/owner/repo/pull/42", exitCode: 0},
	}}
	g := New("owner/repo", testLogger())
	g.commandContext = ct.commandContext

	pr, err := g.CreatePR(context.Background(), "feat-branch", "main", "Add feature", "body text")
	require.NoError(t, err)
	assert.Equal(t, "https://github.com/owner/repo/pull/42", pr.URL)
	assert.Equal(t, 42, pr.Number)
}

func TestCreatePR_Failure(t *testing.T) {
	ct := &callTracker{results: map[string]stubResult{
		"gh": {stdout: "auth required", exitCode: 1},
	}}
	g := New("owner/repo", testLogger())
	g.commandContext = ct.commandContext

	_, err := g.CreatePR(context.Background(), "feat-branch", "main", "title", "body")
	require.Error(t, err)
	assert.Contains(t, err.Error(), "gh pr create")
}

// --- GetPRComments tests ---

func TestGetPRComments_Success(t *testing.T) {
	type ghComment struct {
		ID   int `json:"id"`
		User struct {
			Login string `json:"login"`
		} `json:"user"`
		Body string `json:"body"`
	}
	c1 := ghComment{ID: 100, Body: "Looks good"}
	c1.User.Login = "reviewer"
	c2 := ghComment{ID: 101, Body: "Claude finished"}
	c2.User.Login = "claude-bot"

	data, err := json.Marshal([]ghComment{c1, c2})
	require.NoError(t, err)

	ct := &callTracker{results: map[string]stubResult{
		"gh": {stdout: string(data), exitCode: 0},
	}}
	g := New("owner/repo", testLogger())
	g.commandContext = ct.commandContext

	comments, err := g.GetPRComments(context.Background(), 42)
	require.NoError(t, err)
	require.Len(t, comments, 2)
	assert.Equal(t, "100", comments[0].ID)
	assert.Equal(t, "reviewer", comments[0].Author)
	assert.Equal(t, "Looks good", comments[0].Body)
	assert.Equal(t, "claude-bot", comments[1].Author)
}

func TestGetPRComments_APIError(t *testing.T) {
	ct := &callTracker{results: map[string]stubResult{
		"gh": {stdout: "not found", exitCode: 1},
	}}
	g := New("owner/repo", testLogger())
	g.commandContext = ct.commandContext

	_, err := g.GetPRComments(context.Background(), 999)
	require.Error(t, err)
	assert.Contains(t, err.Error(), "gh api get comments")
}

// --- PostPRComment tests ---

func TestPostPRComment_Success(t *testing.T) {
	ct := &callTracker{results: map[string]stubResult{}}
	g := New("owner/repo", testLogger())
	g.commandContext = ct.commandContext

	err := g.PostPRComment(context.Background(), 42, "Fix applied")
	require.NoError(t, err)
}

func TestPostPRComment_Error(t *testing.T) {
	ct := &callTracker{results: map[string]stubResult{
		"gh": {stdout: "auth required", exitCode: 1},
	}}
	g := New("owner/repo", testLogger())
	g.commandContext = ct.commandContext

	err := g.PostPRComment(context.Background(), 42, "Fix applied")
	require.Error(t, err)
	assert.Contains(t, err.Error(), "gh pr comment")
}

// --- GetIssue tests ---

func TestGetIssue_Success(t *testing.T) {
	issueJSON, err := json.Marshal(map[string]interface{}{
		"number": 43,
		"title":  "Add dark mode",
		"body":   "Implement dark mode for the app.",
		"url":    "https://github.com/owner/repo/issues/43",
	})
	require.NoError(t, err)

	ct := &callTracker{results: map[string]stubResult{
		"gh": {stdout: string(issueJSON), exitCode: 0},
	}}
	g := New("owner/repo", testLogger())
	g.commandContext = ct.commandContext

	issue, err := g.GetIssue(context.Background(), 43)
	require.NoError(t, err)
	assert.Equal(t, 43, issue.Number)
	assert.Equal(t, "Add dark mode", issue.Title)
	assert.Equal(t, "Implement dark mode for the app.", issue.Body)
	assert.Equal(t, "https://github.com/owner/repo/issues/43", issue.URL)
}

func TestGetIssue_Error(t *testing.T) {
	ct := &callTracker{results: map[string]stubResult{
		"gh": {stdout: "not found", exitCode: 1},
	}}
	g := New("owner/repo", testLogger())
	g.commandContext = ct.commandContext

	_, err := g.GetIssue(context.Background(), 999)
	require.Error(t, err)
	assert.Contains(t, err.Error(), "gh issue view")
}

// --- ListIssues tests ---

func TestListIssues_Success(t *testing.T) {
	issuesJSON, err := json.Marshal([]map[string]interface{}{
		{"number": 1, "title": "Add auth", "body": "Implement auth.", "url": "https://github.com/owner/repo/issues/1"},
		{"number": 2, "title": "Add logging", "body": "Depends on #1", "url": "https://github.com/owner/repo/issues/2"},
	})
	require.NoError(t, err)

	ct := &callTracker{results: map[string]stubResult{
		"gh": {stdout: string(issuesJSON), exitCode: 0},
	}}
	g := New("owner/repo", testLogger())
	g.commandContext = ct.commandContext

	issues, err := g.ListIssues(context.Background(), "open", "")
	require.NoError(t, err)
	require.Len(t, issues, 2)
	assert.Equal(t, 1, issues[0].Number)
	assert.Equal(t, "Add auth", issues[0].Title)
	assert.Equal(t, 2, issues[1].Number)
	assert.Equal(t, "Depends on #1", issues[1].Body)
}

func TestListIssues_WithLabel(t *testing.T) {
	ct := &callTracker{results: map[string]stubResult{
		"gh": {stdout: "[]", exitCode: 0},
	}}
	g := New("owner/repo", testLogger())
	g.commandContext = ct.commandContext

	issues, err := g.ListIssues(context.Background(), "open", "forge")
	require.NoError(t, err)
	assert.Empty(t, issues)

	// Verify --label flag was passed.
	require.Len(t, ct.calls, 1)
	assert.Contains(t, ct.calls[0], "--label forge")
}

func TestListIssues_Error(t *testing.T) {
	ct := &callTracker{results: map[string]stubResult{
		"gh": {stdout: "auth required", exitCode: 1},
	}}
	g := New("owner/repo", testLogger())
	g.commandContext = ct.commandContext

	_, err := g.ListIssues(context.Background(), "open", "")
	require.Error(t, err)
	assert.Contains(t, err.Error(), "gh issue list")
}

// --- AmendAndForcePush tests ---

func TestAmendAndForcePush_Success(t *testing.T) {
	ct := &callTracker{results: map[string]stubResult{}}
	g := New("owner/repo", testLogger())
	g.commandContext = ct.commandContext

	err := g.AmendAndForcePush(context.Background(), t.TempDir(), "feat-branch")
	require.NoError(t, err)
	assert.Len(t, ct.calls, 3)
}

func TestAmendAndForcePush_Failure(t *testing.T) {
	ct := &callTracker{results: map[string]stubResult{
		"git": {stdout: "nothing to commit", exitCode: 1},
	}}
	g := New("owner/repo", testLogger())
	g.commandContext = ct.commandContext

	err := g.AmendAndForcePush(context.Background(), t.TempDir(), "feat-branch")
	require.Error(t, err)
	assert.Contains(t, err.Error(), "git add")
}
