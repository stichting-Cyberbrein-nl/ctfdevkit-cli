package cli

import (
	"context"
	"errors"
	"os"
	"time"

	"github.com/spf13/cobra"
	"github.com/stichting-Cyberbrein-nl/ctfdevkit-cli/internal/config"
	"github.com/stichting-Cyberbrein-nl/ctfdevkit-cli/internal/output"
	"github.com/stichting-Cyberbrein-nl/ctfdevkit-cli/internal/releases"
	"github.com/stichting-Cyberbrein-nl/ctfdevkit-cli/internal/state"
	"github.com/stichting-Cyberbrein-nl/ctfdevkit-cli/internal/tui"
	"github.com/stichting-Cyberbrein-nl/ctfdevkit-cli/internal/update"
)

// errRestartRequired signals that the binary was replaced and the command must be re-run.
// Execute turns it into a friendly message and a zero exit code.
var errRestartRequired = errors.New("devkit has been updated; please run your command again")

// updateCheckInterval throttles the manifest fetch so routine commands stay snappy.
const updateCheckInterval = time.Hour

// commandsSkippingUpdateCheck must never trigger an update: either they are the update
// machinery itself (recursion) or they must keep working on a broken/offline install.
var commandsSkippingUpdateCheck = map[string]bool{
	"self-update": true,
	"update":      true,
	"version":     true,
	"help":        true,
	"completion":  true,
	"__complete":  true,
	"doctor":      true,
}

// commandsUpdatingPayload also pull a newer Docker image, because they are the commands
// that actually (re)start the stack. Other commands only check the CLI binary, so a
// `devkit logs` never blocks on a multi-minute image pull.
var commandsUpdatingPayload = map[string]bool{
	"up":    true,
	"setup": true,
}

// ensureUpToDate runs before every real command. It self-updates the CLI when a newer
// release exists and, for the commands that start the stack, pulls a newer payload image.
// Network failures are reported but never block the command the user actually asked for.
func ensureUpToDate(cmd *cobra.Command) error {
	// AI tools start `mcp serve` without a terminal; it must never self-update and exit.
	if cmd.Annotations[skipUpdateCheckAnnotation] == "true" {
		return nil
	}

	ctx := cmd.Context()
	cfg := configFrom(ctx)
	s := stateFrom(ctx)

	if !shouldCheckForUpdates(cmd.Name(), Version, s.LastUpdateCheck) {
		return nil
	}

	wantsPayload := commandsUpdatingPayload[cmd.Name()]

	manifest, err := releases.Fetch(ctx, cfg.ManifestURL)
	if err != nil {
		output.Hintf("Could not check for updates: %v", err)
		return nil
	}

	s.LastUpdateCheck = time.Now()
	s.CLIVersion = Version
	if err := state.Save(s); err != nil {
		output.Hintf("Could not save update check: %v", err)
	}

	if err := applyCLIUpdate(ctx, manifest); err != nil {
		return err
	}

	if wantsPayload {
		applyPayloadUpdate(ctx, &cfg, &s, manifest)
	}

	// The command reads config and state from its context; hand it the updated copies
	// so `setup` installs the version just pulled instead of the stale one.
	cmd.SetContext(withState(withConfig(ctx, cfg), s))
	return nil
}

// shouldCheckForUpdates decides whether this invocation is allowed to hit the network.
// Commands that start the stack always check; everything else is throttled so routine
// commands stay fast.
func shouldCheckForUpdates(cmdName, version string, lastCheck time.Time) bool {
	if commandsSkippingUpdateCheck[cmdName] {
		return false
	}

	// A locally built binary has no meaningful version to compare against.
	if version == "" || version == "dev" {
		return false
	}

	if commandsUpdatingPayload[cmdName] {
		return true
	}

	return time.Since(lastCheck) >= updateCheckInterval
}

// applyCLIUpdate replaces the binary when the manifest advertises a newer CLI.
// It returns errRestartRequired after a successful update, because the running process
// is still the old build.
func applyCLIUpdate(ctx context.Context, manifest *releases.Manifest) error {
	newer, err := manifest.IsNewerCLI(Version)
	if err != nil {
		output.Hintf("Could not compare CLI version: %v", err)
		return nil
	}
	if !newer {
		return nil
	}

	// Without a terminal there is nobody to ask, so update silently rather than
	// hanging a script on a prompt it can never answer.
	if isInteractive() {
		accepted, err := tui.AskRequiredUpdate(Version, manifest.CLI.Version)
		if err != nil {
			return err
		}
		if !accepted {
			return errors.New("update required: run `devkit self-update` and try again")
		}
	} else {
		output.Infof("New CLI available (%s → %s), updating automatically...", Version, manifest.CLI.Version)
	}

	if err := update.SelfUpdate(ctx, manifest, Version, platformFrom(ctx)); err != nil {
		return err
	}

	return errRestartRequired
}

// applyPayloadUpdate pulls a newer DevKit image. A failure here is reported but not fatal:
// the installed version still works, so `up` should continue with what is installed.
func applyPayloadUpdate(ctx context.Context, cfg *config.Config, s *state.State, manifest *releases.Manifest) {
	newer, err := manifest.IsNewerPayload(s.PayloadVersion)
	if err != nil || !newer {
		return
	}

	if s.PayloadVersion == "" {
		output.Infof("Downloading DevKit v%s...", manifest.Payload.Version)
	} else {
		output.Infof("New DevKit version available: v%s → v%s", s.PayloadVersion, manifest.Payload.Version)
	}
	if err := installDevKitVersion(ctx, cfg, s, manifest.Payload); err != nil {
		output.Warnf("DevKit update failed, current version remains active: %v", err)
		return
	}

	output.Successf("DevKit updated to v%s", manifest.Payload.Version)
}

// isInteractive reports whether stdin is a terminal we can prompt on.
func isInteractive() bool {
	info, err := os.Stdin.Stat()
	if err != nil {
		return false
	}
	return info.Mode()&os.ModeCharDevice != 0
}
