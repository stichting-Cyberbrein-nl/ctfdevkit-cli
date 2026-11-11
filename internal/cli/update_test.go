package cli

import (
	"testing"

	"github.com/stichting-Cyberbrein-nl/ctfdevkit-cli/internal/config"
	"github.com/stichting-Cyberbrein-nl/ctfdevkit-cli/internal/state"
)

func TestShouldRefreshPayload(t *testing.T) {
	cases := []struct {
		name      string
		installed string
		latest    string
		want      bool
	}{
		{"older install updates", "1.1.2", "1.3.0", true},
		// The regression: an equal version skipped the pull, so a republished image or a
		// local image with the same tag never got replaced.
		{"same version still pulls", "1.3.0", "1.3.0", true},
		{"nothing installed yet", "", "1.3.0", true},
		{"newer local test build is kept", "1.4.0", "1.3.0", false},
	}

	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			got, err := shouldRefreshPayload(tc.installed, tc.latest)
			if err != nil {
				t.Fatalf("shouldRefreshPayload(%q, %q) error: %v", tc.installed, tc.latest, err)
			}
			if got != tc.want {
				t.Fatalf("shouldRefreshPayload(%q, %q) = %v, want %v", tc.installed, tc.latest, got, tc.want)
			}
		})
	}
}

func TestSetupPayloadVersion(t *testing.T) {
	cases := []struct {
		name       string
		configured string
		installed  string
		pinned     bool
		want       string
	}{
		// The regression: config.json still named 1.1.2, so setup reinstalled 1.1.2 over 1.3.0.
		{"stale config never downgrades", "1.1.2", "1.3.0", false, "1.3.0"},
		{"newer configured version installs", "1.3.0", "1.1.2", false, "1.3.0"},
		{"first install uses the configured version", "1.3.0", "", false, "1.3.0"},
		{"DEVKIT_VERSION may pick an older version", "1.1.2", "1.3.0", true, "1.1.2"},
	}

	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			if tc.pinned {
				t.Setenv("DEVKIT_VERSION", tc.configured)
			} else {
				t.Setenv("DEVKIT_VERSION", "")
			}

			got := setupPayloadVersion(config.Config{Version: tc.configured}, state.State{PayloadVersion: tc.installed})
			if got != tc.want {
				t.Fatalf("setupPayloadVersion(config %q, installed %q) = %q, want %q", tc.configured, tc.installed, got, tc.want)
			}
		})
	}
}
