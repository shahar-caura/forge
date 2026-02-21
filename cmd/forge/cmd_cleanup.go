package main

import (
	"context"
	"fmt"
	"log/slog"
	"os"
	"os/signal"
	"path/filepath"

	"github.com/shahar-caura/forge/internal/config"
	"github.com/shahar-caura/forge/internal/pipeline"
	"github.com/shahar-caura/forge/internal/provider/vcs"
	"github.com/shahar-caura/forge/internal/provider/worktree"
	"github.com/spf13/cobra"
)

func newCleanupCmd(logger *slog.Logger) *cobra.Command {
	return &cobra.Command{
		Use:   "cleanup",
		Short: "Remove worktrees for merged PRs",
		RunE: func(cmd *cobra.Command, args []string) error {
			cfg, err := config.Load("forge.yaml")
			if err != nil {
				return fmt.Errorf("loading config: %w", err)
			}

			repoRoot, err := filepath.Abs(".")
			if err != nil {
				return fmt.Errorf("resolving repo root: %w", err)
			}

			v := vcs.New(cfg.VCS.Repo, logger)
			wt := worktree.New(
				cfg.Worktree.CreateCmd,
				cfg.Worktree.RemoveCmd,
				true, // force cleanup regardless of config flag
				repoRoot,
				logger,
			)

			ctx, stop := signal.NotifyContext(context.Background(), os.Interrupt)
			defer stop()

			cleaned, err := pipeline.CleanupMergedWorktrees(ctx, v, wt, logger)
			if err != nil {
				return err
			}

			if cleaned == 0 {
				logger.Info("no merged worktrees to clean up")
			} else {
				logger.Info("cleaned up merged worktrees", "count", cleaned)
			}
			return nil
		},
	}
}
