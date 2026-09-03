package cli

import (
	"testing"
	"time"
)

func TestShouldCheckForUpdates(t *testing.T) {
	recent := time.Now().Add(-5 * time.Minute)
	stale := time.Now().Add(-24 * time.Hour)

	cases := []struct {
		name      string
		cmd       string
		version   string
		lastCheck time.Time
		want      bool
	}{
		// The regression: `setup` and `up` never checked, so a broken CLI stayed broken.
		{"setup always checks", "setup", "1.0.14", recent, true},
		{"up always checks", "up", "1.0.14", recent, true},

		{"self-update never recurses", "self-update", "1.0.14", stale, false},
		{"update never recurses", "update", "1.0.14", stale, false},
		{"version stays offline", "version", "1.0.14", stale, false},
		{"doctor stays usable offline", "doctor", "1.0.14", stale, false},

		{"dev build never checks", "status", "dev", stale, false},
		{"empty version never checks", "status", "", stale, false},

		{"routine command throttled", "status", "1.0.14", recent, false},
		{"routine command after interval", "status", "1.0.14", stale, true},
		{"never checked before", "status", "1.0.14", time.Time{}, true},
	}

	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			if got := shouldCheckForUpdates(tc.cmd, tc.version, tc.lastCheck); got != tc.want {
				t.Fatalf("shouldCheckForUpdates(%q, %q) = %v, want %v", tc.cmd, tc.version, got, tc.want)
			}
		})
	}
}

// Every command that pulls a payload must also be allowed to check at all.
func TestPayloadCommandsAreNotSkipped(t *testing.T) {
	for cmd := range commandsUpdatingPayload {
		if commandsSkippingUpdateCheck[cmd] {
			t.Fatalf("%q both updates the payload and skips the update check", cmd)
		}
	}
}

func TestIsInteractiveDoesNotPanic(t *testing.T) {
	_ = isInteractive()
}
