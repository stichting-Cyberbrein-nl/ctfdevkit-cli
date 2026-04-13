// Package hosts manages /etc/hosts bindings for devkit domains.
package hosts

import (
	"bufio"
	"encoding/base64"
	"fmt"
	"os"
	"os/exec"
	"strings"

	"github.com/stichting-Cyberbrein-nl/ctfdevkit-cli/internal/output"
	"github.com/stichting-Cyberbrein-nl/ctfdevkit-cli/internal/platform"
)

// EnsureBinding idempotently adds "<ip> <domain>" to the hosts file.
func EnsureBinding(ip, domain string, plat platform.Platform) error {
	switch plat.OS {
	case platform.OSWindows:
		return ensureWindowsNativeHosts(ip, domain)
	case platform.OSWSL:
		// Bind on Linux side.
		if err := ensureUnixHosts(ip, domain, plat.UnixHostsPath()); err != nil {
			return err
		}
		// Also bind on the Windows hosts file if accessible.
		winHosts := plat.WindowsHostsPath()
		if _, err := os.Stat(winHosts); err == nil {
			if err := ensureWindowsHostsFromWSL(ip, domain, winHosts); err != nil {
				output.Warnf("Could not update Windows hosts file: %v", err)
				output.Hint("You may need to run this command in an elevated WSL session.")
			}
		}
		return nil
	default:
		return ensureUnixHosts(ip, domain, plat.UnixHostsPath())
	}
}

// RemoveBinding removes the "<ip> <domain>" entry from the hosts file.
func RemoveBinding(domain string, plat platform.Platform) error {
	if plat.OS == platform.OSWindows {
		return removeFromWindowsNativeHosts(domain)
	}
	return removeFromHostsFile(domain, plat.UnixHostsPath())
}

// ensureWindowsNativeHosts updates the Windows hosts file on native Windows.
// It tries a direct write first (succeeds when running as Administrator),
// then falls back to a UAC-elevated PowerShell session.
func ensureWindowsNativeHosts(ip, domain string) error {
	hostsPath := `C:\Windows\System32\drivers\etc\hosts`

	content, err := os.ReadFile(hostsPath)
	if err != nil {
		output.Warn("Cannot read Windows hosts file.")
		printWindowsHostsHint(ip, domain, hostsPath)
		return nil
	}

	if containsEntry(string(content), ip, domain) {
		output.Successf("Hosts entry already present: %s %s", ip, domain)
		return nil
	}

	entry := ip + " " + domain
	newContent := strings.TrimRight(string(content), "\r\n") + "\r\n" + entry + "\r\n"

	// Try direct write first (works when running as Administrator).
	if err := os.WriteFile(hostsPath, []byte(newContent), 0644); err == nil {
		output.Successf("Windows hosts updated: %s", entry)
		return nil
	}

	// Not admin — attempt UAC elevation via PowerShell.
	output.Info("Requesting Administrator privileges to update hosts file (UAC prompt may appear)...")
	if err := elevateWindowsHostsWrite(hostsPath, newContent); err == nil {
		output.Successf("Windows hosts updated: %s", entry)
		return nil
	}

	output.Warn("Could not automatically update Windows hosts file.")
	printWindowsHostsHint(ip, domain, hostsPath)
	return nil
}

// removeFromWindowsNativeHosts removes a domain entry from the Windows hosts file.
func removeFromWindowsNativeHosts(domain string) error {
	hostsPath := `C:\Windows\System32\drivers\etc\hosts`

	content, err := os.ReadFile(hostsPath)
	if err != nil {
		return nil // can't read — skip silently
	}

	var lines []string
	scanner := bufio.NewScanner(strings.NewReader(string(content)))
	for scanner.Scan() {
		line := scanner.Text()
		fields := strings.Fields(line)
		if len(fields) >= 2 && fields[1] == domain {
			continue
		}
		lines = append(lines, line)
	}

	newContent := strings.Join(lines, "\r\n") + "\r\n"

	if err := os.WriteFile(hostsPath, []byte(newContent), 0644); err == nil {
		return nil
	}

	return elevateWindowsHostsWrite(hostsPath, newContent)
}

// elevateWindowsHostsWrite writes content to the Windows hosts file via a
// UAC-elevated PowerShell session. It writes the content to a temp file first,
// then copies it to the protected path in the elevated session.
func elevateWindowsHostsWrite(hostsPath, content string) error {
	tmp, err := os.CreateTemp("", "devkit-hosts-*.txt")
	if err != nil {
		return err
	}
	tmpPath := tmp.Name()
	defer os.Remove(tmpPath)

	if _, err := tmp.WriteString(content); err != nil {
		tmp.Close()
		return err
	}
	tmp.Close()

	// Build the inner PowerShell command (single-quote escape by doubling).
	innerCmd := fmt.Sprintf(
		`Copy-Item -Path '%s' -Destination '%s' -Force`,
		strings.ReplaceAll(tmpPath, `'`, `''`),
		strings.ReplaceAll(hostsPath, `'`, `''`),
	)

	// Encode as UTF-16LE Base64 for -EncodedCommand (avoids all quoting issues).
	encoded := encodePS(innerCmd)

	cmd := exec.Command(
		"powershell", "-NoProfile", "-NonInteractive", "-Command",
		fmt.Sprintf(
			`Start-Process powershell -Verb RunAs -Wait -ArgumentList '-NoProfile','-NonInteractive','-EncodedCommand','%s'`,
			encoded,
		),
	)
	return cmd.Run()
}

