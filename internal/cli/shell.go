package cli

import (
	"github.com/spf13/cobra"
	"github.com/stichting-Cyberbrein-nl/ctfdevkit-cli/internal/docker"
	"github.com/stichting-Cyberbrein-nl/ctfdevkit-cli/internal/payload"
)

func newShellCmd() *cobra.Command {
	return &cobra.Command{
		Use:   "shell",
		Short: "Open an interactive shell in the app container",
		RunE: func(cmd *cobra.Command, args []string) error {
			ctx := cmd.Context()
			cfg := configFrom(ctx)
			s := stateFrom(ctx)

			composeDir, err := payload.ComposeDir(s)
			if err != nil {
				return err
			}

			// interactive=true attaches stdin for PTY.
			return docker.ComposeExec(ctx, composeDir, cfg.AppContainer, true, "/bin/bash")
		},
	}
}
