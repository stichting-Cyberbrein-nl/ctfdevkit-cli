package cli

import (
	"github.com/spf13/cobra"
	"github.com/stichting-Cyberbrein-nl/ctfdevkit-cli/internal/docker"
	"github.com/stichting-Cyberbrein-nl/ctfdevkit-cli/internal/payload"
)

func newLogsCmd() *cobra.Command {
	return &cobra.Command{
		Use:   "logs [service]",
		Short: "Stream container logs (optionally for a specific service)",
		Args:  cobra.MaximumNArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			ctx := cmd.Context()
			s := stateFrom(ctx)

			composeDir, err := payload.ComposeDir(s)
			if err != nil {
				return err
			}

			service := ""
			if len(args) > 0 {
				service = args[0]
			}

			return docker.ComposeLogs(ctx, composeDir, service)
		},
	}
}
