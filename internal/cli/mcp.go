package cli

import (
	"context"
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"time"

	"github.com/spf13/cobra"
	"github.com/stichting-Cyberbrein-nl/ctfdevkit-cli/internal/docker"
	"github.com/stichting-Cyberbrein-nl/ctfdevkit-cli/internal/mcpclients"
	"github.com/stichting-Cyberbrein-nl/ctfdevkit-cli/internal/output"
	"github.com/stichting-Cyberbrein-nl/ctfdevkit-cli/internal/payload"
	"github.com/stichting-Cyberbrein-nl/ctfdevkit-cli/internal/webapp"
)

// skipUpdateCheckAnnotation marks commands that must never self-update.
const skipUpdateCheckAnnotation = "devkit.skipUpdateCheck"

func newMCPCmd() *cobra.Command {
	cmd := &cobra.Command{
		Use:   "mcp",
		Short: "Connect AI tools to the DevKit MCP server",
	}
	cmd.AddCommand(newMCPInstallCmd(), newMCPUninstallCmd(), newMCPStatusCmd(), newMCPServeCmd())
	return cmd
}

// mcpServer is the command AI tools start: this binary's absolute path plus `mcp serve`.
// GUI-launched tools do not see the shell PATH, so a bare "devkit" would not resolve.
func mcpServer() (mcpclients.Server, error) {
	exe, err := os.Executable()
	if err != nil {
		return mcpclients.Server{}, err
	}
	if resolved, err := filepath.EvalSymlinks(exe); err == nil {
		exe = resolved
	}
	if strings.Contains(exe, "go-build") {
		return mcpclients.Server{}, errors.New("run this from an installed devkit binary, not `go run`")
	}
	return mcpclients.Server{Command: exe, Args: []string{"mcp", "serve"}}, nil
}

// selectClients resolves explicit client IDs; with none it returns nil.
func selectClients(ids []string) ([]mcpclients.Client, error) {
	var out []mcpclients.Client
	for _, id := range ids {
		c, ok := mcpclients.ByID(strings.ToLower(id))
		if !ok {
			return nil, fmt.Errorf("unknown client %q (choose from %s)", id, strings.Join(mcpclients.IDs(), ", "))
		}
		out = append(out, c)
	}
	return out, nil
}

func printResult(name string, r mcpclients.Result) {
	line := fmt.Sprintf("%s — %s (%s)", name, r.Action, r.Location)
	if r.Note != "" {
		line += " — " + r.Note
	}
	switch r.Action {
	case mcpclients.ActionSkipped:
		output.Warn(line)
	case mcpclients.ActionUnchanged:
		output.Info(line)
	default:
		output.Success(line)
	}
}

func newMCPInstallCmd() *cobra.Command {
	var all bool
	cmd := &cobra.Command{
		Use:   "install [client...]",
		Short: "Install the DevKit MCP server in your AI tools (" + strings.Join(mcpclients.IDs(), ", ") + ")",
		RunE: func(cmd *cobra.Command, args []string) error {
			ctx := cmd.Context()
			clients, err := selectClients(args)
			if err != nil {
				return err
			}
			env, err := mcpclients.DefaultEnv()
			if err != nil {
				return err
			}
			server, err := mcpServer()
			if err != nil {
				return err
			}

			explicit := len(clients) > 0 && !all
			switch {
			case all:
				clients = mcpclients.All()
			case len(clients) == 0:
				for _, c := range mcpclients.All() {
					if c.Detected(env) {
						clients = append(clients, c)
					}
				}
				if len(clients) == 0 {
					return fmt.Errorf("No supported AI tools detected. Pass one explicitly: devkit mcp install %s", strings.Join(mcpclients.IDs(), "|"))
				}
			}

			var failed []string
			for _, c := range clients {
				res, err := c.Install(ctx, env, server)
				if err != nil {
					if mcpclients.IsMissingBinary(err) && !explicit {
						res.Action, res.Location, res.Note = mcpclients.ActionSkipped, c.Location(env), err.Error()
						printResult(c.Name(), res)
						continue
					}
					output.Failf("%s — %v", c.Name(), err)
					failed = append(failed, c.ID())
					continue
				}
				printResult(c.Name(), res)
			}

			output.Hint("Restart your AI tool, then ask it to create an assignment.")
			output.Hint("Test the connection: devkit mcp status")
			if platformFrom(ctx).IsWSL {
				output.Hint("This configured AI tools running inside WSL. For tools running on Windows, run devkit mcp install from PowerShell.")
			}
			if len(failed) > 0 {
				return fmt.Errorf("could not install into: %s", strings.Join(failed, ", "))
			}
			return nil
		},
	}
	cmd.Flags().BoolVar(&all, "all", false, "Install into every supported AI tool")
	return cmd
}

