package main

import (
	"errors"
	"fmt"
	"log/slog"
	"os"
	"strings"

	"github.com/shahar-caura/forge/internal/intent"
	"github.com/spf13/cobra"
)

// nlClassifying guards against classify → execute → classify recursion.
var nlClassifying bool

func runNaturalLanguage(cmd *cobra.Command, logger *slog.Logger, args []string) error {
	if len(args) == 0 {
		return cmd.Help()
	}

	// Recursion guard: prevent classify → execute → classify loops.
	if nlClassifying {
		return fmt.Errorf("unknown command %q", args[0])
	}

	query := strings.Join(args, " ")
	logger.Info("classifying natural language input", "query", query)

	result, err := intent.Classify(cmd.Context(), query)
	if err != nil {
		if errors.Is(err, intent.ErrNoClaude) {
			return fmt.Errorf("unknown command %q (install claude CLI to enable natural language mode)", args[0])
		}
		return fmt.Errorf("could not interpret %q as a forge command: %w", query, err)
	}

	// Validate that the resolved subcommand actually exists.
	sub := result.Argv[0]
	found := false
	for _, c := range cmd.Root().Commands() {
		if c.Name() == sub {
			found = true
			break
		}
	}
	if !found {
		return fmt.Errorf("could not interpret %q as a forge command (resolved to unknown subcommand %q)", query, sub)
	}

	fmt.Fprintf(os.Stderr, "=> forge %s\n", strings.Join(result.Argv, " "))

	nlClassifying = true
	defer func() { nlClassifying = false }()

	cmd.Root().SetArgs(result.Argv)
	return cmd.Root().Execute()
}
