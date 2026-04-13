package cli

import (
	"os"

	"github.com/spf13/cobra"
	"github.com/stichting-Cyberbrein-nl/ctfdevkit-cli/internal/assignments"
	"github.com/stichting-Cyberbrein-nl/ctfdevkit-cli/internal/config"
	"github.com/stichting-Cyberbrein-nl/ctfdevkit-cli/internal/output"
	"github.com/stichting-Cyberbrein-nl/ctfdevkit-cli/internal/tui"
)

func newConfigAssignmentsCmd() *cobra.Command {
	return &cobra.Command{
		Use:   "config-assignments",
		Short: "Set the path to your CTF assignments folder",
		RunE: func(cmd *cobra.Command, args []string) error {
			ctx := cmd.Context()
			cfg := configFrom(ctx)

			// Show current value if already set.
			if cfg.AssignmentsPath != "" {
				count := assignments.Count(cfg.AssignmentsPath)
				output.Infof("Current path: %s (%d assignments)", cfg.AssignmentsPath, count)
				output.Plain("")
			}

			path, err := tui.AskAssignmentsPath()
			if err != nil {
				return err
			}
			if path == "" {
				output.Info("No changes made.")
				return nil
			}

			// Create directory if it doesn't exist yet.
			if err := os.MkdirAll(path, 0755); err != nil {
				return err
			}

			cfg.AssignmentsPath = path
			if err := config.Save(cfg); err != nil {
				return err
			}

			count := assignments.Count(path)
			output.Successf("Assignments path saved: %s (%d found)", path, count)
			output.Hint("Run `devkit up` to restart containers with the new path.")
			return nil
		},
	}
}
