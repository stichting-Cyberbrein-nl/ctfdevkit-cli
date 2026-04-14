// Package ports provides port conflict detection and forced port release.
package ports

import (
	"bufio"
	"context"
	"fmt"
	"net"
	"os/exec"
	"runtime"
	"strconv"
	"strings"
	"time"

	"github.com/stichting-Cyberbrein-nl/ctfdevkit-cli/internal/docker"
	"github.com/stichting-Cyberbrein-nl/ctfdevkit-cli/internal/output"
	"github.com/stichting-Cyberbrein-nl/ctfdevkit-cli/internal/platform"
)

// PortPair holds an HTTP and HTTPS port.
type PortPair struct {
	HTTP  int
	HTTPS int
}

// IsPrivileged returns true if the port requires root on Unix (< 1024).
func IsPrivileged(port int) bool {
	return port < 1024
}

// IsInUse returns true if a TCP listener is bound to the given port.
func IsInUse(port int) bool {
	return IsInUseOnHost("", port)
}

// IsInUseOnHost returns true if a TCP listener is bound to host:port.
func IsInUseOnHost(host string, port int) bool {
	ln, err := net.Listen("tcp", net.JoinHostPort(host, strconv.Itoa(port)))
	if err != nil {
		return true
	}
	ln.Close()
	return false
}

// FindFreePort tries the preferred port first; if unavailable (or unreleasable),
// it asks the OS for any free ephemeral port and returns that instead.
func FindFreePort(ctx context.Context, preferred int, plat platform.Platform) (int, error) {
	return FindFreePortForBindIP(ctx, preferred, "", plat)
}

// FindFreePortForBindIP tries the preferred port on a specific bind IP.
func FindFreePortForBindIP(ctx context.Context, preferred int, bindIP string, plat platform.Platform) (int, error) {
	if !IsInUseOnHost(bindIP, preferred) {
		return preferred, nil
	}

	// Try to release it (non-fatal if it fails).
	output.Infof("Port %d is in use - attempting to release...", preferred)
	_ = ForceRelease(ctx, preferred, plat)
	time.Sleep(400 * time.Millisecond)

	if !IsInUseOnHost(bindIP, preferred) {
		output.Successf("Port %d released", preferred)
		return preferred, nil
	}

	alternatives := alternativesFor(preferred)
	for _, p := range alternatives {
		if !IsInUseOnHost(bindIP, p) {
			output.Warnf("Port %d busy - using port %d instead", preferred, p)
			return p, nil
		}
	}

	ln, err := net.Listen("tcp", net.JoinHostPort(bindIP, "0"))
	if err != nil {
		return 0, fmt.Errorf("no free port available")
	}
	p := ln.Addr().(*net.TCPAddr).Port
	ln.Close()
	output.Warnf("Port %d busy - using random port %d", preferred, p)
	return p, nil
}

// alternativesFor returns sensible fallback ports for well-known port numbers.
func alternativesFor(port int) []int {
	switch port {
	case 80:
		return []int{8080, 8000, 8888, 3080, 4080}
	case 443:
		return []int{8443, 4443, 9443, 3443}
	default:
		// Try +1 to +20.
		var alts []int
		for i := 1; i <= 20; i++ {
			alts = append(alts, port+i)
		}
		return alts
	}
}

// ResolvePorts tries to get the preferred HTTP/HTTPS ports and falls back gracefully.
// Returns the actual PortPair that should be used (may differ from preferred).
func ResolvePorts(ctx context.Context, preferHTTP, preferHTTPS int, plat platform.Platform) (PortPair, error) {
	return ResolvePortsForBindIP(ctx, preferHTTP, preferHTTPS, "", plat)
}

// ResolvePortsForBindIP tries to get the preferred ports on a specific bind IP.
func ResolvePortsForBindIP(ctx context.Context, preferHTTP, preferHTTPS int, bindIP string, plat platform.Platform) (PortPair, error) {
	httpPort, err := FindFreePortForBindIP(ctx, preferHTTP, bindIP, plat)
	if err != nil {
		return PortPair{}, fmt.Errorf("cannot bind HTTP port: %w", err)
	}
	httpsPort, err := FindFreePortForBindIP(ctx, preferHTTPS, bindIP, plat)
	if err != nil {
		return PortPair{}, fmt.Errorf("cannot bind HTTPS port: %w", err)
	}
	return PortPair{HTTP: httpPort, HTTPS: httpsPort}, nil
}

