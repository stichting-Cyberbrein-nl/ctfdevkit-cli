package cli

import (
	"errors"
	"fmt"
	"path/filepath"
	"strings"

	"github.com/spf13/cobra"
	"github.com/stichting-Cyberbrein-nl/ctfdevkit-cli/internal/assignments"
	"github.com/stichting-Cyberbrein-nl/ctfdevkit-cli/internal/config"
	"github.com/stichting-Cyberbrein-nl/ctfdevkit-cli/internal/ctfnet"
	"github.com/stichting-Cyberbrein-nl/ctfdevkit-cli/internal/output"
	"github.com/stichting-Cyberbrein-nl/ctfdevkit-cli/internal/payload"
	"github.com/stichting-Cyberbrein-nl/ctfdevkit-cli/internal/state"
	"github.com/stichting-Cyberbrein-nl/ctfdevkit-cli/internal/tui"
	"github.com/stichting-Cyberbrein-nl/ctfdevkit-cli/internal/webapp"
)

// localAssignmentsPath is the configured assignments folder, else the payload's ./assignments.
func localAssignmentsPath(cfg config.Config, s state.State) (string, error) {
	if strings.TrimSpace(cfg.AssignmentsPath) != "" {
		return cfg.AssignmentsPath, nil
	}
	if s.IsPayloadInstalled() {
		if dir, err := payload.ComposeDir(s); err == nil {
			return filepath.Join(dir, "assignments"), nil
		}
	}
	return "", errors.New("no assignments folder configured — run devkit assignments set-path")
}

func newAssignmentsUploadCmd() *cobra.Command {
	var skipCheck bool

	cmd := &cobra.Command{
		Use:   "upload [name]",
		Short: "Upload an assignment to ctf.cyberbrein.net",
		Args:  cobra.MaximumNArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			ctx := cmd.Context()
			cfg := configFrom(ctx)

			root, err := localAssignmentsPath(cfg, stateFrom(ctx))
			if err != nil {
				return err
			}

			var name string
			switch {
			case len(args) == 1:
				name = args[0]
			case isInteractive():
				names, err := assignments.List(root)
				if err != nil || len(names) == 0 {
					return errors.New("no assignments found in " + root)
				}
				i, err := tui.AskChoice("Upload assignment", "Choose the assignment to publish on "+cfg.PlatformURL, names, 0)
				if err != nil {
					return err
				}
				name = names[i]
			default:
				return errors.New("missing assignment name")
			}
			if !ctfnet.ValidName(name) {
				return fmt.Errorf("%q is not a valid assignment name: use \"[N] Title\", e.g. \"[12] Cookie stealing\" (no / \\ < > : \" | ? * # %% +, not ending in a dot or space)", name)
			}

			if !skipCheck {
				client, _, err := requireWebapp(ctx)
				if err != nil {
					output.Warn("The local DevKit is not running — skipping the assignment check")
				} else {
					report, err := client.Check(ctx, name, false)
					var cmdErr *webapp.CommandError
					if err != nil && !errors.As(err, &cmdErr) {
						return err
					}
					renderCheck(report)
					if !report.OK {
						return errors.New("the DevKit check failed; fix the problems above or use --skip-check")
					}
				}
			}

			archive, err := ctfnet.ZipAssignment(filepath.Join(root, name))
			if err != nil {
				return err
			}

			c := platformClient(ctx)
			creds, err := ensureLoggedIn(ctx, c)
			if err != nil {
				return err
			}
			output.Infof("Uploading %s to %s...", name, c.BaseURL)
			result, err := c.UploadAssignment(ctx, creds.AccessToken, name, archive)
			if errors.Is(err, ctfnet.ErrUnauthorized) {
				creds, err = refreshCredentials(ctx, c, creds)
				if err != nil {
					return errNotLoggedIn
				}
				result, err = c.UploadAssignment(ctx, creds.AccessToken, name, archive)
			}
			if errors.Is(err, ctfnet.ErrUnauthorized) {
				return errNotLoggedIn
			}
			if err != nil {
				return err
			}

			verb := "Published"
			if result.Replaced {
				verb = "Updated"
			}
			output.Successf("%s %s", verb, result.Name)
			output.Hint(result.URL)
			return nil
		},
	}

	cmd.Flags().BoolVar(&skipCheck, "skip-check", false, "Skip the local DevKit assignment check")
	return cmd
}
