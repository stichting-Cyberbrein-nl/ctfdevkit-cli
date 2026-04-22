// Package cli wires together all Cobra commands for the devkit CLI.
package cli

import (
	"context"
	"fmt"
	"os"

	"github.com/spf13/cobra"
	"github.com/stichting-Cyberbrein-nl/ctfdevkit-cli/internal/config"
	"github.com/stichting-Cyberbrein-nl/ctfdevkit-cli/internal/output"
	"github.com/stichting-Cyberbrein-nl/ctfdevkit-cli/internal/payload"
	"github.com/stichting-Cyberbrein-nl/ctfdevkit-cli/internal/platform"
	"github.com/stichting-Cyberbrein-nl/ctfdevkit-cli/internal/releases"
	"github.com/stichting-Cyberbrein-nl/ctfdevkit-cli/internal/state"
	"github.com/stichting-Cyberbrein-nl/ctfdevkit-cli/internal/tui"
	"github.com/stichting-Cyberbrein-nl/ctfdevkit-cli/internal/update"
)

// Version is set at build time via -ldflags.
var Version = "dev"

// rootCmd launches the interactive TUI when called with no subcommand.
var rootCmd = &cobra.Command{
	Use:   "devkit",
	Short: "Cyberbrein CTF DevKit — local environment manager",
	Long:  `devkit manages your local CTF:DevKit environment.`,
	RunE: func(cmd *cobra.Command, args []string) error {
		ctx := cmd.Context()
		cfg := configFrom(ctx)
		s := stateFrom(ctx)

		updated, err := requireCLIUpdateIfAvailable(ctx, cfg)
		if err != nil {
			return err
		}
		if updated {
			return nil
		}

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

func requireCLIUpdateIfAvailable(ctx context.Context, cfg config.Config) (bool, error) {
	if Version == "" || Version == "dev" {
		return false, nil
	}

	manifest, err := releases.Fetch(ctx, cfg.ManifestURL)
	if err != nil {
		output.Hintf("Kon update-check niet uitvoeren: %v", err)
		return false, nil
	}

	newer, err := manifest.IsNewerCLI(Version)
	if err != nil {
		output.Hintf("Kon update-versie niet vergelijken: %v", err)
		return false, nil
	}
	if !newer {
		return false, nil
	}

	accepted, err := tui.AskRequiredUpdate(Version, manifest.CLI.Version)
	if err != nil {
		return false, err
	}
	if !accepted {
		return false, fmt.Errorf("update verplicht: run `devkit self-update` en start DevKit daarna opnieuw")
	}

	if err := update.SelfUpdate(ctx, manifest, Version, platformFrom(ctx)); err != nil {
		return false, err
	}
	return true, nil
}

// Execute is the entrypoint called from main.
func Execute(ctx context.Context, version string) {
	Version = version

	plat := platform.Detect()
	ctx = withPlatform(ctx, plat)

	cfg, err := config.Load()
	if err != nil {
		output.Fatalf("Failed to load config: %v", err)
	}
	ctx = withConfig(ctx, cfg)

	s, err := state.Load()
	if err != nil {
		output.Fatalf("Failed to load state: %v", err)
	}
	ctx = withState(ctx, s)

	// Print banner only when a subcommand is invoked directly (not TUI, not completion).
	if len(os.Args) >= 2 && os.Args[1] != "completion" && os.Args[1] != "__complete" {
		output.Banner(cfg.Brand, version)
	}

	if err := rootCmd.ExecuteContext(ctx); err != nil {
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
		newScaffoldCmd(),
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
