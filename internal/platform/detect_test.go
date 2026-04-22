package platform_test

import (
	"testing"

	"github.com/stichting-Cyberbrein-nl/ctfdevkit-cli/internal/platform"
)

func TestDetectReturnsValidPlatform(t *testing.T) {
	p := platform.Detect()

	// OS must be a recognized value.
	switch p.OS {
	case platform.OSLinux, platform.OSMacOS, platform.OSWindows, platform.OSWSL:
		// OK
	default:
		t.Errorf("unexpected OS: %v", p.OS)
	}

	// Arch must be recognized.
	switch p.Arch {
	case platform.ArchAMD64, platform.ArchARM64:
		// OK
	default:
		t.Errorf("unexpected Arch: %v", p.Arch)
	}
}

func TestAssetKey(t *testing.T) {
	tests := []struct {
		p    platform.Platform
		want string
	}{
		{platform.Platform{OS: platform.OSLinux, Arch: platform.ArchAMD64}, "linux-amd64"},
		{platform.Platform{OS: platform.OSMacOS, Arch: platform.ArchARM64}, "darwin-arm64"},
		{platform.Platform{OS: platform.OSWSL, Arch: platform.ArchAMD64, IsWSL: true}, "linux-amd64"},
		{platform.Platform{OS: platform.OSWindows, Arch: platform.ArchAMD64}, "windows-amd64"},
	}
	for _, tt := range tests {
		got := tt.p.AssetKey()
		if got != tt.want {
			t.Errorf("AssetKey() = %q, want %q", got, tt.want)
		}
	}
}

func TestOSString(t *testing.T) {
	if platform.OSLinux.String() != "linux" {
		t.Error("expected linux")
	}
	if platform.OSMacOS.String() != "macos" {
		t.Error("expected macos")
	}
	if platform.OSWSL.String() != "wsl" {
		t.Error("expected wsl")
	}
	if platform.OSWindows.String() != "windows" {
		t.Error("expected windows")
	}
}

func TestIsUnix(t *testing.T) {
	if !platform.OSLinux.IsUnix() {
		t.Error("Linux should be Unix")
	}
	if !platform.OSMacOS.IsUnix() {
		t.Error("macOS should be Unix")
	}
	if !platform.OSWSL.IsUnix() {
		t.Error("WSL should be Unix")
	}
	if platform.OSWindows.IsUnix() {
		t.Error("Windows should not be Unix")
	}
}
