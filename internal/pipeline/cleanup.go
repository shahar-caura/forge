package pipeline

import (
	"context"
	"log/slog"

	"github.com/shahar-caura/forge/internal/provider"
	"github.com/shahar-caura/forge/internal/state"
)

// CleanupMergedWorktrees removes worktrees whose associated PRs have been merged.
// It scans all run states, checks PR status via the VCS provider, and removes
// worktrees for merged PRs. Returns the count of cleaned worktrees.
func CleanupMergedWorktrees(ctx context.Context, vcs provider.VCS, wt provider.Worktree, logger *slog.Logger) (int, error) {
	runs, err := state.List()
	if err != nil {
		return 0, err
	}

	cleaned := 0
	for _, rs := range runs {
		if rs.PRNumber == 0 || rs.WorktreePath == "" {
			continue
		}

		prState, err := vcs.GetPRState(ctx, rs.PRNumber)
		if err != nil {
			logger.Warn("failed to check PR state, skipping", "pr", rs.PRNumber, "error", err)
			continue
		}

		if prState != "MERGED" {
			continue
		}

		logger.Info("removing worktree for merged PR", "pr", rs.PRNumber, "path", rs.WorktreePath)
		if err := wt.Remove(ctx, rs.WorktreePath); err != nil {
			logger.Warn("failed to remove worktree", "path", rs.WorktreePath, "error", err)
			continue
		}

		rs.WorktreePath = ""
		if err := rs.Save(); err != nil {
			logger.Warn("failed to save run state after cleanup", "id", rs.ID, "error", err)
		}
		cleaned++
	}

	return cleaned, nil
}
