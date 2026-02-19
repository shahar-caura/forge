package main

import (
	"fmt"
	"time"

	"github.com/shahar-caura/forge/internal/state"
	"github.com/spf13/cobra"
)

func newStatusCmd() *cobra.Command {
	return &cobra.Command{
		Use:   "status <run-id>",
		Short: "Show status of a run",
		Args:  cobra.ExactArgs(1),
		ValidArgsFunction: func(cmd *cobra.Command, args []string, toComplete string) ([]string, cobra.ShellCompDirective) {
			return completeRunIDs(toComplete)
		},
		RunE: func(cmd *cobra.Command, args []string) error {
			return cmdStatus(args[0])
		},
	}
}

func cmdStatus(runID string) error {
	rs, err := state.Load(runID)
	if err != nil {
		return fmt.Errorf("loading run state: %w", err)
	}

	elapsed := rs.UpdatedAt.Sub(rs.CreatedAt).Truncate(time.Second)
	if rs.Status == state.RunActive {
		elapsed = time.Since(rs.CreatedAt).Truncate(time.Second)
	}

	fmt.Printf("Run:      %s\n", rs.ID)
	fmt.Printf("Status:   %s\n", rs.Status)
	fmt.Printf("Plan:     %s\n", rs.PlanPath)
	fmt.Printf("Elapsed:  %s\n", elapsed)
	if rs.Branch != "" {
		fmt.Printf("Branch:   %s\n", rs.Branch)
	}
	if rs.PRUrl != "" {
		fmt.Printf("PR:       %s\n", rs.PRUrl)
	}
	fmt.Println()

	fmt.Printf("%-4s  %-20s  %-10s  %s\n", "STEP", "NAME", "STATUS", "ERROR")
	for i, step := range rs.Steps {
		marker := " "
		if step.Status == state.StepRunning {
			marker = ">"
		}
		errMsg := ""
		if step.Error != "" {
			errMsg = step.Error
			if len(errMsg) > 60 {
				errMsg = errMsg[:60] + "..."
			}
		}
		fmt.Printf("%s%3d  %-20s  %-10s  %s\n", marker, i, step.Name, step.Status, errMsg)
	}

	return nil
}
