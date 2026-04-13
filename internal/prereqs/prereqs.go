// Package prereqs checks and installs prerequisite tooling.
package prereqs

import (
	"context"
	"fmt"
	"os"
	"os/exec"
	"runtime"
	"strings"

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
			return fmt.Errorf("homebrew not found — cannot auto-install Docker on macOS")
		}
		return exec.CommandContext(ctx, "brew", "install", "--cask", "docker").Run()

	case platform.OSWindows:
		if _, err := exec.LookPath("winget.exe"); err != nil {
			return fmt.Errorf("winget not found — install Docker Desktop manually: https://docs.docker.com/desktop/install/windows-install/")
		}
		cmd := exec.CommandContext(ctx, "winget.exe", "install", "--id", "Docker.DockerDesktop", "-e", "--silent")
		cmd.Stdout = os.Stdout
		cmd.Stderr = os.Stderr
		return cmd.Run()

	default:
		// Linux (including WSL)
		return installDockerLinux(ctx)
	}
}

// installDockerLinux detects the distro and uses the appropriate package manager.
func installDockerLinux(ctx context.Context) error {
	distro := detectLinuxDistro()
	output.Infof("Detected Linux distro: %s", distro)

	switch distro {
	case "arch":
		return installDockerArch(ctx)
	case "debian", "ubuntu", "kali", "raspbian", "linuxmint", "pop":
		return installDockerDebian(ctx)
	case "fedora":
		return installDockerFedora(ctx)
	case "rhel", "centos", "rocky", "almalinux":
		return installDockerRHEL(ctx)
	case "opensuse", "sles":
		return installDockerOpenSUSE(ctx)
	default:
		// Unknown distro — try the official convenience script.
		return installDockerScript(ctx)
	}
}

// detectLinuxDistro reads /etc/os-release to identify the distribution.
func detectLinuxDistro() string {
	if runtime.GOOS != "linux" {
		return "unknown"
	}

	// Arch Linux has its own release file.
	if _, err := os.Stat("/etc/arch-release"); err == nil {
		return "arch"
	}

	data, err := os.ReadFile("/etc/os-release")
	if err != nil {
		// Fallback: check package managers.
		return detectDistroByPackageManager()
	}

	lower := strings.ToLower(string(data))
	for _, line := range strings.Split(lower, "\n") {
		if !strings.HasPrefix(line, "id=") {
			continue
		}
		id := strings.Trim(strings.TrimPrefix(line, "id="), `"' `)
		return id
	}

	return detectDistroByPackageManager()
}

// detectDistroByPackageManager is a fallback when /etc/os-release is absent.
func detectDistroByPackageManager() string {
	if _, err := exec.LookPath("pacman"); err == nil {
		return "arch"
	}
	if _, err := exec.LookPath("apt-get"); err == nil {
		return "debian"
	}
	if _, err := exec.LookPath("dnf"); err == nil {
		return "fedora"
	}
	if _, err := exec.LookPath("yum"); err == nil {
		return "rhel"
	}
	if _, err := exec.LookPath("zypper"); err == nil {
		return "opensuse"
	}
	return "unknown"
}

// installDockerArch installs Docker on Arch Linux via pacman.
func installDockerArch(ctx context.Context) error {
	output.Info("Installing Docker via pacman...")

	// Update package database first.
	sync := exec.CommandContext(ctx, "sudo", "pacman", "-Sy", "--noconfirm")
	sync.Stdout = os.Stdout
	sync.Stderr = os.Stderr
	if err := sync.Run(); err != nil {
		return fmt.Errorf("pacman -Sy failed: %w", err)
	}

	install := exec.CommandContext(ctx, "sudo", "pacman", "-S", "--noconfirm", "docker", "docker-compose")
	install.Stdout = os.Stdout
	install.Stderr = os.Stderr
	if err := install.Run(); err != nil {
		return fmt.Errorf("pacman install failed: %w", err)
	}

	return enableAndStartDocker(ctx)
}

// installDockerDebian installs Docker on Debian / Ubuntu / Kali via apt-get.
func installDockerDebian(ctx context.Context) error {
	output.Info("Installing Docker via apt-get...")

	steps := [][]string{
		{"sudo", "apt-get", "update", "-qq"},
		{"sudo", "apt-get", "install", "-y", "-qq",
			"ca-certificates", "curl", "gnupg", "lsb-release"},
	}

	for _, args := range steps {
		cmd := exec.CommandContext(ctx, args[0], args[1:]...)
		cmd.Stdout = os.Stdout
		cmd.Stderr = os.Stderr
		if err := cmd.Run(); err != nil {
			return fmt.Errorf("%s failed: %w", strings.Join(args, " "), err)
		}
	}

	// Add Docker's official GPG key and repository, then install.
	script := `
set -e
install -m 0755 -d /etc/apt/keyrings
curl -fsSL https://download.docker.com/linux/$(. /etc/os-release && echo "$ID")/gpg \
    | gpg --dearmor --yes -o /etc/apt/keyrings/docker.gpg
chmod a+r /etc/apt/keyrings/docker.gpg
echo "deb [arch=$(dpkg --print-architecture) signed-by=/etc/apt/keyrings/docker.gpg] \
    https://download.docker.com/linux/$(. /etc/os-release && echo "$ID") \
    $(. /etc/os-release && echo "$VERSION_CODENAME") stable" \
    | tee /etc/apt/sources.list.d/docker.list > /dev/null
apt-get update -qq
apt-get install -y -qq docker-ce docker-ce-cli containerd.io docker-buildx-plugin docker-compose-plugin
`
	cmd := exec.CommandContext(ctx, "sudo", "sh", "-c", script)
	cmd.Stdout = os.Stdout
	cmd.Stderr = os.Stderr
	if err := cmd.Run(); err != nil {
		// Fall back to get.docker.com script.
		output.Warn("Official repo install failed, trying convenience script...")
		return installDockerScript(ctx)
	}

	return enableAndStartDocker(ctx)
}