func newMCPUninstallCmd() *cobra.Command {
	var all bool
	cmd := &cobra.Command{
		Use:   "uninstall [client...]",
		Short: "Remove the DevKit MCP server from your AI tools",
		RunE: func(cmd *cobra.Command, args []string) error {
			ctx := cmd.Context()
			clients, err := selectClients(args)
			if err != nil {
				return err
			}
			env, err := mcpclients.DefaultEnv()
			if err != nil {
				return err
			}
			if all {
				clients = mcpclients.All()
			} else if len(clients) == 0 {
				for _, c := range mcpclients.All() {
					if ok, _ := c.Installed(ctx, env); ok {
						clients = append(clients, c)
					}
				}
				if len(clients) == 0 {
					output.Info("The DevKit MCP server is not installed in any supported AI tool.")
					return nil
				}
			}

			var failed []string
			for _, c := range clients {
				res, err := c.Uninstall(ctx, env)
				if err != nil {
					output.Failf("%s — %v", c.Name(), err)
					failed = append(failed, c.ID())
					continue
				}
				printResult(c.Name(), res)
			}
			if len(failed) > 0 {
				return fmt.Errorf("could not uninstall from: %s", strings.Join(failed, ", "))
			}
			return nil
		},
	}
	cmd.Flags().BoolVar(&all, "all", false, "Remove from every supported AI tool")
	return cmd
}

func newMCPStatusCmd() *cobra.Command {
	return &cobra.Command{
		Use:   "status",
		Short: "Show connected AI tools and test the MCP server",
		RunE: func(cmd *cobra.Command, args []string) error {
			ctx := cmd.Context()
			if err := docker.EnsureOnPath(); err != nil {
				output.Warnf("%v", err)
			}
			env, err := mcpclients.DefaultEnv()
			if err != nil {
				return err
			}

			output.Header("AI tools")
			output.Plainf("%-32s %-9s %-10s %s", "CLIENT", "DETECTED", "CONNECTED", "LOCATION")
			for _, c := range mcpclients.All() {
				connected := "no"
				if ok, err := c.Installed(ctx, env); err != nil {
					connected = "error"
				} else if ok {
					connected = "yes"
				}
				output.Plainf("%-32s %-9s %-10s %s", c.Name(), yesNo(c.Detected(env)), connected, c.Location(env))
			}

			output.Header("Server")
			server, err := mcpServer()
			if err != nil {
				return err
			}
			res, err := mcpclients.Probe(ctx, server.Command, server.Args, Version)
			if err != nil {
				output.Failf("Server did not respond: %v", err)
				if res.Stderr != "" {
					output.Plain(res.Stderr)
				}
				return errors.New("MCP server test failed")
			}
			output.Successf("Server %s %s — %d tools: %s", res.ServerName, res.ServerVersion, len(res.Tools), strings.Join(res.Tools, ", "))
			return nil
		},
	}
}

func yesNo(b bool) string {
	if b {
		return "yes"
	}
	return "no"
}

func newMCPServeCmd() *cobra.Command {
	return &cobra.Command{
		Use:         "serve",
		Short:       "Run the MCP server over stdio (started by your AI tools)",
		Annotations: map[string]string{skipUpdateCheckAnnotation: "true"},
		RunE: func(cmd *cobra.Command, args []string) error {
			return runMCPServe(cmd.Context())
		},
	}
}

// runMCPServe bridges stdio into the container. Stdout belongs to the MCP protocol,
// so every diagnostic goes to stderr.
func runMCPServe(ctx context.Context) error {
	logf := func(format string, a ...any) { fmt.Fprintf(os.Stderr, "[devkit] "+format+"\n", a...) }

	if err := docker.EnsureOnPath(); err != nil {
		return err
	}
	cfg := configFrom(ctx)
	s := stateFrom(ctx)
	if !s.IsPayloadInstalled() {
		return errors.New("payload not installed — run `devkit setup` first")
	}
	composeDir, err := payload.ComposeDir(s)
	if err != nil {
		return err
	}
	if _, err := os.Stat(filepath.Join(composeDir, ".env")); err != nil {
		return errors.New(`run "devkit up" once before using the MCP server`)
	}

	client := webapp.New(composeDir, cfg.AppContainer)
	running, err := docker.IsServiceRunning(ctx, composeDir, cfg.AppContainer)
	if err != nil {
		return errors.New("Docker is not running — start Docker Desktop (or the docker service) first")
	}
	if !running {
		logf("Starting DevKit…")
		if err := docker.ComposeUpTo(ctx, composeDir, os.Stderr); err != nil {
			return err
		}
		if err := client.WaitForReady(ctx, 60, 2*time.Second, func(i, n int) {
			logf("Not ready yet (%d/%d)...", i, n)
		}); err != nil {
			return err
		}
	}

	info, err := client.Info(ctx)
	if err != nil {
		return err
	}
	if err := webapp.RequireAPI(info); err != nil {
		return err
	}

	return docker.ComposeExecStdio(ctx, composeDir, cfg.AppContainer, map[string]string{"APP_DEBUG": "false"},
		"php", "-d", "display_errors=stderr", "artisan", "mcp:start", info.MCP.Handle)
}
