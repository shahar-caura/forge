package pipeline

import (
	"context"
	"fmt"
	"log/slog"
	"os"
	"path/filepath"
	"regexp"
	"strings"

	"github.com/shahar-caura/forge/internal/config"
	"github.com/shahar-caura/forge/internal/provider"
	"github.com/shahar-caura/forge/internal/state"
)

// Providers holds the wired provider implementations for a pipeline run.
type Providers struct {
	VCS      provider.VCS
	Agent    provider.Agent
	Worktree provider.Worktree
	Tracker  provider.Tracker  // nil if unconfigured
	Notifier provider.Notifier // nil if unconfigured
}

// Run executes the forge pipeline:
//
//	read plan → create issue → generate branch → create worktree → run agent → commit → PR → notify.
//
// If rs has completed steps (resume), those steps are skipped and locals are restored from rs artifacts.
func Run(ctx context.Context, cfg *config.Config, providers Providers, planPath string, rs *state.RunState, logger *slog.Logger) error {
	var (
		plan         string
		branch       string
		worktreePath string
		lastErr      error
	)

	// Restore artifacts from state on resume.
	branch = rs.Branch
	worktreePath = rs.WorktreePath

	// On failure, mark run as failed, preserve worktree, and best-effort notify.
	defer func() {
		if rs.Status != state.RunCompleted {
			rs.Status = state.RunFailed
			_ = rs.Save()

			// Best-effort failure notification — can't fail-fast when already failing.
			if providers.Notifier != nil && lastErr != nil {
				failMsg := fmt.Sprintf("forge pipeline failed: %s", lastErr)
				_ = providers.Notifier.Notify(ctx, failMsg)
			}
		}
	}()

	// Step 0: Read plan file.
	if err := runStep(rs, 0, logger, func() error {
		planBytes, err := os.ReadFile(planPath)
		if err != nil {
			return err
		}
		plan = string(planBytes)
		return nil
	}); err != nil {
		lastErr = err
		return err
	}
	// Re-read plan on resume (plan content not stored in state).
	if plan == "" {
		planBytes, err := os.ReadFile(planPath)
		if err != nil {
			return fmt.Errorf("re-reading plan on resume: %w", err)
		}
		plan = string(planBytes)
	}

	// Step 1: Create issue (optional — skipped if no tracker configured).
	if err := runStep(rs, 1, logger, func() error {
		if providers.Tracker == nil {
			logger.Info("no tracker configured, skipping")
			return nil
		}
		title := filepath.Base(strings.TrimSuffix(planPath, filepath.Ext(planPath)))
		issue, err := providers.Tracker.CreateIssue(ctx, title, plan)
		if err != nil {
			return err
		}
		rs.IssueKey = issue.Key
		rs.IssueURL = issue.URL
		logger.Info("created issue", "key", issue.Key, "url", issue.URL)
		return nil
	}); err != nil {
		lastErr = err
		return err
	}

	// Step 2: Generate branch name.
	if err := runStep(rs, 2, logger, func() error {
		branch = BranchName(planPath)
		rs.Branch = branch
		logger.Info("generated branch name", "branch", branch)
		return nil
	}); err != nil {
		lastErr = err
		return err
	}

	// Step 3: Create worktree.
	worktreeWasCompleted := rs.Steps[3].Status == state.StepCompleted
	if err := runStep(rs, 3, logger, func() error {
		path, err := providers.Worktree.Create(ctx, branch, cfg.VCS.BaseBranch)
		if err != nil {
			return err
		}
		worktreePath = path
		rs.WorktreePath = worktreePath
		return nil
	}); err != nil {
		lastErr = err
		return err
	}
	// On resume with completed worktree step, validate worktree still exists.
	if worktreeWasCompleted && worktreePath != "" {
		if _, err := os.Stat(worktreePath); err != nil {
			return fmt.Errorf("step 4 (create worktree): worktree path %q no longer exists", worktreePath)
		}
	}

	// Defer cleanup: remove worktree only on success.
	defer func() {
		if worktreePath == "" {
			return
		}
		if rs.Status == state.RunFailed {
			logger.Info("preserving worktree for resume", "path", worktreePath)
			return
		}
		if cleanupErr := providers.Worktree.Remove(ctx, worktreePath); cleanupErr != nil {
			logger.Error("worktree cleanup failed", "error", cleanupErr)
		}
	}()

	// Step 4: Run agent.
	if err := runStep(rs, 4, logger, func() error {
		return providers.Agent.Run(ctx, worktreePath, plan)
	}); err != nil {
		lastErr = err
		return err
	}

	// Step 5: Commit and push.
	if err := runStep(rs, 5, logger, func() error {
		commitMsg := fmt.Sprintf("forge: %s", branch)
		return providers.VCS.CommitAndPush(ctx, worktreePath, branch, commitMsg)
	}); err != nil {
		lastErr = err
		return err
	}

	// Step 6: Create PR.
	if err := runStep(rs, 6, logger, func() error {
		title := fmt.Sprintf("forge: %s", filepath.Base(strings.TrimSuffix(planPath, filepath.Ext(planPath))))
		pr, err := providers.VCS.CreatePR(ctx, branch, cfg.VCS.BaseBranch, title, plan)
		if err != nil {
			return err
		}
		rs.PRUrl = pr.URL
		rs.PRNumber = pr.Number
		logger.Info("created PR", "pr", pr.URL)
		return nil
	}); err != nil {
		lastErr = err
		return err
	}

	// Step 7: Notify (optional — skipped if no notifier configured).
	if err := runStep(rs, 7, logger, func() error {
		if providers.Notifier == nil {
			logger.Info("no notifier configured, skipping")
			return nil
		}
		msg := fmt.Sprintf("PR ready for review: %s", rs.PRUrl)
		if rs.IssueKey != "" {
			msg += fmt.Sprintf(" (issue: %s)", rs.IssueURL)
		}
		return providers.Notifier.Notify(ctx, msg)
	}); err != nil {
		lastErr = err
		return err
	}

	rs.Status = state.RunCompleted
	_ = rs.Save()
	return nil
}

// runStep executes fn for the given step index, skipping if already completed.
// It persists state transitions: pending → running → completed/failed.
func runStep(rs *state.RunState, idx int, logger *slog.Logger, fn func() error) error {
	step := &rs.Steps[idx]

	if step.Status == state.StepCompleted {
		logger.Info("skipping completed step", "step", step.Name)
		return nil
	}

	step.Status = state.StepRunning
	step.Error = ""
	_ = rs.Save()

	if err := fn(); err != nil {
		step.Status = state.StepFailed
		step.Error = err.Error()
		rs.Status = state.RunFailed
		_ = rs.Save()
		return fmt.Errorf("step %d (%s): %w", idx+1, step.Name, err)
	}

	step.Status = state.StepCompleted
	_ = rs.Save()
	return nil
}

var nonAlphanumeric = regexp.MustCompile(`[^a-z0-9-]+`)

// BranchName generates a sanitized branch name from a plan file path.
func BranchName(planPath string) string {
	base := filepath.Base(planPath)
	base = strings.TrimSuffix(base, filepath.Ext(base))
	base = strings.ToLower(base)
	base = strings.ReplaceAll(base, " ", "-")
	base = nonAlphanumeric.ReplaceAllString(base, "")
	base = strings.Trim(base, "-")

	if base == "" {
		base = "unnamed"
	}

	return "forge/" + base
}
