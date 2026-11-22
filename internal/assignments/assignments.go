// Package assignments handles detection of CTF assignment directories.
package assignments

import (
	"os"
	"path/filepath"
	"sort"
)

// List returns the sorted names of the valid assignment directories at path: those
// with both a config.json and a wwwroot/ directory, matching the webapp's notion of valid.
func List(path string) ([]string, error) {
	entries, err := os.ReadDir(path)
	if err != nil {
		return nil, err
	}
	var names []string
	for _, e := range entries {
		if !e.IsDir() {
			continue
		}
		sub := filepath.Join(path, e.Name())
		if fileExists(filepath.Join(sub, "config.json")) && dirExists(filepath.Join(sub, "wwwroot")) {
			names = append(names, e.Name())
		}
	}
	sort.Strings(names)
	return names, nil
}

// Count returns the number of valid assignment directories at path (0 on error).
func Count(path string) int {
	names, err := List(path)
	if err != nil {
		return 0
	}
	return len(names)
}

func fileExists(p string) bool {
	info, err := os.Stat(p)
	return err == nil && !info.IsDir()
}

func dirExists(p string) bool {
	info, err := os.Stat(p)
	return err == nil && info.IsDir()
}
