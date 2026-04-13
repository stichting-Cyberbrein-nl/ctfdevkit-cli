// Package state tracks the installed state of the devkit payload and CLI versions.
package state

import (
	"encoding/json"
	"os"
	"path/filepath"
	"time"

	"github.com/stichting-Cyberbrein-nl/ctfdevkit-cli/internal/platform"
)

const filename = "state.json"

// State tracks what is installed and at which versions.
type State struct {
	CLIVersion          string    `json:"cli_version"`
	PayloadVersion      string    `json:"payload_version"`
	PayloadPath         string    `json:"payload_path"`
	PayloadInstalledAt  time.Time `json:"payload_installed_at"`
	LastUpdateCheck     time.Time `json:"last_update_check"`
}

// Load reads the state file. Returns a zero State if not found.
func Load() (State, error) {
	path, err := statePath()
	if err != nil {
		return State{}, err
	}

	data, err := os.ReadFile(path)
	if err != nil {
		if os.IsNotExist(err) {
			return State{}, nil
		}
		return State{}, err
	}

	var s State
	if err := json.Unmarshal(data, &s); err != nil {
		return State{}, err
	}
	return s, nil
}

// Save writes the state to disk.
func Save(s State) error {
	path, err := statePath()
	if err != nil {
		return err
	}

	data, err := json.MarshalIndent(s, "", "  ")
	if err != nil {
		return err
	}

	return os.WriteFile(path, data, 0644)
}

// IsPayloadInstalled returns true if the payload appears to be correctly installed.
func (s State) IsPayloadInstalled() bool {
	if s.PayloadPath == "" {
		return false
	}
	info, err := os.Stat(filepath.Join(s.PayloadPath, "docker-compose.yml"))
	return err == nil && !info.IsDir()
}

func statePath() (string, error) {
	cfgDir, err := platform.ConfigDir()
	if err != nil {
		return "", err
	}
	return filepath.Join(cfgDir, filename), nil
}
