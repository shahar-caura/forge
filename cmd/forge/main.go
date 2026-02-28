package main

import (
	"fmt"
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
		fmt.Fprintf(os.Stderr, "Error: %s\n", err)
		os.Exit(1)
	}
}

func newRootCmd(logger *slog.Logger) *cobra.Command {
	root := &cobra.Command{
		Use:           "forge",
		Short:         "Execute a development plan end-to-end",
		SilenceUsage:  true,
		SilenceErrors: true,
		Args:          cobra.ArbitraryArgs,
		RunE: func(cmd *cobra.Command, args []string) error {
			if len(args) == 0 {
				return cmd.Help()
			}
			return runNaturalLanguage(cmd, logger, args)
		},
	}

	root.PersistentFlags().String("agent", "", "override agent provider (e.g. claude, codex, gemini)")
	_ = root.RegisterFlagCompletionFunc("agent", func(cmd *cobra.Command, args []string, toComplete string) ([]string, cobra.ShellCompDirective) {
		return []string{"claude", "codex", "gemini", "ralph"}, cobra.ShellCompDirectiveNoFileComp
	})

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
		newCompletionCmd(),
		newCleanupCmd(logger),
		newServeCmd(logger),
	)

	return root
}
