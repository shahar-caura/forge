package main

import (
	"context"
	"fmt"
	"log/slog"
	"os"
	"os/exec"
	"path/filepath"
	"strings"

	"github.com/shahar-caura/forge/internal/config"
	"github.com/shahar-caura/forge/internal/provider/vcs"
	"github.com/shahar-caura/forge/internal/provider/worktree"
	"github.com/shahar-caura/forge/internal/state"
	"github.com/spf13/cobra"
)

func newEditCmd(logger *slog.Logger) *cobra.Command {
	var (
		push    bool
		message string
	)

	cmd := &cobra.Command{
		Use:   "edit <run-id>",
		Short: "Open a worktree for manual editing",
		Args:  cobra.ExactArgs(1),
		ValidArgsFunction: func(cmd *cobra.Command, args []string, toComplete string) ([]string, cobra.ShellCompDirective) {
			return completeRunIDs(toComplete)
		},
		RunE: func(cmd *cobra.Command, args []string) error {
			return cmdEdit(logger, args[0], push, message)
		},
	}

	cmd.Flags().BoolVar(&push, "push", false, "commit and push changes")
	cmd.Flags().StringVarP(&message, "message", "m", "", "commit message (required with --push)")

	return cmd
}

func cmdEdit(logger *slog.Logger, runID string, push bool, message string) error {
	rs, err := state.Load(runID)
	if err != nil {
		return fmt.Errorf("loading run state: %w", err)
	}

	if rs.Branch == "" {
		return fmt.Errorf("run %q has no branch yet (step 'generate branch' not completed)", runID)
	}

	cfg, err := config.Load("forge.yaml")
	if err != nil {
		return fmt.Errorf("loading config: %w", err)
	}

	wtPath := rs.WorktreePath

	// If worktree path is empty or directory doesn't exist, re-create it.
	if wtPath == "" || !dirExists(wtPath) {
		repoRoot, err := filepath.Abs(".")
		if err != nil {
			return fmt.Errorf("resolving repo root: %w", err)
		}

		wt := worktree.New(
			cfg.Worktree.CreateCmd,
			cfg.Worktree.RemoveCmd,
			cfg.Worktree.Cleanup,
			repoRoot,
			logger,
		)

		wtPath, err = wt.Create(context.Background(), rs.Branch, cfg.VCS.BaseBranch)
		if err != nil {
			return fmt.Errorf("re-creating worktree: %w", err)
		}

		rs.WorktreePath = wtPath
		if err := rs.Save(); err != nil {
			return fmt.Errorf("saving run state: %w", err)
		}
	}

	// Fast-forward local branch to match remote (agent/CR may have pushed).
	// Skip if worktree is mid-rebase/merge — pull would fail anyway.
	if detectDirtyGitState(wtPath) == "" {
		pull := exec.Command("git", "fetch", "origin", rs.Branch)
		pull.Dir = wtPath
		if _, err := pull.CombinedOutput(); err == nil {
			merge := exec.Command("git", "merge", "--ff-only", "origin/"+rs.Branch)
			merge.Dir = wtPath
			if out, err := merge.CombinedOutput(); err != nil {
				logger.Warn("fast-forward failed (may have local changes)", "error", err, "output", strings.TrimSpace(string(out)))
			}
		}
	}

	if push {
		if message == "" {
			return fmt.Errorf("--push requires -m \"description of changes\"")
		}
		return editPush(cfg, rs, wtPath, message, logger)
	}

	fmt.Println(wtPath)

	if cfg.Editor.Enabled {
		cmd := exec.Command(cfg.Editor.Command, wtPath)
		cmd.Stdout = os.Stdout
		cmd.Stderr = os.Stderr
		if err := cmd.Run(); err != nil {
			return fmt.Errorf("opening editor: %w", err)
		}
	}

	fmt.Fprintf(os.Stderr, "\nRun 'forge edit %s --push -m \"description\"' to commit and update the PR.\n", runID)
	removeHint := strings.Replace(cfg.Worktree.RemoveCmd, "{{.Path}}", wtPath, 1)
	fmt.Fprintf(os.Stderr, "Run '%s' to exit edit mode.\n", removeHint)
	return nil
}

