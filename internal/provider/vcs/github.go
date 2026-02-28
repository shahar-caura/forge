package vcs

import (
	"context"
	"encoding/json"
	"fmt"
	"log/slog"
	"os"
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
	run := func(name string, args ...string) error {
		g.Logger.Info("running", "step", name)
		cmd := g.commandContext(ctx, args[0], args[1:]...)
		cmd.Dir = dir
		cmd.Env = append(os.Environ(), "LEFTHOOK=0")
		out, err := cmd.CombinedOutput()
		if err != nil {
			return fmt.Errorf("%s: %w: %s", name, err, strings.TrimSpace(string(out)))
		}
		return nil
	}

	if err := run("git add", "git", "add", "."); err != nil {
		return err
	}

	if err := run("git commit", "git", "commit", "-m", message); err != nil {
		// Pre-commit hooks (e.g. ruff format) may reformat files, causing the
		// commit to fail. Re-stage the modified files and retry once.
		g.Logger.Info("commit failed, re-staging and retrying (pre-commit hook may have modified files)")
		if addErr := run("git add (retry)", "git", "add", "."); addErr != nil {
			return err // return original commit error
		}
		if retryErr := run("git commit (retry)", "git", "commit", "-m", message); retryErr != nil {
			return retryErr
		}
	}

	return run("git push", "git", "push", "-u", "origin", branch)
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

func (g *GitHub) ListIssues(ctx context.Context, state string, label string) ([]provider.GitHubIssue, error) {
	g.Logger.Info("listing issues", "state", state, "label", label)

	args := []string{
		"issue", "list",
		"--repo", g.Repo,
		"--state", state,
		"--json", "number,title,body,url",
		"--limit", "200",
	}
	if label != "" {
		args = append(args, "--label", label)
	}

	cmd := g.commandContext(ctx, "gh", args...)
	out, err := cmd.CombinedOutput()
	if err != nil {
		return nil, fmt.Errorf("gh issue list: %w: %s", err, strings.TrimSpace(string(out)))
	}

	var issues []provider.GitHubIssue
	if err := json.Unmarshal(out, &issues); err != nil {
		return nil, fmt.Errorf("parsing issue list JSON: %w", err)
	}

	return issues, nil
}

func (g *GitHub) GetPRState(ctx context.Context, prNumber int) (string, error) {
	g.Logger.Info("fetching PR state", "pr", prNumber)

	cmd := g.commandContext(ctx, "gh", "pr", "view",
		strconv.Itoa(prNumber),
		"--repo", g.Repo,
		"--json", "state",
		"--jq", ".state",
	)

	out, err := cmd.CombinedOutput()
	if err != nil {
		return "", fmt.Errorf("gh pr view: %w: %s", err, strings.TrimSpace(string(out)))
	}

	return strings.TrimSpace(string(out)), nil
}

func (g *GitHub) FetchAndRebase(ctx context.Context, dir, baseBranch string) error {
	g.Logger.Info("rebasing onto latest base branch", "base", baseBranch)

	steps := []struct {
		name string
		args []string
	}{
		{"git fetch base", []string{"git", "fetch", "origin", baseBranch}},
		{"git rebase", []string{"git", "rebase", "origin/" + baseBranch}},
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

func (g *GitHub) amendAndForcePush(ctx context.Context, dir, branch, msgFlag, msgValue string) error {
	g.Logger.Info("amending and force pushing", "branch", branch)

	run := func(name string, args ...string) error {
		g.Logger.Info("running", "step", name)
		cmd := g.commandContext(ctx, args[0], args[1:]...)
		cmd.Dir = dir
		cmd.Env = append(os.Environ(), "LEFTHOOK=0")
		out, err := cmd.CombinedOutput()
		if err != nil {
			return fmt.Errorf("%s: %w: %s", name, err, strings.TrimSpace(string(out)))
		}
		return nil
	}

	if err := run("git add", "git", "add", "."); err != nil {
		return err
	}

	commitArgs := []string{"git", "commit", "--amend", msgFlag}
	if msgValue != "" {
		commitArgs = append(commitArgs, msgValue)
	}

	if err := run("git commit amend", commitArgs...); err != nil {
		g.Logger.Info("amend failed, re-staging and retrying (pre-commit hook may have modified files)")
		if addErr := run("git add (retry)", "git", "add", "."); addErr != nil {
			return err
		}
		if retryErr := run("git commit amend (retry)", commitArgs...); retryErr != nil {
			return retryErr
		}
	}

	return run("git push force", "git", "push", "--force-with-lease", "origin", branch)
}
