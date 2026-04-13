package prereqs

import (
	"context"
	"fmt"
	"os"
	"os/exec"

	"github.com/stichting-Cyberbrein-nl/ctfdevkit-cli/internal/output"
	"github.com/stichting-Cyberbrein-nl/ctfdevkit-cli/internal/platform"
)

func installDocker(ctx context.Context, plat platform.Platform) error {
	output.Info("Attempting to install Docker...")

	switch plat.OS {
	case platform.OSMacOS:
		if _, err := exec.LookPath("brew"); err != nil {
			return fmt.Errorf("homebrew not found - cannot auto-install Docker on macOS")
		}
		return exec.CommandContext(ctx, "brew", "install", "--cask", "docker").Run()

	case platform.OSWindows:
		if _, err := exec.LookPath("winget.exe"); err != nil {
			return fmt.Errorf("winget not found - install Docker Desktop manually: https://docs.docker.com/desktop/install/windows-install/")
		}
		cmd := exec.CommandContext(ctx, "winget.exe", "install", "--id", "Docker.DockerDesktop", "-e", "--silent")
		cmd.Stdout = os.Stdout
		cmd.Stderr = os.Stderr
		return cmd.Run()

	default:
		// Linux, including WSL.
		return installDockerLinux(ctx)
	}
}

// enableAndStartDocker enables the Docker service and adds the current user to
// the docker group so it can be used without sudo.
func enableAndStartDocker(ctx context.Context) error {
	// Enable + start the daemon.
	for _, args := range [][]string{
		{"sudo", "systemctl", "enable", "docker"},
		{"sudo", "systemctl", "start", "docker"},
	} {
		cmd := exec.CommandContext(ctx, args[0], args[1:]...)
		cmd.Stdout = os.Stdout
		cmd.Stderr = os.Stderr
		_ = cmd.Run() // non-fatal - systemctl may not exist, e.g. inside Docker.
	}

	addCurrentUserToDockerGroup(ctx)

	return nil
}

func addCurrentUserToDockerGroup(ctx context.Context) bool {
	if user := os.Getenv("USER"); user != "" && user != "root" {
		cmd := exec.CommandContext(ctx, "sudo", "usermod", "-aG", "docker", user)
		cmd.Stdout = os.Stdout
		cmd.Stderr = os.Stderr
		if err := cmd.Run(); err == nil {
			output.Hint("Gebruiker toegevoegd aan docker groep. Activeer dit met: newgrp docker, een nieuwe terminal, of sudo reboot.")
			return true
		}
	}
	return false
}
