package main

import (
	"context"
	"fmt"
	"log/slog"
	"os"
	"os/signal"
	"strings"

	"github.com/shahar-caura/forge/internal/config"
	"github.com/shahar-caura/forge/internal/pipeline"
	"github.com/shahar-caura/forge/internal/state"
	"github.com/spf13/cobra"
)

func newResumeCmd(logger *slog.Logger) *cobra.Command {
	var fromStep string

	cmd := &cobra.Command{
		Use:   "resume <run-id>",
		Short: "Resume a previous run",
		Args:  cobra.ExactArgs(1),
		ValidArgsFunction: func(cmd *cobra.Command, args []string, toComplete string) ([]string, cobra.ShellCompDirective) {
			return completeRunIDs(toComplete)
		},
		RunE: func(cmd *cobra.Command, args []string) error {
			return cmdResume(logger, args[0], fromStep)
		},
	}

	cmd.Flags().StringVar(&fromStep, "from", "", "step name to resume from")
	_ = cmd.RegisterFlagCompletionFunc("from", func(cmd *cobra.Command, args []string, toComplete string) ([]string, cobra.ShellCompDirective) {
		return completeStepNames(toComplete)
	})

	return cmd
}

func cmdResume(logger *slog.Logger, runID, fromStep string) error {
	rs, err := state.Load(runID)
	if err != nil {
		return fmt.Errorf("loading run state: %w", err)
	}

	// Plan file check only applies to run mode (push mode may have no plan).
	if rs.Mode != "push" {
		if _, err := os.Stat(rs.PlanPath); err != nil {
			return fmt.Errorf("plan file %q: %w", rs.PlanPath, err)
		}
	}

	if fromStep != "" {
		idx, ok := state.StepIndex(fromStep)
		if !ok {
			return fmt.Errorf("unknown step %q; valid steps: %s", fromStep, strings.Join(state.StepNames, ", "))
		}
		rs.ResetFrom(idx)
		logger.Info("resuming from step", "step", state.StepNames[idx])
	} else {
		if rs.Status == state.RunCompleted {
			return fmt.Errorf("run %q already completed; use --from <step> to re-run from a specific step", runID)
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
	}

	cfg, err := config.Load("forge.yaml")
	if err != nil {
		return fmt.Errorf("loading config: %w", err)
	}

	logger.Info("resuming run", "id", runID, "mode", rs.Mode)

	ctx, stop := signal.NotifyContext(context.Background(), os.Interrupt)
	defer stop()

	var pipelineErr error

	if rs.Mode == "push" {
		providers, err := wirePushProviders(cfg, logger)
		if err != nil {
			return err
		}
		cwd, err := os.Getwd()
		if err != nil {
			return fmt.Errorf("getting working directory: %w", err)
		}
		opts := pipeline.PushOpts{
			Dir:    cwd,
			Branch: rs.Branch,
		}
		pipelineErr = pipeline.Push(ctx, cfg, providers, opts, rs, logger)
	} else {
		providers, err := wireProviders(cfg, logger)
		if err != nil {
			return err
		}
		pipelineErr = pipeline.Run(ctx, cfg, providers, rs.PlanPath, rs, logger)
	}

	// Best-effort cleanup.
	if deleted, err := state.Cleanup(cfg.State.Retention.Duration); err != nil {
		logger.Warn("state cleanup failed", "error", err)
	} else if deleted > 0 {
		logger.Info("cleaned up old run states", "deleted", deleted)
	}

	return pipelineErr
}
