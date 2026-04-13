// Package browser opens URLs in the system's default browser.
package browser

import (
	"fmt"
	"os/exec"
	"runtime"

	"github.com/stichting-Cyberbrein-nl/ctfdevkit-cli/internal/platform"
)

// Open opens the given URL in the default browser for the current platform.
func Open(url string, plat platform.Platform) error {
	switch plat.OS {
	case platform.OSMacOS:
		return exec.Command("open", url).Start()
	case platform.OSLinux:
		return exec.Command("xdg-open", url).Start()
	case platform.OSWSL:
		// Try cmd.exe first; fall back to explorer.exe.
		if err := exec.Command("cmd.exe", "/c", "start", "", url).Start(); err != nil {
			return exec.Command("explorer.exe", url).Start()
		}
		return nil
	case platform.OSWindows:
		return exec.Command("cmd", "/c", "start", "", url).Start()
	default:
		return fmt.Errorf("unsupported platform for browser open: %s (GOOS=%s)", plat.OS, runtime.GOOS)
	}
}