// encodePS encodes a PowerShell command string as UTF-16LE Base64
// for use with the -EncodedCommand parameter.
func encodePS(s string) string {
	runes := []rune(s)
	b := make([]byte, len(runes)*2)
	for i, r := range runes {
		b[i*2] = byte(r)
		b[i*2+1] = byte(r >> 8)
	}
	return base64.StdEncoding.EncodeToString(b)
}

// printWindowsHostsHint prints a manual fallback instruction for Windows.
func printWindowsHostsHint(ip, domain, hostsPath string) {
	output.Plain("Run the following in an elevated PowerShell (Run as Administrator):")
	output.Plainf("  Add-Content -Path \"%s\" -Value \"`n%s %s\"", hostsPath, ip, domain)
}

func ensureUnixHosts(ip, domain, hostsPath string) error {
	content, err := os.ReadFile(hostsPath)
	if err != nil {
		return fmt.Errorf("reading %s: %w", hostsPath, err)
	}

	entry := fmt.Sprintf("%s %s", ip, domain)
	if containsEntry(string(content), ip, domain) {
		output.Successf("Hosts entry already present: %s", entry)
		return nil
	}

	newContent := strings.TrimRight(string(content), "\n") + "\n" + entry + "\n"
	return writeHostsFile(hostsPath, newContent)
}

func ensureWindowsHostsFromWSL(ip, domain, hostsPath string) error {
	content, err := os.ReadFile(hostsPath)
	if err != nil {
		return fmt.Errorf("reading Windows hosts: %w", err)
	}

	if containsEntry(string(content), ip, domain) {
		return nil
	}

	entry := fmt.Sprintf("%s %s", ip, domain)
	newContent := strings.TrimRight(string(content), "\n\r") + "\r\n" + entry + "\r\n"

	// Write via temp file and cp with elevated permissions.
	tmp, err := os.CreateTemp("", "devkit-hosts-*.txt")
	if err != nil {
		return err
	}
	defer os.Remove(tmp.Name())

	if _, err := tmp.WriteString(newContent); err != nil {
		return err
	}
	tmp.Close()

	// On WSL we can use cp directly since we may have access.
	cmd := exec.Command("cp", tmp.Name(), hostsPath)
	if out, err := cmd.CombinedOutput(); err != nil {
		return fmt.Errorf("cp to Windows hosts failed: %s: %w", string(out), err)
	}
	output.Successf("Windows hosts updated: %s %s", ip, domain)
	return nil
}

func writeHostsFile(hostsPath, content string) error {
	// Write to temp file, then move with sudo on protected paths.
	tmp, err := os.CreateTemp(os.TempDir(), "devkit-hosts-*.txt")
	if err != nil {
		return err
	}
	defer os.Remove(tmp.Name())

	if _, err := tmp.WriteString(content); err != nil {
		return err
	}
	tmp.Close()

	// Try direct write first (useful in containers or if already root).
	if err := os.WriteFile(hostsPath, []byte(content), 0644); err == nil {
		output.Successf("Hosts file updated: %s", hostsPath)
		return nil
	}

	// Fall back to sudo.
	output.Info("Updating hosts file requires elevated privileges...")
	cmd := exec.Command("sudo", "cp", tmp.Name(), hostsPath)
	cmd.Stdin = os.Stdin
	cmd.Stdout = os.Stdout
	cmd.Stderr = os.Stderr
	if err := cmd.Run(); err != nil {
		return fmt.Errorf("sudo cp to %s failed: %w", hostsPath, err)
	}
	output.Successf("Hosts file updated: %s", hostsPath)
	return nil
}

func removeFromHostsFile(domain, hostsPath string) error {
	content, err := os.ReadFile(hostsPath)
	if err != nil {
		return err
	}

	var lines []string
	scanner := bufio.NewScanner(strings.NewReader(string(content)))
	for scanner.Scan() {
		line := scanner.Text()
		fields := strings.Fields(line)
		if len(fields) >= 2 && fields[1] == domain {
			continue // Skip this entry.
		}
		lines = append(lines, line)
	}

	newContent := strings.Join(lines, "\n") + "\n"
	return writeHostsFile(hostsPath, newContent)
}

func containsEntry(content, ip, domain string) bool {
	scanner := bufio.NewScanner(strings.NewReader(content))
	for scanner.Scan() {
		line := strings.TrimSpace(scanner.Text())
		if strings.HasPrefix(line, "#") {
			continue
		}
		fields := strings.Fields(line)
		if len(fields) >= 2 && fields[0] == ip && fields[1] == domain {
			return true
		}
	}
	return false
}
