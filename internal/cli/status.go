package cli

import (
	"errors"

	"github.com/spf13/cobra"
	"github.com/stichting-Cyberbrein-nl/ctfdevkit-cli/internal/docker"
	"github.com/stichting-Cyberbrein-nl/ctfdevkit-cli/internal/output"
	"github.com/stichting-Cyberbrein-nl/ctfdevkit-cli/internal/payload"
	"github.com/stichting-Cyberbrein-nl/ctfdevkit-cli/internal/webapp"
)

func newStatusCmd() *cobra.Command {
	return &cobra.Command{
		Use:   "status",
		Short: "Show container status and run a health check",
		RunE: func(cmd *cobra.Command, args []string) error {
			ctx := cmd.Context()
			cfg := configFrom(ctx)
			s := stateFrom(ctx)

			composeDir, err := payload.ComposeDir(s)
			if err != nil {
				return err
			}

			output.Header("Container Status")
			if err := docker.ComposePS(ctx, composeDir); err != nil {
				output.Warnf("Could not get container status: %v", err)
			}

			output.Header("Health Check")
			report, err := webapp.New(composeDir, cfg.AppContainer).Doctor(ctx, true)
			var cmdErr *webapp.CommandError
			if err != nil && !errors.As(err, &cmdErr) {
				output.Failf("Health check failed: %v", err)
				return nil
			}
			renderDoctor(report)

			return nil
		},
	}
}
