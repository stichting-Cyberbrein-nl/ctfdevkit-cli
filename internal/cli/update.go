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
			rel := manifest.Payload

			refresh, err := shouldRefreshPayload(s.PayloadVersion, rel.Version)
			if err != nil {
				return err
			}
			if !refresh {
				output.Successf("Installed DevKit v%s is newer than the latest release (v%s); nothing to update", s.PayloadVersion, rel.Version)
				return nil
			}

			versionChanged := s.PayloadVersion != rel.Version
			if versionChanged && s.PayloadVersion != "" {
				output.Infof("Updating DevKit: v%s → v%s", s.PayloadVersion, rel.Version)
			} else if !versionChanged {
				output.Infof("Checking DevKit v%s for a newer image...", rel.Version)
			}

			// Always pull: the same version tag can be republished with fixes, and a local
			// image with that tag may not be the published one.
			imageBefore := docker.ImageID(ctx, rel.Image)
			if err := installDevKitVersion(ctx, &cfg, &s, rel); err != nil {
				return err
			}
			changed := versionChanged || imageBefore == "" || docker.ImageID(ctx, rel.Image) != imageBefore

			if !s.IsPayloadInstalled() {
				output.Successf("DevKit v%s downloaded", rel.Version)
				output.Hint("Run `devkit setup` to install it.")
				return nil
			}

			composeDir, err := payload.ComposeDir(s)
			if err != nil {
				return err
			}

			if running, err := docker.IsServiceRunning(ctx, composeDir, cfg.AppContainer); err != nil || !running {
				if changed {
					output.Successf("DevKit updated to v%s", rel.Version)
				} else {
					output.Successf("DevKit is already up to date (v%s)", rel.Version)
				}
				output.Hint("Run `devkit up` to start it.")
				return nil
			}

			// A running stack keeps its old container until it is recreated. Compose only
			// recreates the services whose image or configuration actually changed.
			output.Info("Applying the update to the running containers...")
			if err := docker.ComposeUp(ctx, composeDir); err != nil {
				return err
			}
			if err := waitForApp(ctx, webapp.New(composeDir, cfg.AppContainer)); err != nil {
				return err
			}

			if changed {
				output.Successf("DevKit updated to v%s and running", rel.Version)
			} else {
				output.Successf("DevKit is already up to date (v%s) and running", rel.Version)
			}
			return nil
		},
	}
}

// shouldRefreshPayload reports whether `devkit update` should (re)install the latest
// release: always, unless the installed version is newer (a local test build).
func shouldRefreshPayload(installed, latest string) (bool, error) {
	if installed == "" {
		return true, nil
	}
	installedIsNewer, err := releases.IsNewer(installed, latest)
	if err != nil {
		return false, err
	}
	return !installedIsNewer, nil
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
