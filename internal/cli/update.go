package cli

import (
	"context"

	"github.com/spf13/cobra"
	"github.com/stichting-Cyberbrein-nl/ctfdevkit-cli/internal/config"
	"github.com/stichting-Cyberbrein-nl/ctfdevkit-cli/internal/docker"
	"github.com/stichting-Cyberbrein-nl/ctfdevkit-cli/internal/output"
	"github.com/stichting-Cyberbrein-nl/ctfdevkit-cli/internal/payload"
	"github.com/stichting-Cyberbrein-nl/ctfdevkit-cli/internal/releases"
	"github.com/stichting-Cyberbrein-nl/ctfdevkit-cli/internal/state"
	"github.com/stichting-Cyberbrein-nl/ctfdevkit-cli/internal/webapp"
)

func newUpdateCmd() *cobra.Command {
	return &cobra.Command{
		Use:   "update",
		Short: "Update the DevKit environment to the latest version",
		RunE: func(cmd *cobra.Command, args []string) error {
			ctx := cmd.Context()
			cfg := configFrom(ctx)
			s := stateFrom(ctx)

			output.Info("Checking for the latest DevKit version...")
			manifest, err := releases.Fetch(ctx, cfg.ManifestURL)
			if err != nil {
				return err
			}

			newer, err := manifest.IsNewerPayload(s.PayloadVersion)
			if err != nil {
				return err
			}
			if !newer {
				output.Successf("DevKit is already up to date (v%s)", s.PayloadVersion)
				return nil
			}

			if s.PayloadVersion != "" {
				output.Infof("Updating DevKit: v%s → v%s", s.PayloadVersion, manifest.Payload.Version)
			}
			if err := installDevKitVersion(ctx, &cfg, &s, manifest.Payload); err != nil {
				return err
			}

			if !s.IsPayloadInstalled() {
				output.Successf("DevKit v%s downloaded", manifest.Payload.Version)
				output.Hint("Run `devkit setup` to install it.")
				return nil
			}

			composeDir, err := payload.ComposeDir(s)
			if err != nil {
				return err
			}

			// A running stack keeps the old container until it is recreated, so apply
			// the update right away instead of leaving the user on the old version.
			if running, err := docker.IsServiceRunning(ctx, composeDir, cfg.AppContainer); err != nil || !running {
				output.Successf("DevKit updated to v%s", manifest.Payload.Version)
				output.Hint("Run `devkit up` to start it.")
				return nil
			}

			output.Info("Restarting containers with the new version...")
			if err := docker.ComposeUp(ctx, composeDir); err != nil {
				return err
			}
			if err := waitForApp(ctx, webapp.New(composeDir, cfg.AppContainer)); err != nil {
				return err
			}

			output.Successf("DevKit updated to v%s and running", manifest.Payload.Version)
			return nil
		},
	}
}

// installDevKitVersion pulls the release image, points the compose file at it and records
// it as the configured version in config.json. Without that last step `devkit setup`
// would re-extract the bundle with the old configured version and undo the update.
func installDevKitVersion(ctx context.Context, cfg *config.Config, s *state.State, rel releases.PayloadRelease) error {
	if err := payload.Update(ctx, s, rel); err != nil {
		return err
	}

	cfg.Version = rel.Version
	return config.Save(*cfg)
}
