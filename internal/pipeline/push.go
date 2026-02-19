package pipeline

import (
	"context"
	"fmt"
	"log/slog"
	"os/exec"
	"regexp"
	"strings"
	"time"

	"github.com/shahar-caura/forge/internal/config"
	"github.com/shahar-caura/forge/internal/state"
)

// PushOpts holds the options for a push pipeline run.
type PushOpts struct {
	Title   string // PR title (flag or inferred)
	Message string // PR body (flag or inferred)
	Dir     string // working directory (cwd)
	Branch  string // current branch
}

// Push executes the push pipeline: create issue → commit+push → PR → notify.
// Steps that don't apply to push mode are auto-completed.
func Push(ctx context.Context, cfg *config.Config, providers Providers, opts PushOpts, rs *state.RunState, logger *slog.Logger) error {
	var lastErr error

	defer func() {
		if rs.Status != state.RunCompleted {
			rs.Status = state.RunFailed
			_ = rs.Save()

			if providers.Notifier != nil && lastErr != nil {
				failMsg := fmt.Sprintf("forge push failed: %s", lastErr)
				_ = providers.Notifier.Notify(ctx, failMsg)
			}
		}
	}()

	title := opts.Title
	body := opts.Message

	// Step 0: Read plan — auto-complete (title from flag/branch).
	if err := runStep(rs, 0, logger, func() error {
		if title == "" {
			title = TitleFromBranch(opts.Branch)
		}
		rs.PlanTitle = title
		return nil
	}); err != nil {
		lastErr = err
		return err
	}
	// Restore title on resume.
	if title == "" {
		title = rs.PlanTitle
	}

	displayTitle := title

	// Step 1: Create issue (optional).
	if err := runStep(rs, 1, logger, func() error {
		if providers.Tracker == nil {
			logger.Info("no tracker configured, skipping")
			return nil
		}
		issue, err := providers.Tracker.CreateIssue(ctx, displayTitle, body)
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

	// Step 2: Generate branch — auto-complete (use current branch).
	if err := runStep(rs, 2, logger, func() error {
		rs.Branch = opts.Branch
		logger.Info("using current branch", "branch", opts.Branch)
		return nil
	}); err != nil {
		lastErr = err
		return err
	}

	// Step 3: Create worktree — auto-complete (use cwd).
	if err := runStep(rs, 3, logger, func() error {
		rs.WorktreePath = opts.Dir
		logger.Info("using current directory", "dir", opts.Dir)
		return nil
	}); err != nil {
		lastErr = err
		return err
	}

	// Step 4: Run agent — auto-complete (no agent for push).
	if err := runStep(rs, 4, logger, func() error {
		logger.Info("no agent for push mode, skipping")
		return nil
	}); err != nil {
		lastErr = err
		return err
	}

	// Step 5: Commit and push.
	if err := runStep(rs, 5, logger, func() error {
		hasChanges, err := providers.VCS.HasChanges(ctx, opts.Dir)
		if err != nil {
			return fmt.Errorf("checking for changes: %w", err)
		}
		if hasChanges {
			if cfg.Hooks.PreCommit != "" {
				if err := runHook(ctx, cfg.Hooks.PreCommit, opts.Dir, logger); err != nil {
					return fmt.Errorf("pre-commit hook: %w", err)
				}
			}
			commitMsg := fmt.Sprintf("forge: %s", displayTitle)
			return providers.VCS.CommitAndPush(ctx, opts.Dir, opts.Branch, commitMsg)
		}
		// No uncommitted changes — just push existing commits.
		return providers.VCS.Push(ctx, opts.Dir, opts.Branch)
	}); err != nil {
		lastErr = err
		return err
	}

	// Infer body from commit log if not provided.
	if body == "" {
		body = commitLogSummary(opts.Dir, cfg.VCS.BaseBranch)
	}

	// Step 6: Create PR.
	if err := runStep(rs, 6, logger, func() error {
		pr, err := providers.VCS.CreatePR(ctx, opts.Branch, cfg.VCS.BaseBranch, displayTitle, body)
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

	// Step 7: Poll CR (optional — skipped if CR not enabled).
	if err := runStep(rs, 7, logger, func() error {
		if !cfg.CR.Enabled {
			logger.Info("CR feedback loop disabled, skipping")
			return nil
		}
		logger.Info("polling for CR comment...", "pattern", cfg.CR.CommentPattern, "timeout", cfg.CR.PollTimeout.Duration)

		pattern, err := regexp.Compile(cfg.CR.CommentPattern)
		if err != nil {
			return fmt.Errorf("compiling comment_pattern: %w", err)
		}

		deadline := time.Now().Add(cfg.CR.PollTimeout.Duration)
		for {
			if time.Now().After(deadline) {
				return fmt.Errorf("poll timeout: no matching CR comment found after %s", cfg.CR.PollTimeout.Duration)
			}

			comments, err := providers.VCS.GetPRComments(ctx, rs.PRNumber)
			if err != nil {
				return fmt.Errorf("fetching PR comments: %w", err)
			}

			for _, c := range comments {
				if pattern.MatchString(c.Body) {
					logger.Info("matched CR comment", "author", c.Author, "id", c.ID)
					rs.CRFeedback = c.Body
					return nil
				}
			}

			select {
			case <-ctx.Done():
				return ctx.Err()
			case <-time.After(cfg.CR.PollInterval.Duration):
				// continue polling
			}
		}
	}); err != nil {
		lastErr = err
		return err
	}

	// Step 8: Fix CR — auto-complete (no agent for push; feedback logged for user).
	if err := runStep(rs, 8, logger, func() error {
		if !cfg.CR.Enabled {
			logger.Info("CR feedback loop disabled, skipping")
			return nil
		}
		logger.Info("CR feedback received; no agent in push mode — fix manually or re-run forge push", "feedback", rs.CRFeedback)
		return nil
	}); err != nil {
		lastErr = err
		return err
	}

	// Step 9: Push CR fix — auto-complete (user fixes manually).
	if err := runStep(rs, 9, logger, func() error {
		if !cfg.CR.Enabled {
			logger.Info("CR feedback loop disabled, skipping")
			return nil
		}
		logger.Info("no auto-fix in push mode, skipping")
		return nil
	}); err != nil {
		lastErr = err
		return err
	}

	// Step 10: Notify (optional).
	if err := runStep(rs, 10, logger, func() error {
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

var branchPrefixRe = regexp.MustCompile(`^(?:forge/|[A-Z]+-\d+-?)`)

// TitleFromBranch converts a branch name into a title string.
// Strips "forge/" prefix or "PROJ-123-" issue prefixes, then title-cases the rest.
func TitleFromBranch(branch string) string {
	name := branchPrefixRe.ReplaceAllString(branch, "")
	if name == "" {
		name = branch
	}
	return TitleFromFilename(name)
}

// commitLogSummary returns `git log --oneline <base>..HEAD` output for use as PR body.
func commitLogSummary(dir, baseBranch string) string {
	cmd := exec.Command("git", "log", "--oneline", baseBranch+"..HEAD")
	cmd.Dir = dir
	out, err := cmd.Output()
	if err != nil {
		return ""
	}
	return strings.TrimSpace(string(out))
}
