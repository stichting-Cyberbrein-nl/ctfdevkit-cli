package platform

import (
	"os"
	"path/filepath"
	"runtime"
)

// ConfigDir returns the OS-appropriate config directory for devkit.
// Linux/macOS: ~/.config/devkit/
// Windows:     %APPDATA%\devkit\
func ConfigDir() (string, error) {
	base, err := os.UserConfigDir()
	if err != nil {
		return "", err
	}
	dir := filepath.Join(base, "devkit")
	return dir, os.MkdirAll(dir, 0755)
}

// DataDir returns the OS-appropriate data directory for devkit.
// Linux/macOS: ~/.local/share/devkit/
// Windows:     %LOCALAPPDATA%\devkit\
func DataDir() (string, error) {
	var base string
	if runtime.GOOS == "windows" {
		base = os.Getenv("LOCALAPPDATA")
		if base == "" {
			home, err := os.UserHomeDir()
			if err != nil {
				return "", err
			}
			base = filepath.Join(home, "AppData", "Local")
		}
	} else {
		home, err := os.UserHomeDir()
		if err != nil {
			return "", err
		}
		base = filepath.Join(home, ".local", "share")
	}
	dir := filepath.Join(base, "devkit")
	return dir, os.MkdirAll(dir, 0755)
}

// PayloadDir returns the directory where the config bundle is installed.
func PayloadDir() (string, error) {
	data, err := DataDir()
	if err != nil {
		return "", err
	}
	dir := filepath.Join(data, "payload")
	return dir, os.MkdirAll(dir, 0755)
}

// CertsDir returns the directory where TLS certificates are stored.
// This lives inside the payload directory so Caddy can mount it.
func CertsDir() (string, error) {
	payload, err := PayloadDir()
	if err != nil {
		return "", err
	}
	dir := filepath.Join(payload, "certs")
	return dir, os.MkdirAll(dir, 0755)
}

// TempDir returns a devkit-specific temp directory.
func TempDir() (string, error) {
	dir := filepath.Join(os.TempDir(), "devkit")
	return dir, os.MkdirAll(dir, 0755)
}
