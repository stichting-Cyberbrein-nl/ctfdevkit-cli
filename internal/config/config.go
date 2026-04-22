// Package config handles loading and saving devkit configuration.
package config

import (
	"encoding/json"
	"os"
	"path/filepath"

	"github.com/stichting-Cyberbrein-nl/ctfdevkit-cli/internal/platform"
)

const filename = "config.json"

// Config holds all user-configurable devkit settings.
type Config struct {
	Brand              string `json:"brand"`
	Version            string `json:"version"`
	Domain             string `json:"domain"`
	BindIP             string `json:"bind_ip"`
	URL                string `json:"url"`
	AppContainer       string `json:"app_container"`
	GitHubRepo         string `json:"github_repo"`
	DockerImage        string `json:"docker_image"`
	ManifestURL        string `json:"manifest_url"`
	AssignmentsRepoURL string `json:"assignments_repo_url"`
	AssignmentsPath    string `json:"assignments_path"` // empty = use payload dir ./assignments
}

// Default returns the factory default configuration.
func Default() Config {
	return Config{
		Brand:              "Cyberbrein",
		Version:            "1.0.0",
		Domain:             "ctf.dev",
		BindIP:             "127.0.0.1",
		URL:                "https://ctf.dev",
		AppContainer:       "app",
		GitHubRepo:         "stichting-Cyberbrein-nl/ctfdevkit-cli",
		DockerImage:        "sympactdev/ctfdevkit",
		ManifestURL:        "https://raw.githubusercontent.com/stichting-Cyberbrein-nl/ctfdevkit-cli/main/manifest.json",
		AssignmentsRepoURL: "https://github.com/stichting-Cyberbrein-nl/assignments",
	}
}

// Load reads the config file, returning defaults for any missing fields.
// Environment variables override file values.
func Load() (Config, error) {
	cfg := Default()

	cfgDir, err := platform.ConfigDir()
	if err != nil {
		return applyEnv(cfg), nil
	}

	path := filepath.Join(cfgDir, filename)
	data, err := os.ReadFile(path)
	if err != nil {
		if os.IsNotExist(err) {
			return applyEnv(cfg), nil
		}
		return cfg, err
	}

	if err := json.Unmarshal(data, &cfg); err != nil {
		return cfg, err
	}

	return applyEnv(cfg), nil
}

// Save writes the config to disk.
func Save(cfg Config) error {
	cfgDir, err := platform.ConfigDir()
	if err != nil {
		return err
	}

	data, err := json.MarshalIndent(cfg, "", "  ")
	if err != nil {
		return err
	}

	return os.WriteFile(filepath.Join(cfgDir, filename), data, 0644)
}

// applyEnv overrides config fields with environment variables.
func applyEnv(cfg Config) Config {
	if v := os.Getenv("DEVKIT_BRAND"); v != "" {
		cfg.Brand = v
	}
	if v := os.Getenv("DEVKIT_VERSION"); v != "" {
		cfg.Version = v
	}
	if v := os.Getenv("DEVKIT_BIND_IP"); v != "" {
		cfg.BindIP = v
	}
	if v := os.Getenv("DEVKIT_URL"); v != "" {
		cfg.URL = v
	}
	if v := os.Getenv("DEVKIT_DOMAIN"); v != "" {
		cfg.Domain = v
	}
	if v := os.Getenv("DEVKIT_MANIFEST_URL"); v != "" {
		cfg.ManifestURL = v
	}
	if v := os.Getenv("DEVKIT_ASSIGNMENTS_REPO_URL"); v != "" {
		cfg.AssignmentsRepoURL = v
	}
	if v := os.Getenv("DEVKIT_ASSIGNMENTS_PATH"); v != "" {
		cfg.AssignmentsPath = v
	}
	return cfg
}
