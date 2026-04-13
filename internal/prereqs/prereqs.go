// Package prereqs checks and installs prerequisite tooling.
package prereqs

import (
	"context"
	"fmt"
	"os/exec"
	"runtime"

	"github.com/stichting-Cyberbrein-nl/ctfdevkit-cli/internal/certs"
	"github.com/stichting-Cyberbrein-nl/ctfdevkit-cli/internal/docker"
	"github.com/stichting-Cyberbrein-nl/ctfdevkit-cli/internal/output"
	"github.com/stichting-Cyberbrein-nl/ctfdevkit-cli/internal/platform"
)

// Check verifies all prerequisites are present.
// It installs missing tools on supported platforms.
func Check(ctx context.Context, plat platform.Platform) error {
	if err := ensureDocker(ctx, plat); err != nil {
		return err
	}
	if err := ensureMkcert(plat); err != nil {
		return err
	}
	return nil
}

func ensureDocker(ctx context.Context, plat platform.Platform) error {
	if !docker.IsAvailable() {
		output.Warn("Docker is not installed.")
		if err := installDocker(ctx, plat); err != nil {
			return fmt.Errorf("docker installation failed: %w\n\nPlease install Docker manually: https://docs.docker.com/get-docker/", err)
		}
		if !docker.IsAvailable() {
			return fmt.Errorf("docker still not found after installation — please restart your terminal")
		}
	}

	if !docker.IsRunning(ctx) {
		output.Warn("Docker daemon is not running.")
		_ = docker.TryLaunch(ctx)

		waitCtx, cancel := context.WithTimeout(ctx, 120_000_000_000) // 120s
		defer cancel()
		if err := docker.WaitForDaemon(waitCtx); err != nil {
			return err
		}
	}

	if err := docker.Validate(ctx); err != nil {
		return err
	}

	output.Success("Docker is ready")
	return nil
}

func ensureMkcert(plat platform.Platform) error {
	if certs.IsMkcertInstalled() {
		output.Success("mkcert is installed")
		return nil
	}

	output.Warn("mkcert is not installed.")
	if err := certs.InstallMkcert(plat); err != nil {
		return fmt.Errorf("mkcert installation failed: %w\n\nInstall manually: https://github.com/FiloSottile/mkcert", err)
	}

	if !certs.IsMkcertInstalled() {
		return fmt.Errorf("mkcert still not found after installation — please restart your terminal")
	}

	output.Success("mkcert installed")
	return nil
}

func installDocker(ctx context.Context, plat platform.Platform) error {
	output.Info("Attempting to install Docker...")

	switch plat.OS {
	case platform.OSMacOS:
		if _, err := exec.LookPath("brew"); err != nil {
			return fmt.Errorf("homebrew not found — cannot auto-install Docker")
		}
		cmd := exec.CommandContext(ctx, "brew", "install", "--cask", "docker")
		return cmd.Run()
	case platform.OSWindows:
		cmd := exec.CommandContext(ctx, "winget.exe", "install", "--id", "Docker.DockerDesktop", "-e", "--silent")
		return cmd.Run()
	default:
		// Linux: print instructions, cannot safely auto-install.
		output.Plain("Please install Docker using your distribution's package manager.")
		output.Hint("Ubuntu/Debian: curl -fsSL https://get.docker.com | sh")

		if runtime.GOOS == "linux" {
			// Try the convenience script.
			cmd := exec.CommandContext(ctx, "sh", "-c", "curl -fsSL https://get.docker.com | sh")
			return cmd.Run()
		}
		return fmt.Errorf("cannot auto-install Docker on this platform")
	}
}
