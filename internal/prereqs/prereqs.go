// Package prereqs checks and installs prerequisite tooling.
package prereqs

import (
	"context"
	"fmt"
	"os"
	"os/exec"
	"runtime"
	"slices"
	"strings"
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
	if addCurrentUserToDockerGroup(ctx) {
		return fmt.Errorf("Docker is running, but %s cannot access /var/run/docker.sock yet - run `newgrp docker` or open a new terminal, then run devkit again", user)
	}

	return fmt.Errorf("Docker is running, but the current user cannot access /var/run/docker.sock - add the user to the docker group or run Docker with proper permissions")
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
		return fmt.Errorf("mkcert still not found after installation - please restart your terminal")
	}

	output.Success("mkcert installed")
	return nil
}

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

// installDockerLinux detects the distro and uses the appropriate package manager.
func installDockerLinux(ctx context.Context) error {
	distro := detectLinuxDistroInfo()
	output.Infof("Detected Linux distro: %s", distro.ID)

	switch {
	case distro.ID == "arch":
		return installDockerArch(ctx)
	case isDebianOrUbuntuBased(distro):
		return installDockerDebian(ctx, distro)
	case distro.ID == "fedora":
		return installDockerFedora(ctx)
	case slices.Contains([]string{"rhel", "centos", "rocky", "almalinux"}, distro.ID):
		return installDockerRHEL(ctx)
	case slices.Contains([]string{"opensuse", "sles"}, distro.ID):
		return installDockerOpenSUSE(ctx)
	default:
		// Unknown distro - try the official convenience script.
		return installDockerScript(ctx)
	}
}

type linuxDistro struct {
	ID              string
	IDLike          []string
	VersionCodename string
	UbuntuCodename  string
}

// detectLinuxDistro reads /etc/os-release to identify the distribution.
func detectLinuxDistro() string {
	return detectLinuxDistroInfo().ID
}

func detectLinuxDistroInfo() linuxDistro {
	if runtime.GOOS != "linux" {
		return linuxDistro{ID: "unknown"}
	}

	// Arch Linux has its own release file.
	if _, err := os.Stat("/etc/arch-release"); err == nil {
		return linuxDistro{ID: "arch"}
	}

	data, err := os.ReadFile("/etc/os-release")
	if err != nil {
		// Fallback: check package managers.
		return linuxDistro{ID: detectDistroByPackageManager()}
	}

	distro := parseLinuxOSRelease(string(data))
	if distro.ID == "" {
		distro.ID = detectDistroByPackageManager()
	}
	return distro
}

func parseLinuxOSRelease(data string) linuxDistro {
	values := map[string]string{}
	for _, line := range strings.Split(data, "\n") {
		line = strings.TrimSpace(line)
		if line == "" || strings.HasPrefix(line, "#") {
			continue
		}

		key, value, ok := strings.Cut(line, "=")
		if !ok {
			continue
		}
		values[strings.ToUpper(strings.TrimSpace(key))] = strings.ToLower(strings.Trim(strings.TrimSpace(value), `"'`))
	}

	return linuxDistro{
		ID:              values["ID"],
		IDLike:          strings.Fields(values["ID_LIKE"]),
		VersionCodename: values["VERSION_CODENAME"],
		UbuntuCodename:  values["UBUNTU_CODENAME"],
	}
}

func isDebianOrUbuntuBased(distro linuxDistro) bool {
	if slices.Contains([]string{"debian", "ubuntu", "kali", "raspbian", "linuxmint", "pop"}, distro.ID) {
		return true
	}
	return slices.Contains(distro.IDLike, "debian") || slices.Contains(distro.IDLike, "ubuntu")
}

type dockerAPTRepo struct {
	OS       string
	Codename string
}

func resolveDockerAPTRepo(distro linuxDistro) (dockerAPTRepo, error) {
	switch distro.ID {
	case "debian":
		return dockerAPTRepo{OS: "debian", Codename: distro.VersionCodename}, nil
	case "ubuntu":
		return dockerAPTRepo{OS: "ubuntu", Codename: firstNonEmpty(distro.UbuntuCodename, distro.VersionCodename)}, nil
	case "kali":
		// Kali is rolling but Docker publishes Debian suites; Kali docs currently recommend Debian stable.
		return dockerAPTRepo{OS: "debian", Codename: "trixie"}, nil
	case "raspbian":
		return dockerAPTRepo{OS: "raspbian", Codename: distro.VersionCodename}, nil
	case "linuxmint":
		return dockerAPTRepo{OS: "ubuntu", Codename: firstNonEmpty(distro.UbuntuCodename, distro.VersionCodename)}, nil
	case "pop":
		return dockerAPTRepo{OS: "ubuntu", Codename: firstNonEmpty(distro.UbuntuCodename, distro.VersionCodename)}, nil
	}

	switch {
	case slices.Contains(distro.IDLike, "ubuntu"):
		return dockerAPTRepo{OS: "ubuntu", Codename: firstNonEmpty(distro.UbuntuCodename, distro.VersionCodename)}, nil
	case slices.Contains(distro.IDLike, "debian"):
		return dockerAPTRepo{OS: "debian", Codename: distro.VersionCodename}, nil
	default:
		return dockerAPTRepo{}, fmt.Errorf("%s is not Debian or Ubuntu based", distro.ID)
	}
}