// EnsureReserved is kept for compatibility — it now uses ResolvePorts internally.
// Callers that need the actual ports should use ResolvePorts directly.
func EnsureReserved(ctx context.Context, plat platform.Platform) error {
	_, err := ResolvePorts(ctx, 80, 443, plat)
	return err
}

// ForceRelease tries to free the given port using platform-specific methods.
func ForceRelease(ctx context.Context, port int, plat platform.Platform) error {
	// First, try stopping Docker containers that publish this port.
	if err := docker.StopContainersOnPort(ctx, port); err == nil {
		if !IsInUse(port) {
			return nil
		}
	}

	switch runtime.GOOS {
	case "windows":
		return forceReleaseWindows(ctx, port)
	default:
		return forceReleaseUnix(ctx, port)
	}
}

// forceReleaseUnix releases a port on Linux/macOS/WSL.
func forceReleaseUnix(ctx context.Context, port int) error {
	pids, err := findPIDsOnPortUnix(ctx, port)
	if err != nil || len(pids) == 0 {
		return nil
	}

	filtered := filterDockerPIDs(ctx, pids)
	if len(filtered) == 0 {
		return nil
	}

	// Check for Herd.app on macOS and stop it gracefully.
	if runtime.GOOS == "darwin" {
		if hasHerd(ctx, filtered) {
			output.Info("Stopping Laravel Herd to free port...")
			exec.CommandContext(ctx, "herd", "stop").Run()
			time.Sleep(1 * time.Second)
			if !IsInUse(port) {
				return nil
			}
		}
	}

	for _, pid := range filtered {
		// SIGTERM first.
		exec.CommandContext(ctx, "sudo", "kill", "-TERM", strconv.Itoa(pid)).Run()
	}
	time.Sleep(500 * time.Millisecond)

	for _, pid := range filtered {
		// SIGKILL if still running.
		exec.CommandContext(ctx, "sudo", "kill", "-KILL", strconv.Itoa(pid)).Run()
	}

	return nil
}

// forceReleaseWindows releases a port on native Windows.
func forceReleaseWindows(ctx context.Context, port int) error {
	pids, err := findPIDsOnPortWindows(ctx, port)
	if err != nil || len(pids) == 0 {
		return nil
	}

	hasWSLRelay := false
	for _, pid := range pids {
		if isProcessName(ctx, pid, "wslrelay.exe") {
			hasWSLRelay = true
		} else if isDockerProcess(ctx, pid) {
			continue
		}
		exec.CommandContext(ctx, "taskkill.exe", "/PID", strconv.Itoa(pid), "/F").Run()
	}
	if hasWSLRelay {
		stopUserWSLDistros(ctx)
	}
	return nil
}

// findPIDsOnPortUnix uses lsof (preferred) or ss/netstat to find PIDs.
func findPIDsOnPortUnix(ctx context.Context, port int) ([]int, error) {
	// Try lsof first.
	if path, err := exec.LookPath("lsof"); err == nil {
		out, err := exec.CommandContext(ctx, path, "-ti", fmt.Sprintf(":%d", port)).Output()
		if err == nil {
			return parsePIDs(string(out)), nil
		}
	}

	// Fallback: ss.
	if path, err := exec.LookPath("ss"); err == nil {
		out, err := exec.CommandContext(ctx, path, "-tlnp", fmt.Sprintf("sport = :%d", port)).Output()
		if err == nil {
			return parsePIDsFromSS(string(out)), nil
		}
	}

	return nil, fmt.Errorf("no tool available to inspect port %d (tried lsof, ss)", port)
}

