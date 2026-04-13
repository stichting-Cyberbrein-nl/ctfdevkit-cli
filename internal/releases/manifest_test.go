package releases_test

import (
	"encoding/json"
	"testing"

	"github.com/stichting-Cyberbrein-nl/ctfdevkit-cli/internal/releases"
)

func TestManifestParsing(t *testing.T) {
	raw := `{
		"cli": {
			"version": "1.2.0",
			"assets": {
				"linux-amd64": {"url": "https://example.com/devkit-linux-amd64.tar.gz", "sha256": "abc123"}
			}
		},
		"payload": {
			"version": "1.1.0",
			"image": "sympactdev/ctfdevkit:1.1.0"
		}
	}`

	var m releases.Manifest
	if err := json.Unmarshal([]byte(raw), &m); err != nil {
		t.Fatalf("unmarshal failed: %v", err)
	}

	if m.CLI.Version != "1.2.0" {
		t.Errorf("expected CLI version 1.2.0, got %s", m.CLI.Version)
	}
	asset, ok := m.CLI.Assets["linux-amd64"]
	if !ok {
		t.Fatal("missing linux-amd64 asset")
	}
	if asset.SHA256 != "abc123" {
		t.Errorf("expected sha256 abc123, got %s", asset.SHA256)
	}
	if m.Payload.Image != "sympactdev/ctfdevkit:1.1.0" {
		t.Errorf("unexpected payload image: %s", m.Payload.Image)
	}
}

func TestIsNewerCLI(t *testing.T) {
	tests := []struct {
		name           string
		manifestVer    string
		currentVer     string
		expectNewer    bool
		expectError    bool
	}{
		{"newer available", "1.2.0", "1.1.0", true, false},
		{"already latest", "1.1.0", "1.1.0", false, false},
		{"older manifest", "1.0.0", "1.1.0", false, false},
		{"empty current", "1.0.0", "", true, false},
		{"dev current", "1.0.0", "dev", true, false},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			m := &releases.Manifest{}
			m.CLI.Version = tt.manifestVer

			got, err := m.IsNewerCLI(tt.currentVer)
			if (err != nil) != tt.expectError {
				t.Errorf("unexpected error: %v", err)
			}
			if got != tt.expectNewer {
				t.Errorf("IsNewerCLI(%q) = %v, want %v", tt.currentVer, got, tt.expectNewer)
			}
		})
	}
}

func TestIsNewerPayload(t *testing.T) {
	m := &releases.Manifest{}
	m.Payload.Version = "2.0.0"

	newer, err := m.IsNewerPayload("1.9.0")
	if err != nil {
		t.Fatal(err)
	}
	if !newer {
		t.Error("expected 2.0.0 to be newer than 1.9.0")
	}

	same, err := m.IsNewerPayload("2.0.0")
	if err != nil {
		t.Fatal(err)
	}
	if same {
		t.Error("expected 2.0.0 to not be newer than 2.0.0")
	}
}
