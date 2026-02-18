package main

import (
	"context"
	"fmt"
	"log/slog"
	"os"
	"os/signal"
	"path/filepath"
	"strings"
	"time"

	"github.com/shahar-caura/forge/internal/config"
	"github.com/shahar-caura/forge/internal/pipeline"
	"github.com/shahar-caura/forge/internal/provider/agent"
	"github.com/shahar-caura/forge/internal/provider/notifier"
	"github.com/shahar-caura/forge/internal/provider/tracker"
	"github.com/shahar-caura/forge/internal/provider/vcs"
	"github.com/shahar-caura/forge/internal/provider/worktree"
	"github.com/shahar-caura/forge/internal/state"
)

func main() {
	logger := slog.New(slog.NewTextHandler(os.Stderr, nil))

	if err := run(logger); err != nil {
		logger.Error("forge failed", "error", err)
		os.Exit(1)
	}
}

func run(logger *slog.Logger) error {
	if len(os.Args) < 2 {
		return fmt.Errorf("usage: forge <run|resume|runs>")
	}

	switch os.Args[1] {
	case "run":
		return cmdRun(logger)
	case "resume":
		return cmdResume(logger)
	case "runs":
		return cmdRuns(logger)
	default:
		return fmt.Errorf("usage: forge <run|resume|runs>")
	}
}

func cmdRun(logger *slog.Logger) error {
	if len(os.Args) < 3 {
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

	// Generate run ID: YYYYMMDD-HHMMSS-<slug>
	slug := pipeline.SlugFromTitle(filepath.Base(strings.TrimSuffix(planPath, filepath.Ext(planPath))))
	runID := time.Now().Format("20060102-150405") + "-" + slug

	rs := state.New(runID, planPath)
	if err := rs.Save(); err != nil {
		return fmt.Errorf("saving initial run state: %w", err)
	}

	logger.Info("starting run", "id", runID, "plan", planPath)

	providers, err := wireProviders(cfg, logger)
	if err != nil {
		return err
	}

	ctx, stop := signal.NotifyContext(context.Background(), os.Interrupt)
	defer stop()

	pipelineErr := pipeline.Run(ctx, cfg, providers, planPath, rs, logger)

	// Best-effort cleanup of old completed runs.
	if deleted, err := state.Cleanup(cfg.State.Retention.Duration); err != nil {
		logger.Warn("state cleanup failed", "error", err)
	} else if deleted > 0 {
		logger.Info("cleaned up old run states", "deleted", deleted)
	}

	return pipelineErr
}

func cmdResume(logger *slog.Logger) error {
	if len(os.Args) < 3 {
		return fmt.Errorf("usage: forge resume <run-id>")
	}

	runID := os.Args[2]

	rs, err := state.Load(runID)
	if err != nil {
		return fmt.Errorf("loading run state: %w", err)
	}

	if rs.Status == state.RunCompleted {
		return fmt.Errorf("run %q already completed", runID)
	}

	if _, err := os.Stat(rs.PlanPath); err != nil {
		return fmt.Errorf("plan file %q: %w", rs.PlanPath, err)
	}

	// Reset failed status to active for re-run.
	rs.Status = state.RunActive
	// Reset any failed steps to pending so they get re-run.
	for i := range rs.Steps {
		if rs.Steps[i].Status == state.StepFailed {
			rs.Steps[i].Status = state.StepPending
			rs.Steps[i].Error = ""
		}
	}

	cfg, err := config.Load("forge.yaml")
	if err != nil {
		return fmt.Errorf("loading config: %w", err)
	}

	logger.Info("resuming run", "id", runID, "plan", rs.PlanPath)

	providers, err := wireProviders(cfg, logger)
	if err != nil {
		return err
	}

	ctx, stop := signal.NotifyContext(context.Background(), os.Interrupt)
	defer stop()

	pipelineErr := pipeline.Run(ctx, cfg, providers, rs.PlanPath, rs, logger)

	// Best-effort cleanup.
	if deleted, err := state.Cleanup(cfg.State.Retention.Duration); err != nil {
		logger.Warn("state cleanup failed", "error", err)
	} else if deleted > 0 {
		logger.Info("cleaned up old run states", "deleted", deleted)
	}

	return pipelineErr
}

func cmdRuns(logger *slog.Logger) error {
	runs, err := state.List()
	if err != nil {
		return fmt.Errorf("listing runs: %w", err)
	}

	if len(runs) == 0 {
		fmt.Println("No runs found.")
		return nil
	}

	fmt.Printf("%-30s  %-10s  %-20s  %s\n", "ID", "STATUS", "CREATED", "PLAN")
	for _, r := range runs {
		fmt.Printf("%-30s  %-10s  %-20s  %s\n",
			r.ID,
			r.Status,
			r.CreatedAt.Format("2006-01-02 15:04:05"),
			r.PlanPath,
		)
	}

	_ = logger // unused but kept for consistency
	return nil
}

func wireProviders(cfg *config.Config, logger *slog.Logger) (pipeline.Providers, error) {
	repoRoot, err := filepath.Abs(".")
	if err != nil {
		return pipeline.Providers{}, fmt.Errorf("resolving repo root: %w", err)
	}

	p := pipeline.Providers{
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

	if cfg.Tracker.Provider != "" {
		p.Tracker = tracker.New(cfg.Tracker.BaseURL, cfg.Tracker.Project, cfg.Tracker.Email, cfg.Tracker.Token, cfg.Tracker.BoardID)
	}

	if cfg.Notifier.Provider != "" {
		p.Notifier = notifier.New(cfg.Notifier.WebhookURL)
	}

	return p, nil
}
