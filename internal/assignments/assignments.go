// Package assignments handles detection of CTF assignment directories.
package assignments

import (
	"os"
	"path/filepath"
)

// Count returns the number of valid assignment directories at path: those with both
// a config.json and a wwwroot/ directory, matching the webapp's notion of valid.
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
		if fileExists(filepath.Join(sub, "config.json")) && dirExists(filepath.Join(sub, "wwwroot")) {
			n++
		}
	}
	return n
}
func fileExists(p string) bool {
	info, err := os.Stat(p)
	return err == nil && !info.IsDir()
}

func dirExists(p string) bool {
	info, err := os.Stat(p)
	return err == nil && info.IsDir()
}
