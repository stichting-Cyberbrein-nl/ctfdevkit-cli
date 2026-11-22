package ctfnet

import (
	"archive/zip"
	"bytes"
	"os"
	"path/filepath"
	"slices"
	"strings"
	"testing"
)

func writeFile(t *testing.T, path, content string) {
	t.Helper()
	if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(path, []byte(content), 0o644); err != nil {
		t.Fatal(err)
	}
}

func TestZipAssignmentPrefixesAndSkips(t *testing.T) {
	dir := filepath.Join(t.TempDir(), "[12] Test")
	writeFile(t, filepath.Join(dir, "config.json"), "{}")
	writeFile(t, filepath.Join(dir, "wwwroot", "index.php"), "<?php echo 1;")
	writeFile(t, filepath.Join(dir, ".git", "HEAD"), "ref")
	writeFile(t, filepath.Join(dir, "node_modules", "x.js"), "x")
	writeFile(t, filepath.Join(dir, "wwwroot", ".DS_Store"), "x")

	data, err := ZipAssignment(dir)
	if err != nil {
		t.Fatal(err)
	}
	zr, err := zip.NewReader(bytes.NewReader(data), int64(len(data)))
	if err != nil {
		t.Fatal(err)
	}
	var names []string
	for _, f := range zr.File {
		names = append(names, f.Name)
	}
	slices.Sort(names)
	if want := []string{"[12] Test/config.json", "[12] Test/wwwroot/index.php"}; !slices.Equal(names, want) {
		t.Fatalf("entries = %v, want %v", names, want)
	}
}

func TestZipAssignmentRejectsSymlinks(t *testing.T) {
	dir := filepath.Join(t.TempDir(), "[12] Test")
	writeFile(t, filepath.Join(dir, "config.json"), "{}")
	if err := os.Symlink(filepath.Join(dir, "config.json"), filepath.Join(dir, "link.json")); err != nil {
		t.Skipf("cannot create symlink: %v", err)
	}
	_, err := ZipAssignment(dir)
	if err == nil || !strings.Contains(err.Error(), "link.json") {
		t.Fatalf("err = %v, want symlink error naming link.json", err)
	}
}

func TestValidName(t *testing.T) {
	cases := map[string]bool{
		"[12] Cookie stealing": true,
		"[0] Introductie":      true,
		"Cookie":               false,
		"[12] a/b":             false,
		"[12] x.":              false,
		"[12] 50%":             false,
		"[12] trailing ":       false,
		"[12345] Too long id":  false,
	}
	for name, want := range cases {
		if got := ValidName(name); got != want {
			t.Errorf("ValidName(%q) = %v, want %v", name, got, want)
		}
	}
}
