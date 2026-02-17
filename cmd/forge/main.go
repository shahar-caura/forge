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
	"github.com/shahar-caura/forge/internal/provider/agent"
	"github.com/shahar-caura/forge/internal/provider/vcs"
	"github.com/shahar-caura/forge/internal/provider/worktree"
)

func main() {
	logger := slog.New(slog.NewTextHandler(os.Stderr, nil))

	if err := run(logger); err != nil {
		logger.Error("forge failed", "error", err)
		os.Exit(1)
	}
}

func run(logger *slog.Logger) error {
	if len(os.Args) < 3 || os.Args[1] != "run" {
		return fmt.Errorf("usage: forge run <plan.md>")
	}

	planPath := os.Args[2]

	if _, err := os.Stat(planPath); err != nil {
		return fmt.Errorf("plan file: %w", err)
	}

	cfg, err := config.Load("forge.yaml")
	if err != nil {
		return fmt.Errorf("loading config: %w", err)
	}

	repoRoot, err := filepath.Abs(".")
	if err != nil {
		return fmt.Errorf("resolving repo root: %w", err)
	}

	providers := pipeline.Providers{
		Worktree: worktree.New(
			cfg.Worktree.CreateCmd,
			cfg.Worktree.RemoveCmd,
			cfg.Worktree.Cleanup,
			repoRoot,
			logger,
		),
		Agent: agent.New(cfg.Agent.Timeout.Duration, logger),
		VCS:   vcs.New(cfg.VCS.Repo, logger),
	}

	ctx, stop := signal.NotifyContext(context.Background(), os.Interrupt)
	defer stop()

	return pipeline.Run(ctx, cfg, providers, planPath, logger)
}
