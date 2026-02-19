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
	"github.com/shahar-caura/forge/internal/state"
	"github.com/spf13/cobra"
)

func newRunCmd(logger *slog.Logger) *cobra.Command {
	var issueNumber int

	cmd := &cobra.Command{
		Use:   "run [plan.md]",
		Short: "Execute a plan file or GitHub issue",
		Args:  cobra.MaximumNArgs(1),
		ValidArgsFunction: func(cmd *cobra.Command, args []string, toComplete string) ([]string, cobra.ShellCompDirective) {
			return []string{"md"}, cobra.ShellCompDirectiveFilterFileExt
		},
		RunE: func(cmd *cobra.Command, args []string) error {
			hasPlan := len(args) == 1
			hasIssue := issueNumber > 0

			if hasPlan && hasIssue {
				return fmt.Errorf("cannot specify both a plan file and --issue")
			}
			if !hasPlan && !hasIssue {
				return fmt.Errorf("provide a plan file argument or --issue flag")
			}

			var planPath string
			if hasPlan {
				planPath = args[0]
			}
			return cmdRun(logger, planPath, issueNumber)
		},
	}

	cmd.Flags().IntVar(&issueNumber, "issue", 0, "GitHub issue number to use as plan")
	return cmd
}

func cmdRun(logger *slog.Logger, planPath string, issueNumber int) error {
	cfg, err := config.Load("forge.yaml")
	if err != nil {
		return fmt.Errorf("loading config: %w", err)
	}

	// Wire providers early — needed for --issue fetch before run ID generation.
	providers, err := wireProviders(cfg, logger)
	if err != nil {
		return err
	}

	ctx, stop := signal.NotifyContext(context.Background(), os.Interrupt)
	defer stop()

	// If --issue is set, fetch the issue and write a temp plan file.
	if issueNumber > 0 {
		issue, err := providers.VCS.GetIssue(ctx, issueNumber)
		if err != nil {
			return fmt.Errorf("fetching issue #%d: %w", issueNumber, err)
		}

		slug := pipeline.SlugFromTitle(issue.Title)
		runID := time.Now().Format("20060102-150405") + "-" + slug

		// Write temp plan file so the pipeline's Step 0 (read plan) works unchanged.
		if err := os.MkdirAll(".forge/runs", 0o755); err != nil {
			return fmt.Errorf("creating runs dir: %w", err)
		}
		planPath = filepath.Join(".forge/runs", runID+"-plan.md")
		planContent := fmt.Sprintf("---\ntitle: %q\n---\n%s\n", issue.Title, issue.Body)
		if err := os.WriteFile(planPath, []byte(planContent), 0o644); err != nil {
			return fmt.Errorf("writing temp plan: %w", err)
		}

		rs := state.New(runID, planPath)
		rs.SourceIssue = issueNumber
		if err := rs.Save(); err != nil {
			return fmt.Errorf("saving initial run state: %w", err)
		}

		logger.Info("starting run from issue", "id", runID, "issue", issueNumber, "title", issue.Title)

		pipelineErr := pipeline.Run(ctx, cfg, providers, planPath, rs, logger)
		cleanupOldRuns(cfg, logger)
		return pipelineErr
	}

	// File-based plan path.
	if _, err := os.Stat(planPath); err != nil {
		return fmt.Errorf("plan file: %w", err)
	}

	slug := pipeline.SlugFromTitle(filepath.Base(strings.TrimSuffix(planPath, filepath.Ext(planPath))))
	runID := time.Now().Format("20060102-150405") + "-" + slug

	rs := state.New(runID, planPath)
	if err := rs.Save(); err != nil {
		return fmt.Errorf("saving initial run state: %w", err)
	}

	logger.Info("starting run", "id", runID, "plan", planPath)

	pipelineErr := pipeline.Run(ctx, cfg, providers, planPath, rs, logger)
	cleanupOldRuns(cfg, logger)
	return pipelineErr
}

func cleanupOldRuns(cfg *config.Config, logger *slog.Logger) {
	if deleted, err := state.Cleanup(cfg.State.Retention.Duration); err != nil {
		logger.Warn("state cleanup failed", "error", err)
	} else if deleted > 0 {
		logger.Info("cleaned up old run states", "deleted", deleted)
	}
}