func editPush(cfg *config.Config, rs *state.RunState, wtPath, message string, logger *slog.Logger) error {
	// Fail fast if worktree is mid-rebase or mid-merge.
	if reason := detectDirtyGitState(wtPath); reason != "" {
		return fmt.Errorf("worktree has an unfinished %s; resolve it before pushing", reason)
	}

	ctx := context.Background()
	v := vcs.New(cfg.VCS.Repo, logger)

	hasChanges, err := v.HasChanges(ctx, wtPath)
	if err != nil {
		return fmt.Errorf("checking for changes: %w", err)
	}

	if !hasChanges {
		if hasUnpushedCommits(wtPath, rs.Branch) {
			fmt.Fprintf(os.Stderr, "Branch has diverged from remote (rebase?). Push manually:\n\n  git -C %s push --force-with-lease origin %s\n\n", wtPath, rs.Branch)
			return fmt.Errorf("no uncommitted changes but branch has unpushed commits")
		}
		return fmt.Errorf("no changes to push in %s", wtPath)
	}

	// Stage all changes.
	addCmd := exec.Command("git", "add", ".")
	addCmd.Dir = wtPath
	if out, err := addCmd.CombinedOutput(); err != nil {
		return fmt.Errorf("git add: %w: %s", err, strings.TrimSpace(string(out)))
	}

	// Commit based on strategy (push comes after rebase).
	switch cfg.CR.FixStrategy {
	case "new-commit":
		msg := fmt.Sprintf("forge: %s", message)
		commitCmd := exec.Command("git", "commit", "-m", msg)
		commitCmd.Dir = wtPath
		if out, err := commitCmd.CombinedOutput(); err != nil {
			return fmt.Errorf("git commit: %w: %s", err, strings.TrimSpace(string(out)))
		}
	default: // "amend"
		// Append -m message to existing commit message before amending.
		logCmd := exec.Command("git", "log", "-1", "--format=%B")
		logCmd.Dir = wtPath
		existing, err := logCmd.Output()
		if err != nil {
			return fmt.Errorf("reading commit message: %w", err)
		}
		newMsg := strings.TrimSpace(string(existing)) + "\n\n" + message
		commitCmd := exec.Command("git", "commit", "--amend", "-m", newMsg)
		commitCmd.Dir = wtPath
		if out, err := commitCmd.CombinedOutput(); err != nil {
			return fmt.Errorf("git commit --amend: %w: %s", err, strings.TrimSpace(string(out)))
		}
	}

	// Fetch and rebase onto latest base branch before pushing.
	if err := v.FetchAndRebase(ctx, wtPath, cfg.VCS.BaseBranch); err != nil {
		return fmt.Errorf("rebase onto %s: %w", cfg.VCS.BaseBranch, err)
	}

	// Force push (rebase may rewrite history).
	pushCmd := exec.Command("git", "push", "--force-with-lease", "origin", rs.Branch)
	pushCmd.Dir = wtPath
	if out, err := pushCmd.CombinedOutput(); err != nil {
		return fmt.Errorf("git push: %w: %s", err, strings.TrimSpace(string(out)))
	}

	if rs.PRNumber != 0 {
		comment := fmt.Sprintf("Manual edit: %s", message)
		if err := v.PostPRComment(ctx, rs.PRNumber, comment); err != nil {
			logger.Warn("failed to post PR comment", "error", err)
		}
	}

	if rs.PRUrl != "" {
		fmt.Println(rs.PRUrl)
	}

	if cfg.Worktree.Cleanup {
		rm := exec.Command("git", "worktree", "remove", "--force", wtPath)
		if out, err := rm.CombinedOutput(); err != nil {
			logger.Warn("worktree cleanup failed", "error", err, "output", strings.TrimSpace(string(out)))
		} else {
			logger.Info("worktree removed", "path", wtPath)
		}
	}

	return nil
}
