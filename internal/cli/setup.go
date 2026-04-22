package cli

import (
	"context"
	"fmt"
	"os"
	"path/filepath"

	"github.com/spf13/cobra"
	"github.com/stichting-Cyberbrein-nl/ctfdevkit-cli/internal/browser"
	"github.com/stichting-Cyberbrein-nl/ctfdevkit-cli/internal/certs"
	"github.com/stichting-Cyberbrein-nl/ctfdevkit-cli/internal/config"
	"github.com/stichting-Cyberbrein-nl/ctfdevkit-cli/internal/docker"
	"github.com/stichting-Cyberbrein-nl/ctfdevkit-cli/internal/doctor"
	"github.com/stichting-Cyberbrein-nl/ctfdevkit-cli/internal/hosts"
	"github.com/stichting-Cyberbrein-nl/ctfdevkit-cli/internal/output"
	"github.com/stichting-Cyberbrein-nl/ctfdevkit-cli/internal/payload"
	"github.com/stichting-Cyberbrein-nl/ctfdevkit-cli/internal/platform"
	"github.com/stichting-Cyberbrein-nl/ctfdevkit-cli/internal/ports"
	"github.com/stichting-Cyberbrein-nl/ctfdevkit-cli/internal/prereqs"
	"github.com/stichting-Cyberbrein-nl/ctfdevkit-cli/internal/state"
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
	bindIP := effectiveBindIP(cfg, plat)

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
	if err := hosts.EnsureBinding(bindIP, cfg.Domain, plat); err != nil {
		return err
	}

	// Step 5 — TLS certificates
	output.Step(5, totalSteps, "Generating TLS certificates")
	certsDir, err := platform.CertsDir()
	if err != nil {
		return err
	}
	if err := certs.Generate(certsDir, cfg.Domain, bindIP, false, plat); err != nil {
		return err
	}

	// Step 6 — Resolve ports + assignments + start containers
	output.Step(6, totalSteps, "Resolving ports and starting containers")

	pair, err := ports.ResolvePortsForBindIP(ctx, 80, 443, bindIP, plat)
	if err != nil {
		return err
	}

	composeDir, err := payload.ComposeDir(s)
	if err != nil {
		return err
	}

	// Resolve assignments path — auto-detect, prompt if needed, save permanently.
	assignmentsPath, err := resolveAssignmentsRepoPath(ctx, cfg, plat, defaultAssignmentsPrompter(), defaultAssignmentsFlowOps())
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

	if err := writeComposeEnv(composeDir, bindIP, pair.HTTP, pair.HTTPS, appURL, assignmentsPath); err != nil {
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

// writeComposeEnv writes a .env file that Docker Compose reads for port, URL and path variables.
func writeComposeEnv(composeDir, bindIP string, httpPort, httpsPort int, appURL, assignmentsPath string) error {
	content := fmt.Sprintf(
		"DEVKIT_BIND_IP=%s\nDEVKIT_HTTP_PORT=%d\nDEVKIT_HTTPS_PORT=%d\nAPP_URL=%s\nASSIGNMENTS_PATH=%s\n",
		bindIP, httpPort, httpsPort, appURL, assignmentsPath,
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

func effectiveBindIP(cfg config.Config, plat platform.Platform) string {
	if plat.OS == platform.OSWindows && (cfg.BindIP == "" || cfg.BindIP == "127.0.0.1") {
		return "127.0.0.2"
	}
	if cfg.BindIP == "" {
		return "127.0.0.1"
	}
	return cfg.BindIP
}
