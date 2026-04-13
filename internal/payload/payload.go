// Package payload manages the devkit config bundle (docker-compose, Caddyfile, .env).
// The bundle files are embedded in the CLI binary and extracted to the payload directory.
package payload

import (
	"bufio"
	"bytes"
	"context"
	"embed"
	"fmt"
	"io/fs"
	"os"
	"path/filepath"
	"strings"
	"time"

	"github.com/stichting-Cyberbrein-nl/ctfdevkit-cli/internal/docker"
	"github.com/stichting-Cyberbrein-nl/ctfdevkit-cli/internal/output"
	"github.com/stichting-Cyberbrein-nl/ctfdevkit-cli/internal/platform"
	"github.com/stichting-Cyberbrein-nl/ctfdevkit-cli/internal/releases"
	"github.com/stichting-Cyberbrein-nl/ctfdevkit-cli/internal/state"
)

//go:embed bundle/*
var bundleFS embed.FS

// Extract writes the embedded bundle files to payloadDir, replacing VERSION with the actual version.
func Extract(payloadDir, version string) error {
	if err := os.MkdirAll(payloadDir, 0755); err != nil {
		return fmt.Errorf("creating payload dir: %w", err)
	}

	return fs.WalkDir(bundleFS, "bundle", func(path string, d fs.DirEntry, err error) error {
		if err != nil {
			return err
		}
		if d.IsDir() {
			return nil
		}

		data, err := bundleFS.ReadFile(path)
		if err != nil {
			return err
		}

		// Replace {{VERSION}} placeholder with the real version.
		data = bytes.ReplaceAll(data, []byte("{{VERSION}}"), []byte(version))

		// Strip the "bundle/" prefix for the destination.
		rel := strings.TrimPrefix(path, "bundle/")
		dest := filepath.Join(payloadDir, rel)

		if err := os.MkdirAll(filepath.Dir(dest), 0755); err != nil {
			return err
		}
		return os.WriteFile(dest, data, 0644)
	})
}

// IsInstalled returns true if the payload directory contains a valid compose file.
func IsInstalled(s state.State) bool {
	return s.IsPayloadInstalled()
}

// EnsureInstalled installs the payload bundle. Bundle config files (Caddyfile,
// docker-compose.yml) are always refreshed from the embedded copy so that CLI
// upgrades take effect immediately. Data dirs (certs/, assignments/) are left
// untouched.
func EnsureInstalled(ctx context.Context, s *state.State, payloadVersion string) error {
	payloadDir, err := platform.PayloadDir()
	if err != nil {
		return err
	}

	alreadyInstalled := s.IsPayloadInstalled()

	// Always re-extract bundle config files so Caddyfile / docker-compose.yml
	// stay in sync with the embedded version shipped in this binary.
	if err := Extract(payloadDir, payloadVersion); err != nil {
		return fmt.Errorf("extracting payload bundle: %w", err)
	}

	if alreadyInstalled {
		output.Successf("Payload config refreshed (v%s)", payloadVersion)
	} else {
		output.Infof("Config bundle v%s installed", payloadVersion)
	}

	// Create certs directory inside payload dir.
	certsDir := filepath.Join(payloadDir, "certs")
	if err := os.MkdirAll(certsDir, 0755); err != nil {
		return err
	}

	s.PayloadPath = payloadDir
	s.PayloadVersion = payloadVersion
	s.PayloadInstalledAt = time.Now()
	return state.Save(*s)
}

// Update pulls the new Docker image and updates the compose file.
func Update(ctx context.Context, s *state.State, rel releases.PayloadRelease) error {
	output.Infof("Pulling image %s...", rel.Image)
	if err := docker.Pull(ctx, rel.Image); err != nil {
		return fmt.Errorf("docker pull failed: %w", err)
	}

	payloadDir, err := platform.PayloadDir()
	if err != nil {
		return err
	}

	if err := UpdateImage(payloadDir, rel.Image); err != nil {
		return fmt.Errorf("updating docker-compose image tag: %w", err)
	}

	s.PayloadVersion = rel.Version
	return state.Save(*s)
}

// UpdateImage rewrites the image tag in docker-compose.yml.
func UpdateImage(payloadDir, newImage string) error {
	composePath := filepath.Join(payloadDir, "docker-compose.yml")
	data, err := os.ReadFile(composePath)
	if err != nil {
		return err
	}

	var lines []string
	scanner := bufio.NewScanner(bytes.NewReader(data))
	for scanner.Scan() {
		line := scanner.Text()
		trimmed := strings.TrimSpace(line)
		if strings.HasPrefix(trimmed, "image: sympactdev/ctfdevkit") {
			// Preserve indentation.
			indent := line[:len(line)-len(strings.TrimLeft(line, " \t"))]
			line = indent + "image: " + newImage
		}
		lines = append(lines, line)
	}

	newContent := strings.Join(lines, "\n") + "\n"
	return os.WriteFile(composePath, []byte(newContent), 0644)
}

// ComposeDir returns the directory where docker compose should be run.
func ComposeDir(s state.State) (string, error) {
	if s.PayloadPath == "" {
		return "", fmt.Errorf("payload not installed — run `devkit setup` first")
	}
	if _, err := os.Stat(filepath.Join(s.PayloadPath, "docker-compose.yml")); err != nil {
		return "", fmt.Errorf("payload directory missing or corrupted — run `devkit setup` to reinstall")
	}
	return s.PayloadPath, nil
}
