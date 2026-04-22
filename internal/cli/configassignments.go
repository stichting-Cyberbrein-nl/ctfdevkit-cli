package cli

import (
	"github.com/spf13/cobra"
	"github.com/stichting-Cyberbrein-nl/ctfdevkit-cli/internal/config"
	"github.com/stichting-Cyberbrein-nl/ctfdevkit-cli/internal/output"
)

func newConfigAssignmentsCmd() *cobra.Command {
	var pathOverride string
	var repoURL string

	cmd := &cobra.Command{
		Use:   "config-assignments",
		Short: "Set up the assignments repository path",
		RunE: func(cmd *cobra.Command, args []string) error {
			ctx := cmd.Context()
			cfg := configFrom(ctx)
			plat := platformFrom(ctx)
			if repoURL != "" {
				cfg.AssignmentsRepoURL = repoURL
			}

			path, err := chooseAssignmentsRepoPath(ctx, cfg, plat, pathOverride, defaultAssignmentsPrompter(), defaultAssignmentsFlowOps())
			if err != nil {
				return err
			}

			cfg.AssignmentsPath = path
			if err := config.Save(cfg); err != nil {
				return err
			}

			output.Successf("Assignments path saved: %s", path)
			output.Hint("Run `devkit up` to restart containers with the new path.")
			return nil
		},
	}

	cmd.Flags().StringVar(&pathOverride, "path", "", "Assignments repository path override")
	cmd.Flags().StringVar(&repoURL, "repo-url", "", "Assignments repository URL override")
	return cmd
}
