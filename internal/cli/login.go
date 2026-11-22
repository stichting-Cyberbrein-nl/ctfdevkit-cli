package cli

import (
	"github.com/spf13/cobra"
	"github.com/stichting-Cyberbrein-nl/ctfdevkit-cli/internal/ctfnet"
	"github.com/stichting-Cyberbrein-nl/ctfdevkit-cli/internal/output"
)

func newLoginCmd() *cobra.Command {
	return &cobra.Command{
		Use:   "login",
		Short: "Log in to ctf.cyberbrein.net to upload assignments",
		RunE: func(cmd *cobra.Command, args []string) error {
			ctx := cmd.Context()
			c := platformClient(ctx)
			creds, err := deviceLogin(ctx, c)
			if err != nil {
				return err
			}
			output.Successf("Logged in as %s (@%s) on %s", creds.User.Name, creds.User.Username, c.BaseURL)
			return nil
		},
	}
}

func newLogoutCmd() *cobra.Command {
	return &cobra.Command{
		Use:   "logout",
		Short: "Log out of ctf.cyberbrein.net",
		RunE: func(cmd *cobra.Command, args []string) error {
			if err := ctfnet.DeleteCredentials(); err != nil {
				return err
			}
			output.Success("Logged out")
			return nil
		},
	}
}
