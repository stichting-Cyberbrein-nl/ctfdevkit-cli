package cli

import (
	"fmt"
	"path/filepath"

	"github.com/spf13/cobra"
	"github.com/stichting-Cyberbrein-nl/ctfdevkit-cli/internal/docker"
	"github.com/stichting-Cyberbrein-nl/ctfdevkit-cli/internal/doctor"
	"github.com/stichting-Cyberbrein-nl/ctfdevkit-cli/internal/output"
	"github.com/stichting-Cyberbrein-nl/ctfdevkit-cli/internal/payload"
	"github.com/stichting-Cyberbrein-nl/ctfdevkit-cli/internal/ports"
)

func newUpCmd() *cobra.Command {
	return &cobra.Command{
		Use:   "up",
		Short: "Build and start the containers",
		RunE: func(cmd *cobra.Command, args []string) error {
			ctx := cmd.Context()
			cfg := configFrom(ctx)
			s := stateFrom(ctx)
			plat := platformFrom(ctx)
			bindIP := effectiveBindIP(cfg, plat)

			if !s.IsPayloadInstalled() {
				return fmt.Errorf("payload not installed — run `devkit setup` first")
			}

			composeDir, err := payload.ComposeDir(s)
			if err != nil {
				return err
			}

			output.Info("Resolving ports...")
			pair, err := ports.ResolvePortsForBindIP(ctx, 80, 443, bindIP, plat)
			if err != nil {
				return err
			}

			appURL := buildURL(cfg.Domain, pair.HTTPS)
			assignmentsPath := cfg.AssignmentsPath
			if assignmentsPath == "" {
				assignmentsPath = filepath.Join(composeDir, "assignments")
			}
			if err := writeComposeEnv(composeDir, bindIP, pair.HTTP, pair.HTTPS, appURL, assignmentsPath); err != nil {
				return err
			}

			output.Info("Starting containers...")
			if err := docker.ComposeUp(ctx, composeDir); err != nil {
				return err
			}

			output.Info("Waiting for health check...")
			if err := doctor.WaitForReady(ctx, composeDir, cfg.AppContainer); err != nil {
				return err
			}

			output.Successf("Environment is up at %s", buildURL(cfg.Domain, pair.HTTPS))
			return nil
		},
	}
}
