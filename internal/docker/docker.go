// Package docker provides Docker daemon detection and management utilities.
package docker

import (
	"context"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"runtime"
	"strings"
	"time"

	"github.com/stichting-Cyberbrein-nl/ctfdevkit-cli/internal/output"
)

// IsAvailable returns true if the docker binary is in PATH.
func IsAvailable() bool {
	_, err := exec.LookPath("docker")
	return err == nil
}

// HasComposePlugin returns true if `docker compose` is available.
func HasComposePlugin(ctx context.Context) bool {
	cmd := exec.CommandContext(ctx, "docker", "compose", "version")
	return cmd.Run() == nil
}

// IsRunning returns true if the Docker daemon is responding.
func IsRunning(ctx context.Context) bool {
	cmd := exec.CommandContext(ctx, "docker", "info")
	return cmd.Run() == nil
}

// WaitForDaemon polls the Docker daemon until it responds or the context expires.
func WaitForDaemon(ctx context.Context) error {
	output.Info("Waiting for Docker daemon...")
	ticker := time.NewTicker(2 * time.Second)
	defer ticker.Stop()

	for {
		select {
		case <-ctx.Done():
			return fmt.Errorf("timed out waiting for Docker daemon — is Docker Desktop running?")
		case <-ticker.C:
			if IsRunning(ctx) {
				return nil
			}
		}
	}
}

// TryLaunch attempts to start Docker on platforms that support auto-launch.
func TryLaunch(ctx context.Context) error {
	switch runtime.GOOS {
	case "darwin":
		output.Info("Starting Docker Desktop...")
		cmd := exec.CommandContext(ctx, "open", "-a", "Docker")
		return cmd.Run()
	case "windows":
		output.Info("Starting Docker Desktop...")
		// Try the registered Start Menu shortcut first.
		if err := exec.CommandContext(ctx, "cmd.exe", "/c", "start", "", "Docker Desktop").Run(); err == nil {
			return nil
		}
		// Fallback: launch the executable directly.
		progFiles := os.Getenv("ProgramFiles")
		if progFiles == "" {
			progFiles = `C:\Program Files`
		}
		exe := filepath.Join(progFiles, "Docker", "Docker", "Docker Desktop.exe")
		return exec.CommandContext(ctx, exe).Start()
	default:
		return fmt.Errorf("cannot auto-start Docker on this platform — please start it manually")
	}
}

// DiskPercentUsed returns approximate disk usage percentage for the root filesystem.
func DiskPercentUsed(ctx context.Context) (int, error) {
	if runtime.GOOS == "windows" {
		return diskPercentWindows(ctx)
	}
	out, err := exec.CommandContext(ctx, "df", "/").Output()
	if err != nil {
		return 0, err
	}
	return parseDiskPercent(string(out)), nil
}

// diskPercentWindows uses PowerShell to get disk usage for the system drive.
func diskPercentWindows(ctx context.Context) (int, error) {
	drive := os.Getenv("SystemDrive")
	if drive == "" {
		drive = "C:"
	}
	// Output is two lines: Used bytes, Free bytes.
	script := fmt.Sprintf(
		`$d = Get-PSDrive %s; $d.Used; $d.Free`,
		strings.TrimSuffix(drive, ":"),
	)
	out, err := exec.CommandContext(ctx, "powershell", "-NoProfile", "-NonInteractive", "-Command", script).Output()
	if err != nil {
		return 0, err
	}
	lines := strings.Fields(strings.TrimSpace(string(out)))
	if len(lines) < 2 {
		return 0, fmt.Errorf("unexpected powershell output: %q", string(out))
	}
	var used, free float64
	fmt.Sscanf(lines[0], "%f", &used)
	fmt.Sscanf(lines[1], "%f", &free)
	total := used + free
	if total == 0 {
		return 0, nil
	}
	return int(used / total * 100), nil
}

// MaybeFreeSpace prunes Docker if disk usage is critically high (>95%).
func MaybeFreeSpace(ctx context.Context) error {
	pct, err := DiskPercentUsed(ctx)
	if err != nil {
		// Non-fatal: just skip the check.
		return nil
	}

	if pct < 95 {
		return nil
	}

	output.Warnf("Disk is %d%% full — running Docker cleanup to free space...", pct)
	if err := SystemPrune(ctx); err != nil {
		return fmt.Errorf("docker cleanup failed: %w", err)
	}
	return nil
}

// SystemPrune removes unused Docker data aggressively.
func SystemPrune(ctx context.Context) error {
	cmds := [][]string{
		{"docker", "system", "prune", "-af"},
		{"docker", "builder", "prune", "-af"},
	}
	for _, args := range cmds {
		cmd := exec.CommandContext(ctx, args[0], args[1:]...)
		cmd.Stdout = nil
		cmd.Stderr = nil
		if err := cmd.Run(); err != nil {
			return err
		}
	}
	return nil
}

// StopContainersOnPort stops Docker containers that publish the given port.
func StopContainersOnPort(ctx context.Context, port int) error {
	cmd := exec.CommandContext(ctx, "docker", "ps", "--format", "{{.ID}}", "--filter",
		fmt.Sprintf("publish=%d", port))
	out, err := cmd.Output()
	if err != nil {
		return nil // no containers or docker not running
	}

	ids := strings.Fields(strings.TrimSpace(string(out)))
	if len(ids) == 0 {
		return nil
	}

	output.Infof("Stopping %d container(s) using port %d...", len(ids), port)
	stopArgs := append([]string{"stop"}, ids...)
	stop := exec.CommandContext(ctx, "docker", stopArgs...)
	return stop.Run()
}

func parseDiskPercent(dfOutput string) int {
	lines := strings.Split(dfOutput, "\n")
	for _, line := range lines[1:] {
		fields := strings.Fields(line)
		if len(fields) < 5 {
			continue
		}
		pctStr := strings.TrimSuffix(fields[4], "%")
		var pct int
		fmt.Sscanf(pctStr, "%d", &pct)
		return pct
	}
	return 0
}
