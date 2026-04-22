package prereqs

import (
	"context"
	"fmt"
	"os"
	"os/exec"

	"github.com/stichting-Cyberbrein-nl/ctfdevkit-cli/internal/output"
	"github.com/stichting-Cyberbrein-nl/ctfdevkit-cli/internal/platform"
)

var (
	gitLookPath  = exec.LookPath
	gitInstaller = installGit
)

func EnsureGit(ctx context.Context, plat platform.Platform) error {
	if IsGitInstalled() {
		output.Success("Git is installed")
		return nil
	}

	output.Warn("Git is not installed.")
	if err := gitInstaller(ctx, plat); err != nil {
		return fmt.Errorf("git installation failed: %w\n\nInstall manually: %s", err, gitInstallHint(plat))
	}
	if !IsGitInstalled() {
		return fmt.Errorf("git still not found after installation - please restart your terminal")
	}

	output.Success("Git installed")
	return nil
}

func IsGitInstalled() bool {
	_, err := gitLookPath("git")
	return err == nil
}

func installGit(ctx context.Context, plat platform.Platform) error {
	switch plat.OS {
	case platform.OSMacOS:
		if _, err := exec.LookPath("brew"); err == nil {
			return exec.CommandContext(ctx, "brew", "install", "git").Run()
		}
		return fmt.Errorf("homebrew not found")
	case platform.OSWindows:
		if _, err := exec.LookPath("winget.exe"); err == nil {
			cmd := exec.CommandContext(ctx, "winget.exe", "install", "--id", "Git.Git", "-e", "--silent")
			cmd.Stdout = os.Stdout
			cmd.Stderr = os.Stderr
			return cmd.Run()
		}
		return fmt.Errorf("winget not found")
	default:
		return installGitLinux(ctx)
	}
}

func installGitLinux(ctx context.Context) error {
	steps := []struct {
		name string
		args []string
	}{
		{name: "apt-get", args: []string{"sudo", "apt-get", "install", "-y", "git"}},
		{name: "pacman", args: []string{"sudo", "pacman", "-S", "--noconfirm", "git"}},
		{name: "dnf", args: []string{"sudo", "dnf", "install", "-y", "git"}},
		{name: "yum", args: []string{"sudo", "yum", "install", "-y", "git"}},
		{name: "zypper", args: []string{"sudo", "zypper", "--non-interactive", "install", "git"}},
	}

	for _, step := range steps {
		if _, err := exec.LookPath(step.name); err != nil {
			continue
		}
		cmd := exec.CommandContext(ctx, step.args[0], step.args[1:]...)
		cmd.Stdout = os.Stdout
		cmd.Stderr = os.Stderr
		return cmd.Run()
	}

	return fmt.Errorf("no supported package manager found")
}

func gitInstallHint(plat platform.Platform) string {
	switch plat.OS {
	case platform.OSMacOS:
		return "https://git-scm.com/download/mac"
	case platform.OSWindows:
		return "https://git-scm.com/download/win"
	default:
		return "https://git-scm.com/download/linux"
	}
}
