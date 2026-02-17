package vcs

import (
	"context"
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
