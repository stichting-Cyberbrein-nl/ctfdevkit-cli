// Package certs manages mkcert installation and local TLS certificate generation.
package certs

import (
	"crypto/x509"
	"encoding/pem"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"runtime"
	"strings"
	"time"

	"github.com/stichting-Cyberbrein-nl/ctfdevkit-cli/internal/output"
	"github.com/stichting-Cyberbrein-nl/ctfdevkit-cli/internal/platform"
)

// IsMkcertInstalled returns true if the mkcert binary is in PATH.
func IsMkcertInstalled() bool {
	_, err := exec.LookPath("mkcert")
	return err == nil
}

// InstallMkcert installs mkcert using the platform package manager.
func InstallMkcert(plat platform.Platform) error {
	output.Info("Installing mkcert...")
	switch plat.OS {
	case platform.OSMacOS:
		return exec.Command("brew", "install", "mkcert").Run()
	case platform.OSWindows:
		if _, err := exec.LookPath("winget.exe"); err != nil {
			return fmt.Errorf("winget is not available — install mkcert manually from https://github.com/FiloSottile/mkcert/releases or via: choco install mkcert")
		}
		cmd := exec.Command("winget.exe", "install", "--id", "FiloSottile.mkcert", "-e", "--silent")
		cmd.Stdout = os.Stdout
		cmd.Stderr = os.Stderr
		return cmd.Run()
	case platform.OSWSL:
		return installMkcertLinux()
	default:
		return installMkcertLinux()
	}
}

func installMkcertLinux() error {
	// Try package managers first.
	if _, err := exec.LookPath("apt-get"); err == nil {
		cmd := exec.Command("sudo", "apt-get", "install", "-y", "mkcert", "libnss3-tools")
		cmd.Stdout = os.Stdout
		cmd.Stderr = os.Stderr
		return cmd.Run()
	}
	if _, err := exec.LookPath("brew"); err == nil {
		return exec.Command("brew", "install", "mkcert").Run()
	}
	return fmt.Errorf("could not auto-install mkcert — please install it manually: https://github.com/FiloSottile/mkcert")
}

// EnsureTrustStoreTools installs helper tools needed by mkcert to trust the CA
// in browser-specific stores such as Firefox NSS profiles.
func EnsureTrustStoreTools(plat platform.Platform) error {
	if plat.OS != platform.OSLinux && plat.OS != platform.OSWSL {
		return nil
	}
	if _, err := exec.LookPath("certutil"); err == nil {
		return nil
	}
	if _, err := exec.LookPath("apt-get"); err == nil {
		output.Info("Installing Firefox certificate trust helper (libnss3-tools)...")
		cmd := exec.Command("sudo", "apt-get", "install", "-y", "libnss3-tools")
		cmd.Stdout = os.Stdout
		cmd.Stderr = os.Stderr
		return cmd.Run()
	}

	output.Hint("Firefox may reject local HTTPS until NSS certutil is installed.")
	output.Hint("Debian/Ubuntu/Kali: sudo apt-get install -y libnss3-tools")
	return nil
}

// HasValidCert checks if a PEM cert file exists and is valid for the given domain
// (not expired, not within 7 days of expiry).
func HasValidCert(certFile, domain string) bool {
	data, err := os.ReadFile(certFile)
	if err != nil {
		return false
	}

	block, _ := pem.Decode(data)
	if block == nil {
		return false
	}

	cert, err := x509.ParseCertificate(block.Bytes)
	if err != nil {
		return false
	}

	// Check expiry (with 7-day buffer).
	if time.Until(cert.NotAfter) < 7*24*time.Hour {
		return false
	}

	// Check domain coverage.
	if err := cert.VerifyHostname(domain); err != nil {
		return false
	}

	return true
}

