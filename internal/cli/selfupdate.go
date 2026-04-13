package cli

import (
	"github.com/spf13/cobra"
	"github.com/stichting-Cyberbrein-nl/ctfdevkit-cli/internal/output"
	"github.com/stichting-Cyberbrein-nl/ctfdevkit-cli/internal/releases"
	"github.com/stichting-Cyberbrein-nl/ctfdevkit-cli/internal/update"
)

func newSelfUpdateCmd() *cobra.Command {
	var manifestURL string

	cmd := &cobra.Command{
		Use:   "self-update",
		Short: "Update the devkit CLI binary to the latest version",
		RunE: func(cmd *cobra.Command, args []string) error {
			ctx := cmd.Context()
			cfg := configFrom(ctx)
			plat := platformFrom(ctx)

			url := manifestURL
			if url == "" {
				url = cfg.ManifestURL
			}

			output.Info("Fetching release manifest...")
			manifest, err := releases.Fetch(ctx, url)
			if err != nil {
				return err
			}

			return update.SelfUpdate(ctx, manifest, Version, plat)
		},
	}

	cmd.Flags().StringVar(&manifestURL, "manifest-url", "", "Override the manifest URL (for testing)")
	return cmd
}
