package cli

import (
	"github.com/spf13/cobra"
	"github.com/stichting-Cyberbrein-nl/ctfdevkit-cli/internal/certs"
	"github.com/stichting-Cyberbrein-nl/ctfdevkit-cli/internal/platform"
)

func newCertsCmd() *cobra.Command {
	var force bool

	cmd := &cobra.Command{
		Use:   "certs",
		Short: "Generate or refresh local TLS certificates",
		RunE: func(cmd *cobra.Command, args []string) error {
			ctx := cmd.Context()
			cfg := configFrom(ctx)
			plat := platformFrom(ctx)

			certsDir, err := platform.CertsDir()
			if err != nil {
				return err
			}

			return certs.Generate(certsDir, cfg.Domain, cfg.BindIP, force, plat)
		},
	}

	cmd.Flags().BoolVar(&force, "force", false, "Force regeneration even if a valid cert exists")
	return cmd
}
