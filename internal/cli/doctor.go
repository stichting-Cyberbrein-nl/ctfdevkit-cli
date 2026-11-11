package cli

import (
	"errors"

	"github.com/spf13/cobra"
	"github.com/stichting-Cyberbrein-nl/ctfdevkit-cli/internal/payload"
	"github.com/stichting-Cyberbrein-nl/ctfdevkit-cli/internal/webapp"
)

func newDoctorCmd() *cobra.Command {
	return &cobra.Command{
		Use:   "doctor",
		Short: "Run the application health check",
		RunE: func(cmd *cobra.Command, args []string) error {
			ctx := cmd.Context()
			cfg := configFrom(ctx)
			s := stateFrom(ctx)

			composeDir, err := payload.ComposeDir(s)
			if err != nil {
				return err
			}

			report, err := webapp.New(composeDir, cfg.AppContainer).Doctor(ctx, true)
			var cmdErr *webapp.CommandError
			if err != nil && !errors.As(err, &cmdErr) {
				return err
			}
			renderDoctor(report)
			if !report.OK {
				return errors.New("health check failed")
			}
			return nil
		},
	}
}
