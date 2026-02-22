package main

import (
	"log/slog"
	"os"
	"os/signal"

	"github.com/shahar-caura/forge/internal/server"
	"github.com/shahar-caura/forge/internal/state"
	"github.com/spf13/cobra"
)

func newServeCmd(logger *slog.Logger) *cobra.Command {
	var port int
	var runsDir string
	var local bool

	cmd := &cobra.Command{
		Use:   "serve",
		Short: "Start the dashboard web server",
		RunE: func(cmd *cobra.Command, args []string) error {
			state.SetRunsDir(runsDir)

			ctx, stop := signal.NotifyContext(cmd.Context(), os.Interrupt)
			defer stop()

			srv := server.New(port, runsDir, version, logger)
			if !local {
				srv.SetMultiRepo(true)
			}
			return srv.Run(ctx)
		},
	}

	cmd.Flags().IntVar(&port, "port", 8080, "HTTP server port")
	cmd.Flags().StringVar(&runsDir, "runs-dir", ".forge/runs", "path to runs state directory")
	cmd.Flags().BoolVar(&local, "local", false, "only show runs from the local repo")

	return cmd
}
