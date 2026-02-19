package main

import (
	"fmt"

	"github.com/shahar-caura/forge/internal/state"
	"github.com/spf13/cobra"
)

func newStepsCmd() *cobra.Command {
	return &cobra.Command{
		Use:   "steps",
		Short: "List pipeline steps",
		Args:  cobra.NoArgs,
		RunE: func(cmd *cobra.Command, args []string) error {
			return cmdSteps()
		},
	}
}

func cmdSteps() error {
	for i, name := range state.StepNames {
		fmt.Printf("%2d  %s\n", i, name)
	}
	return nil
}
