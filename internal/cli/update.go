package cli

import (
	"github.com/spf13/cobra"
	"github.com/stichting-Cyberbrein-nl/ctfdevkit-cli/internal/output"
	"github.com/stichting-Cyberbrein-nl/ctfdevkit-cli/internal/payload"
	"github.com/stichting-Cyberbrein-nl/ctfdevkit-cli/internal/releases"
	"github.com/stichting-Cyberbrein-nl/ctfdevkit-cli/internal/state"
)

func newUpdateCmd() *cobra.Command {
	return &cobra.Command{
		Use:   "update",
		Short: "Pull the latest DevKit payload (Docker image)",
		RunE: func(cmd *cobra.Command, args []string) error {
			ctx := cmd.Context()
			cfg := configFrom(ctx)
			s := stateFrom(ctx)

			output.Info("Fetching release manifest...")
			manifest, err := releases.Fetch(ctx, cfg.ManifestURL)
			if err != nil {
				return err
			}

			newer, err := manifest.IsNewerPayload(s.PayloadVersion)
			if err != nil {
				return err
			}
			if !newer {
				output.Successf("Payload is already up-to-date (v%s)", s.PayloadVersion)
				return nil
			}

			output.Infof("New payload available: %s → %s", s.PayloadVersion, manifest.Payload.Version)
			if err := payload.Update(ctx, &s, manifest.Payload); err != nil {
				return err
			}

			if err := state.Save(s); err != nil {
				output.Warnf("Could not save state: %v", err)
			}

			output.Successf("Payload updated to v%s", manifest.Payload.Version)
			output.Hint("Run `devkit up` to restart with the new version.")
			return nil
		},
	}
}
