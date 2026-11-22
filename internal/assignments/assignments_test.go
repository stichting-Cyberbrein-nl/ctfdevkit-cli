package assignments

import (
	"os"
	"path/filepath"
	"slices"
	"testing"
)

func TestListReturnsOnlyValidFoldersSorted(t *testing.T) {
	root := t.TempDir()
	mk := func(name string, config, wwwroot bool) {
		dir := filepath.Join(root, name)
		if err := os.MkdirAll(dir, 0o755); err != nil {
			t.Fatal(err)
		}
		if config {
			if err := os.WriteFile(filepath.Join(dir, "config.json"), []byte("{}"), 0o644); err != nil {
				t.Fatal(err)
			}
		}
		if wwwroot {
			if err := os.MkdirAll(filepath.Join(dir, "wwwroot"), 0o755); err != nil {
				t.Fatal(err)
			}
		}
	}
	mk("[2] B", true, true)
	mk("[1] A", true, true)
	mk("[3] No config", false, true)
	mk("[4] No wwwroot", true, false)
	if err := os.WriteFile(filepath.Join(root, "file.txt"), nil, 0o644); err != nil {
		t.Fatal(err)
	}

	got, err := List(root)
	if err != nil {
		t.Fatal(err)
	}
	if want := []string{"[1] A", "[2] B"}; !slices.Equal(got, want) {
		t.Fatalf("List = %v, want %v", got, want)
	}
	if n := Count(root); n != 2 {
		t.Fatalf("Count = %d, want 2", n)
	}
	if n := Count(filepath.Join(root, "missing")); n != 0 {
		t.Fatalf("Count(missing) = %d, want 0", n)
	}
}
