package cli

import (
	"github.com/spf13/cobra"
	"github.com/stichting-Cyberbrein-nl/ctfdevkit-cli/internal/output"
	"github.com/stichting-Cyberbrein-nl/ctfdevkit-cli/internal/ports"
)

func newReleasePortsCmd() *cobra.Command {
	return &cobra.Command{
		Use:   "release-ports",
		Short: "Force-release ports 80 and 443",
		RunE: func(cmd *cobra.Command, args []string) error {
			ctx := cmd.Context()
			plat := platformFrom(ctx)

			output.Info("Releasing ports 80 and 443...")
			if err := ports.EnsureReserved(ctx, plat); err != nil {
				return err
			}
			output.Success("Ports released")
			return nil
		},
	}
}
