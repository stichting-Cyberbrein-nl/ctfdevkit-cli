// Package prereqs checks and installs prerequisite tooling.
package prereqs

import (
	"context"
	"fmt"
	"os"
	"time"

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
			return fmt.Errorf("docker still not found after installation - please restart your terminal")
		}
	}

	if !docker.IsRunning(ctx) {
		if err := ensureDockerSocketAccess(ctx); err != nil {
			return err
		}

		output.Warn("Docker daemon is not running.")
		_ = docker.TryLaunch(ctx)

		waitCtx, cancel := context.WithTimeout(ctx, 120*time.Second)
		defer cancel()
		if err := docker.WaitForDaemon(waitCtx); err != nil {
			if accessErr := ensureDockerSocketAccess(ctx); accessErr != nil {
				return accessErr
			}
			return err
		}
	}

	if err := docker.Validate(ctx); err != nil {
		return err
	}

	output.Success("Docker is ready")
	return nil
}

func ensureDockerSocketAccess(ctx context.Context) error {
	out, err := docker.Info(ctx)
	if err == nil || !docker.IsPermissionError(out) {
		return nil
	}

	user := os.Getenv("USER")
	if user == "" {
		user = "the current user"
	}

	return dockerSocketAccessError(user, addCurrentUserToDockerGroup(ctx))
}

func dockerSocketAccessError(user string, addedToGroup bool) error {
	if addedToGroup {
		return fmt.Errorf(`Docker is running, but this terminal does not have access to Docker yet.
This is expected right after DevKit added the user "%s" to the docker group.

Choose one of these options:
1. Fastest option: run: newgrp docker
2. Easiest option: close this terminal and open a new terminal
3. If in doubt: restart the PC with: sudo reboot

Then start again with: devkit`, user)
	}

	return fmt.Errorf(`Docker is running, but this user does not have access to Docker.
Fix it with:
  sudo usermod -aG docker $USER
  newgrp docker
  devkit

If in doubt: restart the PC with: sudo reboot`)
}

func ensureMkcert(plat platform.Platform) error {
	if certs.IsMkcertInstalled() {
		if err := certs.EnsureTrustStoreTools(plat); err != nil {
			return fmt.Errorf("certificate trust helper installation failed: %w", err)
		}
		output.Success("mkcert is installed")
		return nil
	}

	output.Warn("mkcert is not installed.")
	if err := certs.InstallMkcert(plat); err != nil {
		return fmt.Errorf("mkcert installation failed: %w\n\nInstall manually: https://github.com/FiloSottile/mkcert", err)
	}

	if !certs.IsMkcertInstalled() {
		return fmt.Errorf("mkcert still not found after installation - please restart your terminal")
	}
	if err := certs.EnsureTrustStoreTools(plat); err != nil {
		return fmt.Errorf("certificate trust helper installation failed: %w", err)
	}

	output.Success("mkcert installed")
	return nil
}
