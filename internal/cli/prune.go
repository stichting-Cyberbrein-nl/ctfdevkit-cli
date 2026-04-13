package cli

import (
	"github.com/spf13/cobra"
	"github.com/stichting-Cyberbrein-nl/ctfdevkit-cli/internal/docker"
	"github.com/stichting-Cyberbrein-nl/ctfdevkit-cli/internal/output"
)

func newPruneCmd() *cobra.Command {
	return &cobra.Command{
		Use:   "prune",
		Short: "Aggressively clean up Docker images, containers, and build cache",
		RunE: func(cmd *cobra.Command, args []string) error {
			ctx := cmd.Context()
			output.Warn("This will remove ALL unused Docker data.")
			output.Info("Pruning Docker resources...")
			if err := docker.SystemPrune(ctx); err != nil {
				return err
			}
			output.Success("Docker resources pruned")
			return nil
		},
	}
}
