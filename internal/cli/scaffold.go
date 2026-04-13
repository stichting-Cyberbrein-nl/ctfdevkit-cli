package cli

import (
	"fmt"

	"github.com/spf13/cobra"
	"github.com/stichting-Cyberbrein-nl/ctfdevkit-cli/internal/docker"
	"github.com/stichting-Cyberbrein-nl/ctfdevkit-cli/internal/output"
	"github.com/stichting-Cyberbrein-nl/ctfdevkit-cli/internal/payload"
)

func newScaffoldCmd() *cobra.Command {
	var category string
	var difficulty int

	cmd := &cobra.Command{
		Use:   "scaffold <name>",
		Short: "Scaffold a new CTF assignment",
		Args:  cobra.ExactArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			ctx := cmd.Context()
			cfg := configFrom(ctx)
			s := stateFrom(ctx)

			name := args[0]

			composeDir, err := payload.ComposeDir(s)
			if err != nil {
				return err
			}

			artisanArgs := []string{
				"php", "artisan", "devkit:scaffold-assignment", name,
			}
			if category != "" {
				artisanArgs = append(artisanArgs, fmt.Sprintf("--category=%s", category))
			}
			if cmd.Flags().Changed("difficulty") {
				artisanArgs = append(artisanArgs, fmt.Sprintf("--difficulty=%d", difficulty))
			}

			output.Infof("Scaffolding assignment: %s", name)
			if err := docker.ComposeExec(ctx, composeDir, cfg.AppContainer, false, artisanArgs...); err != nil {
				return err
			}

			output.Successf("Assignment '%s' created", name)
			return nil
		},
	}

	cmd.Flags().StringVar(&category, "category", "", "Assignment category")
	cmd.Flags().IntVar(&difficulty, "difficulty", 1, "Difficulty level (0–5)")

	return cmd
}
