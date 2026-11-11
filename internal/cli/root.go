// Package cli wires together all Cobra commands for the devkit CLI.
package cli

import (
	"context"
	"errors"
	"fmt"
	"os"

	"github.com/spf13/cobra"
	"github.com/stichting-Cyberbrein-nl/ctfdevkit-cli/internal/config"
	"github.com/stichting-Cyberbrein-nl/ctfdevkit-cli/internal/output"
	"github.com/stichting-Cyberbrein-nl/ctfdevkit-cli/internal/payload"
	"github.com/stichting-Cyberbrein-nl/ctfdevkit-cli/internal/platform"
	"github.com/stichting-Cyberbrein-nl/ctfdevkit-cli/internal/state"
	"github.com/stichting-Cyberbrein-nl/ctfdevkit-cli/internal/tui"
)

// Version is set at build time via -ldflags.
var Version = "dev"

// rootCmd launches the interactive TUI when called with no subcommand.
var rootCmd = &cobra.Command{
	Use:   "devkit",
	Short: "Cyberbrein CTF DevKit — local environment manager",
	Long:  `devkit manages your local CTF:DevKit environment.`,
	// Runs for every subcommand too, so `devkit setup` / `devkit up` pick up a new
	// release instead of only the bare TUI invocation.
	PersistentPreRunE: func(cmd *cobra.Command, args []string) error {
		return ensureUpToDate(cmd)
	},
	RunE: func(cmd *cobra.Command, args []string) error {
		ctx := cmd.Context()
		cfg := configFrom(ctx)
		s := stateFrom(ctx)

		// Resolve the compose directory if payload is installed.
		composeDir := ""
		if s.IsPayloadInstalled() {
			if d, err := payload.ComposeDir(s); err == nil {
				composeDir = d
			}
		}

		return tui.Run(ctx, cfg, s, Version, composeDir)
	},
	SilenceUsage:  true,
	SilenceErrors: true,
}

// Execute is the entrypoint called from main.
func Execute(ctx context.Context, version string) {
	Version = version

	// `mcp serve` hands stdout to the MCP protocol: nothing else may write to it.
	mcpServe := isMCPServeInvocation(os.Args[1:])
	fatalf := output.Fatalf
	if mcpServe {
		fatalf = func(format string, a ...any) {
			fmt.Fprintf(os.Stderr, "devkit mcp serve: "+format+"\n", a...)
			os.Exit(1)
		}
	}

	plat := platform.Detect()
	ctx = withPlatform(ctx, plat)

	cfg, err := config.Load()
	if err != nil {
		fatalf("Failed to load config: %v", err)
	}
	ctx = withConfig(ctx, cfg)

	s, err := state.Load()
	if err != nil {
		fatalf("Failed to load state: %v", err)
	}
	ctx = withState(ctx, s)

	// Print banner only when a subcommand is invoked directly (not TUI, not completion).
	if !mcpServe && len(os.Args) >= 2 && os.Args[1] != "completion" && os.Args[1] != "__complete" {
		output.Banner(cfg.Brand, version)
	}

	if err := rootCmd.ExecuteContext(ctx); err != nil {
		if mcpServe {
			fmt.Fprintf(os.Stderr, "devkit mcp serve: %v\n", err)
			os.Exit(1)
		}
		if errors.Is(err, errRestartRequired) {
			output.Successf("%v", err)
			return
		}
		output.Failf("%v", err)
		os.Exit(1)
	}
}

func init() {
	rootCmd.AddCommand(
		newSetupCmd(),
		newUpCmd(),
		newDownCmd(),
		newResetCmd(),
		newLogsCmd(),
		newDoctorCmd(),
		newMCPCmd(),
		newStatusCmd(),
		newShellCmd(),
		newArtisanCmd(),
		newOpenCmd(),
		newBindHostsCmd(),
		newSecureCmd(),
		newReleasePortsCmd(),
		newCertsCmd(),
		newPrereqsCmd(),
		newPruneCmd(),
		newUpdateCmd(),
		newSelfUpdateCmd(),
		newVersionCmd(),
		newAssignmentsCmd(),
		newConfigAssignmentsCmd(),
	)
}

// isMCPServeInvocation reports whether args (without the program name) run `mcp serve`.
func isMCPServeInvocation(args []string) bool {
	return len(args) >= 2 && args[0] == "mcp" && args[1] == "serve"
}

// newVersionCmd shows the current CLI version.
func newVersionCmd() *cobra.Command {
	return &cobra.Command{
		Use:   "version",
		Short: "Show the devkit CLI version",
		Run: func(cmd *cobra.Command, args []string) {
			fmt.Printf("devkit version %s\n", Version)
		},
	}
}

// context key types to avoid collisions.
type ctxKeyPlatform struct{}
type ctxKeyConfig struct{}
type ctxKeyState struct{}

func withPlatform(ctx context.Context, p platform.Platform) context.Context {
	return context.WithValue(ctx, ctxKeyPlatform{}, p)
}

func withConfig(ctx context.Context, c config.Config) context.Context {
	return context.WithValue(ctx, ctxKeyConfig{}, c)
}

func withState(ctx context.Context, s state.State) context.Context {
	return context.WithValue(ctx, ctxKeyState{}, s)
}

func platformFrom(ctx context.Context) platform.Platform {
	if p, ok := ctx.Value(ctxKeyPlatform{}).(platform.Platform); ok {
		return p
	}
	return platform.Detect()
}

func configFrom(ctx context.Context) config.Config {
	if c, ok := ctx.Value(ctxKeyConfig{}).(config.Config); ok {
		return c
	}
	return config.Default()
}

func stateFrom(ctx context.Context) state.State {
	if s, ok := ctx.Value(ctxKeyState{}).(state.State); ok {
		return s
	}
	return state.State{}
}