// installDockerFedora installs Docker on Fedora via dnf.
func installDockerFedora(ctx context.Context) error {
	output.Info("Installing Docker via dnf...")

	steps := [][]string{
		{"sudo", "dnf", "-y", "install", "dnf-plugins-core"},
		{"sudo", "dnf", "config-manager", "--add-repo",
			"https://download.docker.com/linux/fedora/docker-ce.repo"},
		{"sudo", "dnf", "-y", "install",
			"docker-ce", "docker-ce-cli", "containerd.io",
			"docker-buildx-plugin", "docker-compose-plugin"},
	}

	for _, args := range steps {
		cmd := exec.CommandContext(ctx, args[0], args[1:]...)
		cmd.Stdout = os.Stdout
		cmd.Stderr = os.Stderr
		if err := cmd.Run(); err != nil {
			return fmt.Errorf("%s failed: %w", strings.Join(args, " "), err)
		}
	}

	return enableAndStartDocker(ctx)
}

// installDockerRHEL installs Docker on RHEL / CentOS / Rocky / AlmaLinux via yum/dnf.
func installDockerRHEL(ctx context.Context) error {
	output.Info("Installing Docker via yum/dnf...")

	pm := "dnf"
	if _, err := exec.LookPath("dnf"); err != nil {
		pm = "yum"
	}

	steps := [][]string{
		{"sudo", pm, "-y", "install", "yum-utils"},
		{"sudo", "yum-config-manager", "--add-repo",
			"https://download.docker.com/linux/centos/docker-ce.repo"},
		{"sudo", pm, "-y", "install",
			"docker-ce", "docker-ce-cli", "containerd.io",
			"docker-buildx-plugin", "docker-compose-plugin"},
	}

	for _, args := range steps {
		cmd := exec.CommandContext(ctx, args[0], args[1:]...)
		cmd.Stdout = os.Stdout
		cmd.Stderr = os.Stderr
		if err := cmd.Run(); err != nil {
			return fmt.Errorf("%s failed: %w", strings.Join(args, " "), err)
		}
	}

	return enableAndStartDocker(ctx)
}

// installDockerOpenSUSE installs Docker on openSUSE / SLES via zypper.
func installDockerOpenSUSE(ctx context.Context) error {
	output.Info("Installing Docker via zypper...")

	steps := [][]string{
		{"sudo", "zypper", "--non-interactive", "install", "docker", "docker-compose"},
	}

	for _, args := range steps {
		cmd := exec.CommandContext(ctx, args[0], args[1:]...)
		cmd.Stdout = os.Stdout
		cmd.Stderr = os.Stderr
		if err := cmd.Run(); err != nil {
			return fmt.Errorf("%s failed: %w", strings.Join(args, " "), err)
		}
	}

	return enableAndStartDocker(ctx)
}

// installDockerScript uses the official get.docker.com convenience script as a
// last resort for unknown distributions.
func installDockerScript(ctx context.Context) error {
	if _, err := exec.LookPath("curl"); err != nil {
		return fmt.Errorf("curl is required for auto-install on this distro — install Docker manually: https://docs.docker.com/get-docker/")
	}

	output.Info("Installing Docker via official convenience script (get.docker.com)...")
	cmd := exec.CommandContext(ctx, "sh", "-c", "curl -fsSL https://get.docker.com | sudo sh")
	cmd.Stdout = os.Stdout
	cmd.Stderr = os.Stderr
	if err := cmd.Run(); err != nil {
		return fmt.Errorf("convenience script failed: %w", err)
	}

	return enableAndStartDocker(ctx)
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
		_ = cmd.Run() // non-fatal — systemctl may not exist (e.g. inside Docker)
	}

	// Add current user to the docker group so sudo is not required for every command.
	if user := os.Getenv("USER"); user != "" && user != "root" {
		cmd := exec.CommandContext(ctx, "sudo", "usermod", "-aG", "docker", user)
		cmd.Stdout = os.Stdout
		cmd.Stderr = os.Stderr
		if err := cmd.Run(); err == nil {
			output.Hint("Added to docker group — log out and back in (or run: newgrp docker) for this to take effect.")
		}
	}

	return nil
}
