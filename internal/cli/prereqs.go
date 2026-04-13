package cli

import (
	"github.com/spf13/cobra"
	"github.com/stichting-Cyberbrein-nl/ctfdevkit-cli/internal/output"
	"github.com/stichting-Cyberbrein-nl/ctfdevkit-cli/internal/prereqs"
)

func newPrereqsCmd() *cobra.Command {
	return &cobra.Command{
		Use:   "install-prereqs",
		Short: "Install or verify Docker and mkcert",
		RunE: func(cmd *cobra.Command, args []string) error {
			ctx := cmd.Context()
			plat := platformFrom(ctx)

			if err := prereqs.Check(ctx, plat); err != nil {
				return err
			}
			output.Success("All prerequisites are installed")
			return nil
		},
	}
}
