package main

import (
	"fmt"
	"io"
	"os"
	"os/exec"

	"github.com/shahar-caura/forge/internal/pipeline"
	"github.com/spf13/cobra"
)

func newLogsCmd() *cobra.Command {
	var (
		follow bool
		step   int
	)

	cmd := &cobra.Command{
		Use:   "logs <run-id>",
		Short: "Show logs for a run",
		Args:  cobra.ExactArgs(1),
		ValidArgsFunction: func(cmd *cobra.Command, args []string, toComplete string) ([]string, cobra.ShellCompDirective) {
			return completeRunIDs(toComplete)
		},
		RunE: func(cmd *cobra.Command, args []string) error {
			return cmdLogs(args[0], follow, step)
		},
	}

	cmd.Flags().BoolVarP(&follow, "follow", "f", false, "follow log output")
	cmd.Flags().IntVar(&step, "step", 4, "step number to show logs for")

	return cmd
}

func cmdLogs(runID string, follow bool, step int) error {
	logPath := pipeline.AgentLogPath(runID, step)

	if follow {
		cmd := exec.Command("tail", "-f", logPath)
		cmd.Stdout = os.Stdout
		cmd.Stderr = os.Stderr
		return cmd.Run()
	}

	f, err := os.Open(logPath)
	if err != nil {
		return fmt.Errorf("opening log: %w", err)
	}
	defer func() { _ = f.Close() }()

	_, err = io.Copy(os.Stdout, f)
	return err
}
