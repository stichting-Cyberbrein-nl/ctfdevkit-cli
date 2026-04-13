package cli

import (
	"github.com/spf13/cobra"
	"github.com/stichting-Cyberbrein-nl/ctfdevkit-cli/internal/browser"
	"github.com/stichting-Cyberbrein-nl/ctfdevkit-cli/internal/output"
)

func newOpenCmd() *cobra.Command {
	return &cobra.Command{
		Use:   "open",
		Short: "Open the DevKit URL in the default browser",
		RunE: func(cmd *cobra.Command, args []string) error {
			ctx := cmd.Context()
			cfg := configFrom(ctx)
			plat := platformFrom(ctx)

			output.Infof("Opening %s...", cfg.URL)
			return browser.Open(cfg.URL, plat)
		},
	}
}
