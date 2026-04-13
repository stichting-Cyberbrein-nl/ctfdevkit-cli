// Package releases handles fetching and parsing the release manifest.
package releases

import (
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"time"

	"github.com/Masterminds/semver/v3"
)

// Asset describes a single downloadable binary artifact.
type Asset struct {
	URL    string `json:"url"`
	SHA256 string `json:"sha256"`
}

// CLIRelease describes the latest CLI version and its per-platform assets.
type CLIRelease struct {
	Version string           `json:"version"`
	Assets  map[string]Asset `json:"assets"` // key: "linux-amd64", "darwin-arm64", etc.
}

// PayloadRelease describes the latest payload (Docker image).
type PayloadRelease struct {
	Version string `json:"version"`
	Image   string `json:"image"` // e.g. "sympactdev/ctfdevkit:1.1.0"
}

// Manifest is the top-level release manifest structure.
type Manifest struct {
	CLI     CLIRelease     `json:"cli"`
	Payload PayloadRelease `json:"payload"`
}

// Fetch downloads and parses the manifest from the given URL.
func Fetch(ctx context.Context, manifestURL string) (*Manifest, error) {
	ctx, cancel := context.WithTimeout(ctx, 10*time.Second)
	defer cancel()

	req, err := http.NewRequestWithContext(ctx, http.MethodGet, manifestURL, nil)
	if err != nil {
		return nil, fmt.Errorf("building manifest request: %w", err)
	}
	req.Header.Set("User-Agent", "devkit-cli/1.0")

	resp, err := http.DefaultClient.Do(req)
	if err != nil {
		return nil, fmt.Errorf("fetching manifest: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		return nil, fmt.Errorf("manifest server returned %d", resp.StatusCode)
	}

	body, err := io.ReadAll(io.LimitReader(resp.Body, 1<<20)) // 1 MB max
	if err != nil {
		return nil, fmt.Errorf("reading manifest: %w", err)
	}

	var m Manifest
	if err := json.Unmarshal(body, &m); err != nil {
		return nil, fmt.Errorf("parsing manifest: %w", err)
	}

	return &m, nil
}

// IsNewerCLI returns true if the manifest CLI version is newer than currentVersion.
func (m *Manifest) IsNewerCLI(currentVersion string) (bool, error) {
	return isNewer(m.CLI.Version, currentVersion)
}

// IsNewerPayload returns true if the manifest payload version is newer than currentVersion.
func (m *Manifest) IsNewerPayload(currentVersion string) (bool, error) {
	return isNewer(m.Payload.Version, currentVersion)
}

func isNewer(manifestVersion, currentVersion string) (bool, error) {
	if currentVersion == "" || currentVersion == "dev" {
		return true, nil
	}

	mv, err := semver.NewVersion(manifestVersion)
	if err != nil {
		return false, fmt.Errorf("invalid manifest version %q: %w", manifestVersion, err)
	}
	cv, err := semver.NewVersion(currentVersion)
	if err != nil {
		return false, fmt.Errorf("invalid current version %q: %w", currentVersion, err)
	}

	return mv.GreaterThan(cv), nil
}
