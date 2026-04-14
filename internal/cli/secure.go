package cli

import (
	"github.com/spf13/cobra"
	"github.com/stichting-Cyberbrein-nl/ctfdevkit-cli/internal/certs"
	"github.com/stichting-Cyberbrein-nl/ctfdevkit-cli/internal/docker"
	"github.com/stichting-Cyberbrein-nl/ctfdevkit-cli/internal/hosts"
	"github.com/stichting-Cyberbrein-nl/ctfdevkit-cli/internal/output"
	"github.com/stichting-Cyberbrein-nl/ctfdevkit-cli/internal/payload"
	"github.com/stichting-Cyberbrein-nl/ctfdevkit-cli/internal/platform"
)

func newSecureCmd() *cobra.Command {
	return &cobra.Command{
		Use:   "secure",
		Short: "Repair hosts binding and TLS trust (force-regenerate certs)",
		RunE: func(cmd *cobra.Command, args []string) error {
			ctx := cmd.Context()
			cfg := configFrom(ctx)
			plat := platformFrom(ctx)
			s := stateFrom(ctx)
			bindIP := effectiveBindIP(cfg, plat)

			output.Info("Repairing hosts binding...")
			if err := hosts.EnsureBinding(bindIP, cfg.Domain, plat); err != nil {
				return err
			}

			output.Info("Regenerating TLS certificates...")
			certsDir, err := platform.CertsDir()
			if err != nil {
				return err
			}
			if err := certs.Generate(certsDir, cfg.Domain, bindIP, true, plat); err != nil {
				return err
			}

			// Reload proxy so new certs take effect.
			if s.IsPayloadInstalled() {
				composeDir, err := payload.ComposeDir(s)
				if err == nil {
					output.Info("Reloading proxy...")
					_ = docker.ReloadProxy(ctx, composeDir)
				}
			}

			output.Success("Security configuration repaired")
			return nil
		},
	}
}
