package cli

import (
	"github.com/spf13/cobra"
	"github.com/stichting-Cyberbrein-nl/ctfdevkit-cli/internal/docker"
	"github.com/stichting-Cyberbrein-nl/ctfdevkit-cli/internal/output"
	"github.com/stichting-Cyberbrein-nl/ctfdevkit-cli/internal/payload"
)

func newResetCmd() *cobra.Command {
	return &cobra.Command{
		Use:   "reset",
		Short: "Stop containers and remove all volumes (destructive)",
		RunE: func(cmd *cobra.Command, args []string) error {
			ctx := cmd.Context()
			s := stateFrom(ctx)

			composeDir, err := payload.ComposeDir(s)
			if err != nil {
				return err
			}

			output.Warn("This will destroy all container data including the database.")
			output.Info("Resetting environment...")
			if err := docker.ComposeReset(ctx, composeDir); err != nil {
				return err
			}
			output.Success("Environment reset — run `devkit up` to start fresh")
			return nil
		},
	}
}