// findPIDsOnPortWindows parses netstat.exe output to find PIDs.
func findPIDsOnPortWindows(ctx context.Context, port int) ([]int, error) {
	out, err := exec.CommandContext(ctx, "netstat.exe", "-ano").Output()
	if err != nil {
		return nil, err
	}

	var pids []int
	portStr := fmt.Sprintf(":%d", port)
	scanner := bufio.NewScanner(strings.NewReader(string(out)))
	for scanner.Scan() {
		line := scanner.Text()
		if !strings.Contains(line, portStr) {
			continue
		}
		fields := strings.Fields(line)
		if len(fields) < 5 {
			continue
		}
		if !strings.EqualFold(fields[3], "LISTENING") {
			continue
		}
		localAddr := fields[1]
		if !strings.HasSuffix(localAddr, portStr) {
			continue
		}
		pid, err := strconv.Atoi(fields[4])
		if err != nil {
			continue
		}
		pids = append(pids, pid)
	}
	return pids, nil
}

func stopUserWSLDistros(ctx context.Context) {
	output.Info("Stopping WSL distro(s) that forward this port...")
	script := "$names = (wsl.exe --list --running --quiet) -replace \"`0\", \"\" | Where-Object { $_ -and $_ -notlike \"docker-desktop*\" }; foreach ($name in $names) { wsl.exe --terminate $name }"
	_ = exec.CommandContext(ctx, "powershell", "-NoProfile", "-NonInteractive", "-Command", script).Run()
}

func isProcessName(ctx context.Context, pid int, want string) bool {
	out, err := exec.CommandContext(ctx, "tasklist.exe", "/FI", fmt.Sprintf("PID eq %d", pid), "/FO", "CSV", "/NH").Output()
	if err != nil {
		return false
	}
	return strings.Contains(strings.ToLower(string(out)), strings.ToLower(want))
}

// filterDockerPIDs removes Docker-related processes from the list.
func filterDockerPIDs(ctx context.Context, pids []int) []int {
	var filtered []int
	for _, pid := range pids {
		if isDockerProcess(ctx, pid) {
			continue
		}
		filtered = append(filtered, pid)
	}
	return filtered
}

func isDockerProcess(ctx context.Context, pid int) bool {
	var name string
	if runtime.GOOS == "windows" {
		out, err := exec.CommandContext(ctx, "tasklist.exe", "/FI",
			fmt.Sprintf("PID eq %d", pid), "/FO", "CSV", "/NH").Output()
		if err != nil {
			return false
		}
		name = strings.ToLower(string(out))
	} else {
		out, err := exec.CommandContext(ctx, "ps", "-p", strconv.Itoa(pid), "-o", "comm=").Output()
		if err != nil {
			return false
		}
		name = strings.ToLower(strings.TrimSpace(string(out)))
	}
	dockerNames := []string{"docker", "com.docke", "docker-proxy", "containerd"}
	for _, d := range dockerNames {
		if strings.Contains(name, d) {
			return true
		}
	}
	return false
}

func hasHerd(ctx context.Context, pids []int) bool {
	for _, pid := range pids {
		out, err := exec.CommandContext(ctx, "ps", "-p", strconv.Itoa(pid), "-o", "comm=").Output()
		if err != nil {
			continue
		}
		if strings.Contains(strings.ToLower(string(out)), "herd") {
			return true
		}
	}
	return false
}

func parsePIDs(output string) []int {
	var pids []int
	for _, line := range strings.Split(strings.TrimSpace(output), "\n") {
		line = strings.TrimSpace(line)
		if line == "" {
			continue
		}
		pid, err := strconv.Atoi(line)
		if err == nil {
			pids = append(pids, pid)
		}
	}
	return pids
}

func parsePIDsFromSS(ssOutput string) []int {
	// ss output example: tcp ... users:(("nginx",pid=1234,fd=6))
	var pids []int
	for _, line := range strings.Split(ssOutput, "\n") {
		if !strings.Contains(line, "pid=") {
			continue
		}
		for _, part := range strings.Split(line, ",") {
			if strings.HasPrefix(part, "pid=") {
				pidStr := strings.TrimPrefix(part, "pid=")
				pidStr = strings.TrimRight(pidStr, ")")
				pid, err := strconv.Atoi(pidStr)
				if err == nil {
					pids = append(pids, pid)
				}
			}
		}
	}
	return pids
}
