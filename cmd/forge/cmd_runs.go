package main

import (
	"fmt"
	"log/slog"

	"github.com/shahar-caura/forge/internal/state"
	"github.com/spf13/cobra"
)

func newRunsCmd(logger *slog.Logger) *cobra.Command {
	return &cobra.Command{
		Use:   "runs",
		Short: "List all runs",
		Args:  cobra.NoArgs,
		RunE: func(cmd *cobra.Command, args []string) error {
			return cmdRuns(logger)
		},
	}
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
