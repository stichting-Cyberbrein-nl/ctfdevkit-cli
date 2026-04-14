package cli

import (
	"github.com/spf13/cobra"
	"github.com/stichting-Cyberbrein-nl/ctfdevkit-cli/internal/hosts"
	"github.com/stichting-Cyberbrein-nl/ctfdevkit-cli/internal/output"
)

func newBindHostsCmd() *cobra.Command {
	return &cobra.Command{
		Use:   "bind-hosts",
		Short: "Add the DevKit domain to /etc/hosts",
		RunE: func(cmd *cobra.Command, args []string) error {
			ctx := cmd.Context()
			cfg := configFrom(ctx)
			plat := platformFrom(ctx)
			bindIP := effectiveBindIP(cfg, plat)

			output.Infof("Binding %s -> %s", bindIP, cfg.Domain)
			return hosts.EnsureBinding(bindIP, cfg.Domain, plat)
		},
	}
}
