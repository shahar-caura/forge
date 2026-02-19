package main

import (
	"context"
	"fmt"
	"log/slog"
	"os"
	"os/exec"
	"os/signal"
	"strings"
	"time"

	"github.com/shahar-caura/forge/internal/config"
	"github.com/shahar-caura/forge/internal/pipeline"
	"github.com/shahar-caura/forge/internal/provider/notifier"
	"github.com/shahar-caura/forge/internal/provider/tracker"
	"github.com/shahar-caura/forge/internal/provider/vcs"
	"github.com/shahar-caura/forge/internal/state"
	"github.com/spf13/cobra"
)

func newPushCmd(logger *slog.Logger) *cobra.Command {
	var title, message string

	cmd := &cobra.Command{
		Use:   "push",
		Short: "Push current branch as a PR",
		Args:  cobra.NoArgs,
		RunE: func(cmd *cobra.Command, args []string) error {
			return cmdPush(logger, title, message)
		},
	}

	cmd.Flags().StringVarP(&title, "title", "t", "", "PR title (required when on base branch)")
	cmd.Flags().StringVarP(&message, "message", "m", "", "PR body")

	return cmd
}

func cmdPush(logger *slog.Logger, title, message string) error {
	// Load config.
	cfg, err := config.Load("forge.yaml")
	if err != nil {
		return fmt.Errorf("loading config: %w", err)
	}

	// Must be inside a git repo.
	if err := exec.Command("git", "rev-parse", "--is-inside-work-tree").Run(); err != nil {
		return fmt.Errorf("not inside a git repository")
	}

	// Detect current branch.
	branchOut, err := exec.Command("git", "branch", "--show-current").Output()
	if err != nil {
		return fmt.Errorf("detecting current branch: %w", err)
	}
	branch := strings.TrimSpace(string(branchOut))
	if branch == "" {
		return fmt.Errorf("detached HEAD; checkout a branch first")
	}

	// No unfinished rebase/merge.
	cwd, err := os.Getwd()
	if err != nil {
		return fmt.Errorf("getting working directory: %w", err)
	}
	if reason := detectDirtyGitState(cwd); reason != "" {
		return fmt.Errorf("unfinished %s; resolve it before pushing", reason)
	}

	// If on base branch, create a feature branch from uncommitted changes.
	if branch == cfg.VCS.BaseBranch {
		if title == "" {
			return fmt.Errorf("on base branch %q; provide -t with a title to generate a branch name", cfg.VCS.BaseBranch)
		}
		branch = pipeline.BranchName("", title)
		cmd := exec.Command("git", "checkout", "-b", branch)
		if out, err := cmd.CombinedOutput(); err != nil {
			return fmt.Errorf("creating branch %q: %w: %s", branch, err, strings.TrimSpace(string(out)))
		}
		logger.Info("created branch", "branch", branch)
	}

	// Must have something to ship: uncommitted changes or unpushed commits.
	hasChanges := false
	statusOut, err := exec.Command("git", "status", "--porcelain").Output()
	if err == nil {
		hasChanges = len(strings.TrimSpace(string(statusOut))) > 0
	}
	unpushed := hasUnpushedCommits(cwd, branch)
	if !hasChanges && !unpushed {
		return fmt.Errorf("nothing to ship: no uncommitted changes and no unpushed commits")
	}

	// Generate run ID.
	slug := pipeline.SlugFromTitle(branch)
	runID := time.Now().Format("20060102-150405") + "-" + slug

	rs := state.New(runID, "")
	rs.Mode = "push"
	if err := rs.Save(); err != nil {
		return fmt.Errorf("saving initial run state: %w", err)
	}

	logger.Info("starting push", "id", runID, "branch", branch)

	providers, err := wirePushProviders(cfg, logger)
	if err != nil {
		return err
	}

	ctx, stop := signal.NotifyContext(context.Background(), os.Interrupt)
	defer stop()

	opts := pipeline.PushOpts{
		Title:   title,
		Message: message,
		Dir:     cwd,
		Branch:  branch,
	}

	pipelineErr := pipeline.Push(ctx, cfg, providers, opts, rs, logger)

	if pipelineErr == nil && rs.PRUrl != "" {
		fmt.Println(rs.PRUrl)
	}

	// Best-effort cleanup.
	if deleted, err := state.Cleanup(cfg.State.Retention.Duration); err != nil {
		logger.Warn("state cleanup failed", "error", err)
	} else if deleted > 0 {
		logger.Info("cleaned up old run states", "deleted", deleted)
	}

	return pipelineErr
}

// wirePushProviders wires only the providers needed for push (VCS, Tracker, Notifier).
func wirePushProviders(cfg *config.Config, logger *slog.Logger) (pipeline.Providers, error) {
	p := pipeline.Providers{
		VCS: vcs.New(cfg.VCS.Repo, logger),
	}

	if cfg.Tracker.Provider != "" {
		p.Tracker = tracker.New(cfg.Tracker.BaseURL, cfg.Tracker.Project, cfg.Tracker.Email, cfg.Tracker.Token, cfg.Tracker.BoardID)
	}

	if cfg.Notifier.Provider != "" {
		p.Notifier = notifier.New(cfg.Notifier.WebhookURL)
	}

	return p, nil
}
