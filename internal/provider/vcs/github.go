package vcs

import (
	"context"
	"encoding/json"
	"fmt"
	"log/slog"
	"os/exec"
	"strconv"
	"strings"

	"github.com/shahar-caura/forge/internal/provider"
)

// GitHub implements provider.VCS using git and gh CLIs.
type GitHub struct {
	Repo   string
	Logger *slog.Logger

	// commandContext is overridable for testing.
	commandContext func(ctx context.Context, name string, args ...string) *exec.Cmd
}

// New creates a new GitHub VCS provider.
func New(repo string, logger *slog.Logger) *GitHub {
	return &GitHub{
		Repo:           repo,
		Logger:         logger,
		commandContext: exec.CommandContext,
	}
}

func (g *GitHub) CommitAndPush(ctx context.Context, dir, branch, message string) error {
	steps := []struct {
		name string
		args []string
	}{
		{"git add", []string{"git", "add", "."}},
		{"git commit", []string{"git", "commit", "-m", message}},
		{"git push", []string{"git", "push", "-u", "origin", branch}},
	}

	for i, step := range steps {
		g.Logger.Info("running", "step", step.name)

		cmd := g.commandContext(ctx, step.args[0], step.args[1:]...)
		cmd.Dir = dir

		out, err := cmd.CombinedOutput()
		if err != nil {
			return fmt.Errorf("step %d (%s): %w: %s", i+1, step.name, err, strings.TrimSpace(string(out)))
		}
	}

	return nil
}

func (g *GitHub) Push(ctx context.Context, dir, branch string) error {
	g.Logger.Info("pushing", "branch", branch)
	cmd := g.commandContext(ctx, "git", "push", "-u", "origin", branch)
	cmd.Dir = dir
	out, err := cmd.CombinedOutput()
	if err != nil {
		return fmt.Errorf("git push: %w: %s", err, strings.TrimSpace(string(out)))
	}
	return nil
}

func (g *GitHub) CreatePR(ctx context.Context, branch, baseBranch, title, body string) (*provider.PR, error) {
	g.Logger.Info("creating PR", "branch", branch, "base", baseBranch)

	cmd := g.commandContext(ctx, "gh", "pr", "create",
		"--repo", g.Repo,
		"--head", branch,
		"--base", baseBranch,
		"--title", title,
		"--body", body,
	)

	out, err := cmd.CombinedOutput()
	if err != nil {
		return nil, fmt.Errorf("gh pr create: %w: %s", err, strings.TrimSpace(string(out)))
	}

	url := strings.TrimSpace(string(out))

	pr := &provider.PR{URL: url}

	// Best-effort parse PR number from URL.
	if parts := strings.Split(url, "/"); len(parts) > 0 {
		if n, err := strconv.Atoi(parts[len(parts)-1]); err == nil {
			pr.Number = n
		}
	}

	g.Logger.Info("PR created", "url", url, "number", pr.Number)
	return pr, nil
}

func (g *GitHub) GetPRComments(ctx context.Context, prNumber int) ([]provider.Comment, error) {
	g.Logger.Info("fetching PR comments", "pr", prNumber)

	cmd := g.commandContext(ctx, "gh", "api",
		fmt.Sprintf("repos/%s/issues/%d/comments", g.Repo, prNumber),
	)

	out, err := cmd.CombinedOutput()
	if err != nil {
		return nil, fmt.Errorf("gh api get comments: %w: %s", err, strings.TrimSpace(string(out)))
	}

	var raw []struct {
		ID   int `json:"id"`
		User struct {
			Login string `json:"login"`
		} `json:"user"`
		Body string `json:"body"`
	}
	if err := json.Unmarshal(out, &raw); err != nil {
		return nil, fmt.Errorf("parsing PR comments: %w", err)
	}

	comments := make([]provider.Comment, len(raw))
	for i, r := range raw {
		comments[i] = provider.Comment{
			ID:     strconv.Itoa(r.ID),
			Author: r.User.Login,
			Body:   r.Body,
		}
	}

	return comments, nil
}

func (g *GitHub) PostPRComment(ctx context.Context, prNumber int, body string) error {
	g.Logger.Info("posting PR comment", "pr", prNumber)

	cmd := g.commandContext(ctx, "gh", "pr", "comment",
		strconv.Itoa(prNumber),
		"--repo", g.Repo,
		"--body", body,
	)

	out, err := cmd.CombinedOutput()
	if err != nil {
		return fmt.Errorf("gh pr comment: %w: %s", err, strings.TrimSpace(string(out)))
	}
	return nil
}

func (g *GitHub) HasChanges(ctx context.Context, dir string) (bool, error) {
	cmd := g.commandContext(ctx, "git", "status", "--porcelain")
	cmd.Dir = dir
	out, err := cmd.Output()
	if err != nil {
		return false, fmt.Errorf("git status: %w", err)
	}
	return len(strings.TrimSpace(string(out))) > 0, nil
}

func (g *GitHub) AmendAndForcePush(ctx context.Context, dir, branch string) error {
	return g.amendAndForcePush(ctx, dir, branch, "--no-edit", "")
}

func (g *GitHub) AmendAndForcePushMsg(ctx context.Context, dir, branch, message string) error {
	return g.amendAndForcePush(ctx, dir, branch, "-m", message)
}

func (g *GitHub) GetIssue(ctx context.Context, number int) (*provider.GitHubIssue, error) {
	g.Logger.Info("fetching issue", "number", number)

	cmd := g.commandContext(ctx, "gh", "issue", "view",
		strconv.Itoa(number),
		"--repo", g.Repo,
		"--json", "number,title,body,url",
	)

	out, err := cmd.CombinedOutput()
	if err != nil {
		return nil, fmt.Errorf("gh issue view: %w: %s", err, strings.TrimSpace(string(out)))
	}

	var issue provider.GitHubIssue
	if err := json.Unmarshal(out, &issue); err != nil {
		return nil, fmt.Errorf("parsing issue JSON: %w", err)
	}

	return &issue, nil
}

func (g *GitHub) amendAndForcePush(ctx context.Context, dir, branch, msgFlag, msgValue string) error {
	g.Logger.Info("amending and force pushing", "branch", branch)

	commitArgs := []string{"git", "commit", "--amend", msgFlag}
	if msgValue != "" {
		commitArgs = append(commitArgs, msgValue)
	}

	steps := []struct {
		name string
		args []string
	}{
		{"git add", []string{"git", "add", "."}},
		{"git commit amend", commitArgs},
		{"git push force", []string{"git", "push", "--force-with-lease", "origin", branch}},
	}

	for i, step := range steps {
		cmd := g.commandContext(ctx, step.args[0], step.args[1:]...)
		cmd.Dir = dir

		out, err := cmd.CombinedOutput()
		if err != nil {
			return fmt.Errorf("step %d (%s): %w: %s", i+1, step.name, err, strings.TrimSpace(string(out)))
		}
	}

	return nil
}