// Generate creates a trusted local TLS certificate for the given domain.
// If force is true it regenerates even if a valid cert already exists.
func Generate(certDir, domain, bindIP string, force bool, plat platform.Platform) error {
	certFile := filepath.Join(certDir, domain+".pem")
	keyFile := filepath.Join(certDir, domain+"-key.pem")

	if err := os.MkdirAll(certDir, 0755); err != nil {
		return fmt.Errorf("creating cert dir: %w", err)
	}

	if err := EnsureTrustStoreTools(plat); err != nil {
		return fmt.Errorf("installing certificate trust helpers: %w", err)
	}

	// Install the mkcert CA into the system trust store.
	output.Info("Installing mkcert CA into local trust store...")
	installCmd := exec.Command("mkcert", "-install")
	installCmd.Stdout = os.Stdout
	installCmd.Stderr = os.Stderr
	if err := installCmd.Run(); err != nil {
		output.Warnf("mkcert -install failed: %v", err)
		if runtime.GOOS == "windows" {
			output.Hint("On Windows, mkcert -install requires Administrator privileges.")
			output.Hint("Re-run this command in an elevated terminal (Run as Administrator) to trust the certificate.")
		}
	}

	if plat.OS == platform.OSWindows || plat.IsWSL {
		if err := SyncWindowsTrust(); err != nil {
			output.Warnf("Could not sync CA to Windows trust store: %v", err)
			output.Hint("Your browser on Windows may show a certificate warning.")
		} else if plat.OS == platform.OSWindows {
			output.Hint("Windows browsers laden nieuwe certificaten pas na een volledige herstart.")
			output.Hint("Sluit Brave/Chrome/Edge volledig af en open daarna https://ctf.dev opnieuw.")
		}
	}

	if !force && HasValidCert(certFile, domain) {
		output.Successf("TLS certificate already valid for %s", domain)
		return nil
	}

	// Build the list of SANs.
	hosts := []string{domain, "localhost", "127.0.0.1"}
	if bindIP != "" && bindIP != "127.0.0.1" {
		hosts = append(hosts, bindIP)
	}

	args := []string{
		"-cert-file", certFile,
		"-key-file", keyFile,
	}
	args = append(args, hosts...)

	output.Infof("Generating TLS certificate for: %s", strings.Join(hosts, ", "))
	genCmd := exec.Command("mkcert", args...)
	genCmd.Stdout = os.Stdout
	genCmd.Stderr = os.Stderr
	if err := genCmd.Run(); err != nil {
		return fmt.Errorf("mkcert cert generation failed: %w", err)
	}

	output.Successf("TLS certificate generated: %s", certFile)
	return nil
}

// SyncWindowsTrust imports the mkcert CA into the Windows certificate store
// from within a WSL session.
func SyncWindowsTrust() error {
	caRoot, err := getMkcertCARoot()
	if err != nil {
		return err
	}

	caFile := filepath.Join(caRoot, "rootCA.pem")
	if _, err := os.Stat(caFile); err != nil {
		return fmt.Errorf("mkcert CA file not found at %s", caFile)
	}

	// Convert to Windows path if running in WSL.
	winPath, err := wslToWindowsPath(caFile)
	if err != nil {
		winPath = caFile // fallback: try as-is
	}

	certutil := findCertutil()
	if certutil == "" {
		return fmt.Errorf("certutil.exe not found — cannot sync Windows trust store")
	}

	output.Info("Syncing CA certificate to Windows trust store...")
	cmd := exec.Command(certutil, "-f", "-addstore", "Root", winPath)
	cmd.Stdout = os.Stdout
	cmd.Stderr = os.Stderr
	return cmd.Run()
}

func getMkcertCARoot() (string, error) {
	out, err := exec.Command("mkcert", "-CAROOT").Output()
	if err != nil {
		return "", fmt.Errorf("mkcert -CAROOT failed: %w", err)
	}
	return strings.TrimSpace(string(out)), nil
}

func wslToWindowsPath(unixPath string) (string, error) {
	out, err := exec.Command("wslpath", "-w", unixPath).Output()
	if err != nil {
		return "", err
	}
	return strings.TrimSpace(string(out)), nil
}

func findCertutil() string {
	if runtime.GOOS == "windows" {
		return "certutil.exe"
	}
	// WSL: look for Windows certutil.
	candidates := []string{
		"/mnt/c/Windows/System32/certutil.exe",
		"/mnt/c/Windows/SysWOW64/certutil.exe",
	}
	for _, c := range candidates {
		if _, err := os.Stat(c); err == nil {
			return c
		}
	}
	if path, err := exec.LookPath("certutil.exe"); err == nil {
		return path
	}
	return ""
}
