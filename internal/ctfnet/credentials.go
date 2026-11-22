package ctfnet

import (
	"encoding/json"
	"errors"
	"fmt"
	"io/fs"
	"os"
	"path/filepath"
	"time"

	"github.com/stichting-Cyberbrein-nl/ctfdevkit-cli/internal/platform"
)

const credentialsFile = "ctfnet-credentials.json"

// Credentials is the stored platform login.
type Credentials struct {
	PlatformURL  string    `json:"platform_url"`
	AccessToken  string    `json:"access_token"`
	RefreshToken string    `json:"refresh_token"`
	ExpiresAt    time.Time `json:"expires_at"`
	User         User      `json:"user"`
}

// CredentialsPath returns where the platform login is stored.
func CredentialsPath() (string, error) {
	dir, err := platform.ConfigDir()
	if err != nil {
		return "", err
	}
	return filepath.Join(dir, credentialsFile), nil
}

// LoadCredentialsFrom reads credentials from path.
func LoadCredentialsFrom(path string) (Credentials, error) {
	var c Credentials
	data, err := os.ReadFile(path)
	if err != nil {
		return c, err
	}
	if err := json.Unmarshal(data, &c); err != nil {
		return c, fmt.Errorf("parsing %s: %w", path, err)
	}
	return c, nil
}

// SaveCredentialsTo writes credentials to path, readable by the owner only.
func SaveCredentialsTo(path string, c Credentials) error {
	data, err := json.MarshalIndent(c, "", "  ")
	if err != nil {
		return err
	}
	if err := os.WriteFile(path, data, 0o600); err != nil {
		return err
	}
	return os.Chmod(path, 0o600)
}

// DeleteCredentialsAt removes the credentials at path; a missing file is not an error.
func DeleteCredentialsAt(path string) error {
	if err := os.Remove(path); err != nil && !errors.Is(err, fs.ErrNotExist) {
		return err
	}
	return nil
}

// LoadCredentials reads the stored platform login.
func LoadCredentials() (Credentials, error) {
	path, err := CredentialsPath()
	if err != nil {
		return Credentials{}, err
	}
	return LoadCredentialsFrom(path)
}

// SaveCredentials stores the platform login.
func SaveCredentials(c Credentials) error {
	path, err := CredentialsPath()
	if err != nil {
		return err
	}
	return SaveCredentialsTo(path, c)
}

// DeleteCredentials removes the stored platform login.
func DeleteCredentials() error {
	path, err := CredentialsPath()
	if err != nil {
		return err
	}
	return DeleteCredentialsAt(path)
}