func firstNonEmpty(values ...string) string {
	for _, value := range values {
		if value != "" {
			return value
		}
	}
	return ""
}

func validateDockerAPTRepo(repo dockerAPTRepo) error {
	if repo.OS == "" || repo.Codename == "" {
		return fmt.Errorf("could not determine Docker apt repository for this distro")
	}

	switch repo.OS {
	case "debian":
		if slices.Contains([]string{"bullseye", "bookworm", "trixie"}, repo.Codename) {
			return nil
		}
	case "ubuntu":
		if slices.Contains([]string{"jammy", "noble", "questing", "resolute"}, repo.Codename) {
			return nil
		}
	case "raspbian":
		if slices.Contains([]string{"bullseye", "bookworm"}, repo.Codename) {
			return nil
		}
	}
	return fmt.Errorf("Docker does not publish an apt repository for %s %s", repo.OS, repo.Codename)
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

// installDockerDebian installs Docker on Debian / Ubuntu based distributions via apt-get.
func installDockerDebian(ctx context.Context, distro linuxDistro) error {
	output.Info("Installing Docker via apt-get...")

	repo, err := resolveDockerAPTRepo(distro)
	if err != nil {
		return err
	}

	repoSupported := validateDockerAPTRepo(repo) == nil
	if err := removeStaleDockerAPTSource(ctx, repo, repoSupported); err != nil {
		return err
	}

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

	if err := validateDockerAPTRepo(repo); err != nil {
		output.Warnf("%v; using distro packages instead.", err)
		if distroErr := installDockerDebianPackages(ctx); distroErr == nil {
			return nil
		}
		output.Warn("Distro package install failed, trying convenience script...")
		return installDockerScript(ctx)
	}
	output.Infof("Using Docker apt repository: %s %s", repo.OS, repo.Codename)

	script := fmt.Sprintf(`
set -e
install -m 0755 -d /etc/apt/keyrings
curl -fsSL https://download.docker.com/linux/%s/gpg \
    | gpg --dearmor --yes -o /etc/apt/keyrings/docker.gpg
chmod a+r /etc/apt/keyrings/docker.gpg
echo "deb [arch=$(dpkg --print-architecture) signed-by=/etc/apt/keyrings/docker.gpg] \
    https://download.docker.com/linux/%s \
    %s stable" \
    | tee /etc/apt/sources.list.d/docker.list > /dev/null
apt-get update -qq
apt-get install -y -qq docker-ce docker-ce-cli containerd.io docker-buildx-plugin docker-compose-plugin
`, repo.OS, repo.OS, repo.Codename)
	cmd := exec.CommandContext(ctx, "sudo", "sh", "-c", script)
	cmd.Stdout = os.Stdout
	cmd.Stderr = os.Stderr
	if err := cmd.Run(); err != nil {
		output.Warn("Official repo install failed, trying distro packages...")
		if distroErr := installDockerDebianPackages(ctx); distroErr == nil {
			return nil
		}
		output.Warn("Distro package install failed, trying convenience script...")
		return installDockerScript(ctx)
	}

	return enableAndStartDocker(ctx)
}

func removeStaleDockerAPTSource(ctx context.Context, repo dockerAPTRepo, repoSupported bool) error {
	supported := "0"
	if repoSupported {
		supported = "1"
	}

	script := `
set -e
expected_uri="$1"
expected_suite="$2"
supported="$3"

for file in /etc/apt/sources.list.d/docker.list /etc/apt/sources.list.d/docker.sources; do
    [ -f "$file" ] || continue
    grep -q 'download.docker.com/linux' "$file" || continue

    if [ "$supported" != "1" ]; then
        rm -f "$file"
        continue
    fi

    if grep -q "$expected_uri" "$file" && grep -q "$expected_suite" "$file"; then
        continue
    fi

    rm -f "$file"
done
`
	cmd := exec.CommandContext(ctx, "sudo", "sh", "-c", script, "sh",
		"https://download.docker.com/linux/"+repo.OS,
		repo.Codename,
		supported,
	)
	cmd.Stdout = os.Stdout
	cmd.Stderr = os.Stderr
	if err := cmd.Run(); err != nil {
		return fmt.Errorf("cleaning stale Docker apt source failed: %w", err)
	}
	return nil
}

func installDockerDebianPackages(ctx context.Context) error {
	steps := [][]string{
		{"sudo", "apt-get", "update", "-qq"},
		{"sudo", "apt-get", "install", "-y", "-qq", "docker.io", "docker-compose-plugin"},
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
		return fmt.Errorf("curl is required for auto-install on this distro - install Docker manually: https://docs.docker.com/get-docker/")
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
			output.Hint("Added to docker group - log out and back in (or run: newgrp docker) for this to take effect.")
			return true
		}
	}
	return false
}
