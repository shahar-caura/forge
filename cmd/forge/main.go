package main

import (
	"log/slog"
	"os"

	"github.com/spf13/cobra"
)

var (
	version = "dev"
	commit  = "none"
	date    = "unknown"
)

func main() {
	logger := slog.New(slog.NewTextHandler(os.Stderr, nil))

	if err := newRootCmd(logger).Execute(); err != nil {
		os.Exit(1)
	}
}

func newRootCmd(logger *slog.Logger) *cobra.Command {
	root := &cobra.Command{
		Use:           "forge",
		Short:         "Execute a development plan end-to-end",
		SilenceUsage:  true,
		SilenceErrors: true,
	}

	root.AddCommand(
		newVersionCmd(),
		newInitCmd(),
		newRunCmd(logger),
		newPushCmd(logger),
		newResumeCmd(logger),
		newRunsCmd(logger),
		newStatusCmd(),
		newLogsCmd(),
		newStepsCmd(),
		newEditCmd(logger),
	)

	return root
}
