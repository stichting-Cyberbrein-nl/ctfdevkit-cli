package cli

import (
	"github.com/spf13/cobra"
	"github.com/stichting-Cyberbrein-nl/ctfdevkit-cli/internal/docker"
	"github.com/stichting-Cyberbrein-nl/ctfdevkit-cli/internal/output"
	"github.com/stichting-Cyberbrein-nl/ctfdevkit-cli/internal/payload"
)

func newDownCmd() *cobra.Command {
	return &cobra.Command{
		Use:   "down",
		Short: "Stop and remove containers",
		RunE: func(cmd *cobra.Command, args []string) error {
			ctx := cmd.Context()
			s := stateFrom(ctx)

			composeDir, err := payload.ComposeDir(s)
			if err != nil {
				return err
			}

			output.Info("Stopping containers...")
			if err := docker.ComposeDown(ctx, composeDir); err != nil {
				return err
			}
			output.Success("Containers stopped")
			return nil
		},
	}
}
