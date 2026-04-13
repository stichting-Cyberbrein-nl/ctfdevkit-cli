package cli

import (
	"context"
	"fmt"
	"os"
	"path/filepath"

	"github.com/spf13/cobra"
	"github.com/stichting-Cyberbrein-nl/ctfdevkit-cli/internal/assignments"
	"github.com/stichting-Cyberbrein-nl/ctfdevkit-cli/internal/browser"
	"github.com/stichting-Cyberbrein-nl/ctfdevkit-cli/internal/certs"
	"github.com/stichting-Cyberbrein-nl/ctfdevkit-cli/internal/config"
	"github.com/stichting-Cyberbrein-nl/ctfdevkit-cli/internal/doctor"
	"github.com/stichting-Cyberbrein-nl/ctfdevkit-cli/internal/docker"
	"github.com/stichting-Cyberbrein-nl/ctfdevkit-cli/internal/hosts"
	"github.com/stichting-Cyberbrein-nl/ctfdevkit-cli/internal/output"
	"github.com/stichting-Cyberbrein-nl/ctfdevkit-cli/internal/payload"
	"github.com/stichting-Cyberbrein-nl/ctfdevkit-cli/internal/platform"
	"github.com/stichting-Cyberbrein-nl/ctfdevkit-cli/internal/ports"
	"github.com/stichting-Cyberbrein-nl/ctfdevkit-cli/internal/prereqs"
	"github.com/stichting-Cyberbrein-nl/ctfdevkit-cli/internal/state"
	"github.com/stichting-Cyberbrein-nl/ctfdevkit-cli/internal/tui"
)

func newSetupCmd() *cobra.Command {
	return &cobra.Command{
		Use:   "setup",
		Short: "Run the full environment setup wizard",
		Long:  `setup checks prerequisites, installs the payload, configures hosts/certs, starts Docker, and opens the app.`,
		RunE: func(cmd *cobra.Command, args []string) error {
			return runSetup(cmd.Context(), false)
		},
	}
}

func runSetup(ctx context.Context, skipOpen bool) error {
	plat := platformFrom(ctx)
	cfg := configFrom(ctx)
	s := stateFrom(ctx)

	const totalSteps = 6

	// Step 1 — Prerequisites
	output.Step(1, totalSteps, "Checking prerequisites")
	if err := prereqs.Check(ctx, plat); err != nil {
		return err
	}

	// Step 2 — Payload
	output.Step(2, totalSteps, "Installing payload")
	if err := payload.EnsureInstalled(ctx, &s, cfg.Version); err != nil {
		return err
	}

	// Step 3 — Disk sanity
	output.Step(3, totalSteps, "Checking disk space")
	if err := docker.MaybeFreeSpace(ctx); err != nil {
		output.Warnf("Disk check warning: %v", err)
	}

	// Step 4 — Hosts binding
	output.Step(4, totalSteps, "Configuring hosts file")
	if err := hosts.EnsureBinding(cfg.BindIP, cfg.Domain, plat); err != nil {
		return err
	}

	// Step 5 — TLS certificates
	output.Step(5, totalSteps, "Generating TLS certificates")
	certsDir, err := platform.CertsDir()
	if err != nil {
		return err
	}
	if err := certs.Generate(certsDir, cfg.Domain, cfg.BindIP, false, plat); err != nil {
		return err
	}

	// Step 6 — Resolve ports + assignments + start containers
	output.Step(6, totalSteps, "Resolving ports and starting containers")

	pair, err := ports.ResolvePorts(ctx, 80, 443, plat)
	if err != nil {
		return err
	}

	composeDir, err := payload.ComposeDir(s)
	if err != nil {
		return err
	}

	// Resolve assignments path — auto-detect, prompt if needed, save permanently.
	assignmentsPath, err := resolveAssignmentsPath(cfg, composeDir)
	if err != nil {
		return err
	}
	// Persist path to config so future runs skip the prompt.
	if assignmentsPath != cfg.AssignmentsPath {
		cfg.AssignmentsPath = assignmentsPath
		if saveErr := config.Save(cfg); saveErr != nil {
			output.Warnf("Could not save assignments path: %v", saveErr)
		}
	}

	appURL := buildURL(cfg.Domain, pair.HTTPS)
	if appURL != cfg.URL {
		output.Warnf("Using non-standard port — app will be at %s", appURL)
	}

	if err := writeComposeEnv(composeDir, pair.HTTP, pair.HTTPS, appURL, assignmentsPath); err != nil {
		return fmt.Errorf("writing compose .env: %w", err)
	}

	if err := docker.ComposeUp(ctx, composeDir); err != nil {
		return err
	}

	output.Info("Waiting for application health check...")
	if err := doctor.WaitForReady(ctx, composeDir, cfg.AppContainer); err != nil {
		return err
	}

	_ = doctor.Run(ctx, composeDir, cfg.AppContainer)

	output.Divider()
	output.Successf("Setup complete!  →  %s", appURL)
	output.Hint("Assignments loaded from: " + assignmentsPath)
	output.Divider()

	s.PayloadPath = composeDir
	if err := state.Save(s); err != nil {
		output.Warnf("Could not save state: %v", err)
	}

	if !skipOpen {
		_ = browser.Open(appURL, plat)
	}

	return nil
}

