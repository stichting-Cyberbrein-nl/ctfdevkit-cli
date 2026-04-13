package cli

import (
	"github.com/spf13/cobra"
	"github.com/stichting-Cyberbrein-nl/ctfdevkit-cli/internal/docker"
	"github.com/stichting-Cyberbrein-nl/ctfdevkit-cli/internal/payload"
)

func newArtisanCmd() *cobra.Command {
	return &cobra.Command{
		Use:                "artisan [args...]",
		Short:              "Run a Laravel Artisan command inside the container",
		DisableFlagParsing: true,
		Args:               cobra.ArbitraryArgs,
		RunE: func(cmd *cobra.Command, args []string) error {
			ctx := cmd.Context()
			cfg := configFrom(ctx)
			s := stateFrom(ctx)

			composeDir, err := payload.ComposeDir(s)
			if err != nil {
				return err
			}

			artisanArgs := append([]string{"php", "artisan"}, args...)
			return docker.ComposeExec(ctx, composeDir, cfg.AppContainer, false, artisanArgs...)
		},
	}
}
