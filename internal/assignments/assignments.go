// Package assignments handles detection and validation of the CTF assignments directory.
package assignments

import (
	"os"
	"path/filepath"
	"runtime"
)

// IsValid returns true if the directory looks like a CTF assignments folder
// (exists and contains at least one subdirectory with a config.json or wwwroot/).
func IsValid(path string) bool {
	if path == "" {
		return false
	}
	info, err := os.Stat(path)
	if err != nil || !info.IsDir() {
		return false
	}

	entries, err := os.ReadDir(path)
	if err != nil {
		return false
	}

	for _, e := range entries {
		if !e.IsDir() {
			continue
		}
		sub := filepath.Join(path, e.Name())
		if fileExists(filepath.Join(sub, "config.json")) ||
			dirExists(filepath.Join(sub, "wwwroot")) {
			return true
		}
	}
	return false
}

// Count returns the number of valid assignment directories found at path.
func Count(path string) int {
	entries, err := os.ReadDir(path)
	if err != nil {
		return 0
	}
	n := 0
	for _, e := range entries {
		if !e.IsDir() {
			continue
		}
		sub := filepath.Join(path, e.Name())
		if fileExists(filepath.Join(sub, "config.json")) ||
			dirExists(filepath.Join(sub, "wwwroot")) {
			n++
		}
	}
	return n
}

// AutoDetect searches common locations for a valid assignments directory.
// Returns the first match found, or "" if nothing is found.
func AutoDetect() string {
	home, _ := os.UserHomeDir()

	candidates := []string{
		// Current working directory.
		func() string { d, _ := os.Getwd(); return filepath.Join(d, "assignments") }(),
		// Standard project location next to the payload.
		filepath.Join(home, "assignments"),
		filepath.Join(home, "ctf", "assignments"),
		filepath.Join(home, "ctfdevkit", "assignments"),
		filepath.Join(home, "Desktop", "ctfdevkit", "assignments"),
		filepath.Join(home, "Desktop", "ctfdevkit", "ctfdevkit", "assignments"),
	}

	// Native Windows: also search the shared Public folder.
	if runtime.GOOS == "windows" {
		pub := os.Getenv("PUBLIC")
		if pub == "" {
			pub = `C:\Users\Public`
		}
		candidates = append(candidates,
			filepath.Join(pub, "ctf", "assignments"),
			filepath.Join(pub, "ctfdevkit", "assignments"),
		)
	}

	// WSL: search the mounted Windows filesystem.
	if runtime.GOOS == "linux" {
		candidates = append(candidates, "/mnt/c/Users/Public/ctf/assignments")
	}

	for _, c := range candidates {
		if IsValid(c) {
			return c
		}
		// Also accept empty-but-existing dir (user may add assignments later).
		if dirExists(c) {
			return c
		}
	}
	return ""
}

func fileExists(p string) bool {
	info, err := os.Stat(p)
	return err == nil && !info.IsDir()
}

func dirExists(p string) bool {
	info, err := os.Stat(p)
	return err == nil && info.IsDir()
}