// resolveAssignmentsPath returns the assignments directory to use, in this priority:
//  1. Already in config → use it (show confirmation)
//  2. Auto-detected valid path → offer it (user can accept or change)
//  3. Not found → interactive prompt
//  4. User skips → fallback to ./assignments inside payload dir
func resolveAssignmentsPath(cfg config.Config, composeDir string) (string, error) {
	// Already configured and valid — just use it.
	if cfg.AssignmentsPath != "" {
		count := assignments.Count(cfg.AssignmentsPath)
		output.Successf("Assignments: %s (%d found)", cfg.AssignmentsPath, count)
		return cfg.AssignmentsPath, nil
	}

	// Try auto-detect first.
	detected := assignments.AutoDetect()
	if detected != "" && assignments.IsValid(detected) {
		count := assignments.Count(detected)
		output.Successf("Auto-detected assignments: %s (%d found)", detected, count)
		return detected, nil
	}

	// Nothing found — show interactive prompt.
	output.Warn("Could not find an assignments folder automatically.")
	path, err := tui.AskAssignmentsPath()
	if err != nil {
		return "", err
	}

	if path != "" {
		if err := os.MkdirAll(path, 0755); err != nil {
			output.Warnf("Could not create %s: %v", path, err)
		}
		output.Successf("Assignments path set to: %s", path)
		return path, nil
	}

	// User skipped — use fallback inside payload dir.
	fallback := filepath.Join(composeDir, "assignments")
	if err := os.MkdirAll(fallback, 0755); err != nil {
		return "", fmt.Errorf("creating default assignments dir: %w", err)
	}
	output.Infof("Using default assignments directory: %s", fallback)
	output.Hint("Run `devkit config-assignments` to change it later.")
	return fallback, nil
}

// writeComposeEnv writes a .env file that Docker Compose reads for port, URL and path variables.
func writeComposeEnv(composeDir string, httpPort, httpsPort int, appURL, assignmentsPath string) error {
	content := fmt.Sprintf(
		"DEVKIT_HTTP_PORT=%d\nDEVKIT_HTTPS_PORT=%d\nAPP_URL=%s\nASSIGNMENTS_PATH=%s\n",
		httpPort, httpsPort, appURL, assignmentsPath,
	)
	return os.WriteFile(filepath.Join(composeDir, ".env"), []byte(content), 0644)
}

// buildURL constructs the app URL, omitting the port for standard 443.
func buildURL(domain string, httpsPort int) string {
	if httpsPort == 443 {
		return "https://" + domain
	}
	return fmt.Sprintf("https://%s:%d", domain, httpsPort)
}
